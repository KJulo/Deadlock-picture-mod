package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"image/jpeg"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/build"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/templates"
)

const repoRoot = "../../.."

func testPhoto(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, "testdata", "photo_input.png"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// fakeBuild checks the photo like the real build does but returns a tiny VPK instantly.
func fakeBuild(got *photo.Params) BuildFunc {
	return func(data []byte, p photo.Params) (*build.Result, error) {
		if _, err := photo.Load(data, p); err != nil {
			return nil, err
		}
		if got != nil {
			*got = p
		}
		return &build.Result{Files: map[string][]byte{"models/x.vmdl_c": []byte("fake")}}, nil
	}
}

func newServer(t *testing.T, b BuildFunc, mutate ...func(*Config)) http.Handler {
	t.Helper()
	cfg := Config{
		MaxUploadBytes: 1 << 20, MaxConcurrent: 2, QueueTimeout: time.Second,
		AllowedOrigins: []string{"http://localhost:4321"},
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, m := range mutate {
		m(&cfg)
	}
	s, err := New(cfg, b)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handler()
}

// upload builds a multipart request with an optional photo and params.
func upload(t *testing.T, path string, photoData []byte, params string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if photoData != nil {
		fw, _ := mw.CreateFormFile("photo", "cat.png")
		fw.Write(photoData)
	}
	if params != "" {
		mw.WriteField("params", params)
	}
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func do(h http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct{ Code string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("error body is not JSON: %q", rec.Body.String())
	}
	return body.Error.Code
}

func TestHealthAndDefaults(t *testing.T) {
	h := newServer(t, fakeBuild(nil))
	if rec := do(h, httptest.NewRequest("GET", "/healthz", nil)); rec.Code != 200 {
		t.Errorf("healthz %d", rec.Code)
	}
	rec := do(h, httptest.NewRequest("GET", "/api/defaults", nil))
	var d struct {
		Params         photo.Params
		MaxUploadBytes int64 `json:"max_upload_bytes"`
	}
	json.Unmarshal(rec.Body.Bytes(), &d)
	if rec.Code != 200 || d.Params.Layout != "cube6" || d.MaxUploadBytes != 1<<20 {
		t.Errorf("defaults %d %+v", rec.Code, d)
	}
}

func TestMesh(t *testing.T) {
	rec := do(newServer(t, fakeBuild(nil)), httptest.NewRequest("GET", "/api/mesh", nil))
	var m struct {
		Positions []float32
		UVs       []float64
		Indices   []uint32
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	n := len(m.Positions) / 3
	if n != 77*39 || len(m.UVs) != 2*n || len(m.Indices) != 3*2*76*38 {
		t.Fatalf("got %d vertices, %d uvs, %d indices", n, len(m.UVs), len(m.Indices))
	}
	for _, i := range m.Indices {
		if int(i) >= n {
			t.Fatalf("index %d out of range", i)
		}
	}
	for _, uv := range m.UVs {
		if uv < 0 || uv > 1 {
			t.Fatalf("uv %v out of range", uv)
		}
	}
	h := newServer(t, fakeBuild(nil))
	rec = do(h, httptest.NewRequest("GET", "/api/mesh?lod=3", nil))
	json.Unmarshal(rec.Body.Bytes(), &m)
	if rec.Code != 200 || len(m.Positions)/3 != 33*17 {
		t.Errorf("lod 3: %d, %d vertices", rec.Code, len(m.Positions)/3)
	}
	if rec := do(h, httptest.NewRequest("GET", "/api/mesh?lod=9", nil)); rec.Code != 400 {
		t.Errorf("lod 9: %d", rec.Code)
	}
	var cube struct {
		Shape     string
		Positions []float32
		Normals   []float32
	}
	rec = do(h, httptest.NewRequest("GET", "/api/mesh?shape=cube&lod=1", nil))
	json.Unmarshal(rec.Body.Bytes(), &cube)
	if rec.Code != 200 || cube.Shape != "cube" || len(cube.Normals) != len(cube.Positions) || len(cube.Positions) == 0 {
		t.Errorf("cube: %d %q %d normals", rec.Code, cube.Shape, len(cube.Normals))
	}
	if rec := do(h, httptest.NewRequest("GET", "/api/mesh?shape=teapot", nil)); rec.Code != 400 {
		t.Errorf("teapot: %d", rec.Code)
	}
}

func TestBuildReturnsVPKWithParams(t *testing.T) {
	var got photo.Params
	h := newServer(t, fakeBuild(&got))
	rec := do(h, upload(t, "/api/build", testPhoto(t), `{"layout":"wrap","back":"flip"}`))
	if rec.Code != 200 {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, ".vpk") {
		t.Errorf("Content-Disposition %q", cd)
	}
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x34, 0x12, 0xaa, 0x55}) {
		t.Error("body is not a VPK")
	}
	if got.Layout != "wrap" || got.Back != "flip" || got.StickerDeg != 40 {
		t.Errorf("params not applied over the defaults: %+v", got)
	}
}

func TestBuildErrors(t *testing.T) {
	h := newServer(t, fakeBuild(nil))
	cases := []struct {
		name   string
		req    *http.Request
		status int
		code   string
	}{
		{"no photo", upload(t, "/api/build", nil, ""), 400, "missing_photo"},
		{"not multipart", httptest.NewRequest("POST", "/api/build", strings.NewReader("hi")), 400, "bad_request"},
		{"unknown param", upload(t, "/api/build", testPhoto(t), `{"layuot":"wrap"}`), 400, "bad_params"},
		{"invalid param", upload(t, "/api/build", testPhoto(t), `{"radius":99}`), 400, "bad_params"},
		{"garbage photo", upload(t, "/api/build", []byte("not an image at all"), ""), 415, "bad_photo"},
		{"too large", upload(t, "/api/build", make([]byte, 2<<20), ""), 413, "photo_too_large"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(h, c.req)
			if rec.Code != c.status || errorCode(t, rec) != c.code {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body.String(), c.status, c.code)
			}
		})
	}
	if rec := do(h, httptest.NewRequest("GET", "/api/build", nil)); rec.Code != 405 {
		t.Errorf("GET /api/build: %d, want 405", rec.Code)
	}
}

func TestTexture(t *testing.T) {
	h := newServer(t, fakeBuild(nil))
	rec := do(h, upload(t, "/api/texture?size=256", testPhoto(t), ""))
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	img, err := jpeg.Decode(rec.Body)
	if err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("bad jpeg: %v", err)
	}
	if rec := do(h, upload(t, "/api/texture?size=99999", testPhoto(t), "")); rec.Code != 400 {
		t.Errorf("huge size: %d", rec.Code)
	}
	if rec := do(h, upload(t, "/api/texture", testPhoto(t), `{"variant":"cage"}`)); rec.Code != 400 || errorCode(t, rec) != "no_preview" {
		t.Errorf("cage texture: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCORS(t *testing.T) {
	h := newServer(t, fakeBuild(nil))
	pre := httptest.NewRequest("OPTIONS", "/api/build", nil)
	pre.Header.Set("Origin", "http://localhost:4321")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec := do(h, pre)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:4321" {
		t.Errorf("allowed preflight: %d %v", rec.Code, rec.Header())
	}
	evil := httptest.NewRequest("GET", "/api/defaults", nil)
	evil.Header.Set("Origin", "https://evil.example")
	if got := do(h, evil).Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin allowed: %q", got)
	}
}

// With every slot taken, a new request waits QueueTimeout and then gets 503.
func TestBusy(t *testing.T) {
	block := make(chan struct{})
	started := make(chan struct{}, 1)
	slow := func(data []byte, p photo.Params) (*build.Result, error) {
		started <- struct{}{}
		<-block
		return &build.Result{Files: map[string][]byte{"a/b.txt": nil}}, nil
	}
	h := newServer(t, slow, func(c *Config) { c.MaxConcurrent = 1; c.QueueTimeout = 50 * time.Millisecond })
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); do(h, upload(t, "/api/build", testPhoto(t), "")) }()
	<-started
	rec := do(h, upload(t, "/api/build", testPhoto(t), ""))
	close(block)
	wg.Wait()
	if rec.Code != 503 || rec.Header().Get("Retry-After") == "" || errorCode(t, rec) != "busy" {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}
}

// A panic inside a build becomes a 500 and the server keeps working.
func TestPanicRecovered(t *testing.T) {
	boom := func([]byte, photo.Params) (*build.Result, error) { panic("boom") }
	h := newServer(t, boom)
	rec := do(h, upload(t, "/api/build", testPhoto(t), ""))
	if rec.Code != 500 || errorCode(t, rec) != "internal" {
		t.Errorf("got %d %s", rec.Code, rec.Body.String())
	}
	if do(h, httptest.NewRequest("GET", "/healthz", nil)).Code != 200 {
		t.Error("server broken after panic")
	}
}

// The real build through HTTP (needs the game files; skipped in CI).
func TestRealBuild(t *testing.T) {
	tpl, err := build.LoadTemplates(templates.Dir(filepath.Join(repoRoot, "templates")))
	if err != nil {
		t.Skip(err)
	}
	h := newServer(t, func(d []byte, p photo.Params) (*build.Result, error) { return build.Mod(tpl, d, p) })
	rec := do(h, upload(t, "/api/build", testPhoto(t), ""))
	if rec.Code != 200 || rec.Body.Len() < 10<<20 {
		t.Fatalf("status %d, %d bytes", rec.Code, rec.Body.Len())
	}
}

// Clients that accept gzip get a compressed body that decompresses to the exact same bytes.
func TestGzip(t *testing.T) {
	big := bytes.Repeat([]byte("soul container "), 10000)
	b := func([]byte, photo.Params) (*build.Result, error) {
		return &build.Result{Files: map[string][]byte{"a/big.vtex_c": big}}, nil
	}
	h := newServer(t, b)
	plain := do(h, upload(t, "/api/build", testPhoto(t), ""))
	req := upload(t, "/api/build", testPhoto(t), "")
	req.Header.Set("Accept-Encoding", "br, gzip;q=0.8")
	zipped := do(h, req)
	if zipped.Header().Get("Content-Encoding") != "gzip" || plain.Header().Get("Content-Encoding") != "" {
		t.Fatalf("encodings: %q / %q", zipped.Header().Get("Content-Encoding"), plain.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(zipped.Body)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(zr)
	if !bytes.Equal(got, plain.Body.Bytes()) {
		t.Fatal("gzip body differs from plain body")
	}
	t.Logf("%d -> %d bytes", plain.Body.Len(), zipped.Body.Len())
	for header, want := range map[string]bool{"gzip": true, "deflate, gzip;q=0": false, "": false, "GZIP": true} {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Accept-Encoding", header)
		if acceptsGzip(r) != want {
			t.Errorf("acceptsGzip(%q) != %v", header, want)
		}
	}
}
