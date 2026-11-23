package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"image/jpeg"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	if n != 6*23*23 || len(m.UVs) != 2*n || len(m.Indices) != 3*12*22*22 {
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
	if rec.Code != 200 || len(m.Positions)/3 != 6*10*10 {
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

func TestMultipartMultiplePhotos(t *testing.T) {
	data := testPhoto(t)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, name := range []string{"photo", "photo1"} {
		w, err := mw.CreateFormFile(name, "photo.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	mw.WriteField("params", `{"seals":[{"photo":1,"dir":[1,0,0],"center":[0.5,0.5],"radius":0.5}]}`)
	mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/texture?size=256", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := do(newServer(t, fakeBuild(nil)), req)
	if rec.Code != http.StatusOK {
		t.Fatalf("texture: %d %s", rec.Code, rec.Body.String())
	}
}

func jsonPost(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	return req
}

func TestNormalTextureJSON(t *testing.T) {
	h := newServer(t, fakeBuild(nil))

	for _, tc := range []struct {
		name string
		body string
	}{
		{"relief params", `{"relief":0.5,"shape":"sphere"}`},
		{"empty object defaults", `{}`},
		{"multi-seal params without photos", `{"relief":0.4,"seals":[{"photo":1,"dir":[1,0,0],"center":[0.5,0.5],"radius":0.5}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(h, jsonPost("/api/texture?size=256&map=normal", tc.body))
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d ct=%q cc=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), rec.Body.String())
			}
			img, err := png.Decode(rec.Body)
			if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
				t.Fatalf("bad png: err=%v bounds=%v", err, img.Bounds())
			}
		})
	}

	errCases := []struct {
		name   string
		req    *http.Request
		status int
		code   string
	}{
		{"unknown field", jsonPost("/api/texture?size=256&map=normal", `{"relife":0.5}`), 400, "bad_params"},
		{"invalid param", jsonPost("/api/texture?size=256&map=normal", `{"relief":2}`), 400, "bad_params"},
		{"malformed json", jsonPost("/api/texture?size=256&map=normal", `{`), 400, "bad_params"},
		{"trailing data", jsonPost("/api/texture?size=256&map=normal", `{}{}`), 400, "bad_params"},
		{"too long", jsonPost("/api/texture?size=256&map=normal", `{"layout":"`+strings.Repeat("a", maxParamsBytes)+`"}`), 400, "bad_params"},
		{"bad content type", func() *http.Request {
			r := httptest.NewRequest(http.MethodPost, "/api/texture?size=256&map=normal", strings.NewReader("{}"))
			r.Header.Set("Content-Type", "text/plain")
			return r
		}(), 400, "bad_request"},
		{"cage variant", jsonPost("/api/texture?size=256&map=normal", `{"variant":"cage"}`), 400, "no_preview"},
		{"bad size", jsonPost("/api/texture?size=99999&map=normal", `{}`), 400, "bad_size"},
	}
	for _, c := range errCases {
		t.Run(c.name, func(t *testing.T) {
			rec := do(h, c.req)
			if rec.Code != c.status || errorCode(t, rec) != c.code {
				t.Errorf("got %d %s, want %d %s", rec.Code, rec.Body.String(), c.status, c.code)
			}
		})
	}
}

func TestNormalTextureMultipart(t *testing.T) {
	h := newServer(t, fakeBuild(nil))

	for _, tc := range []struct {
		name  string
		photo []byte
	}{
		{"without photo", nil},
		{"with photo", testPhoto(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(h, upload(t, "/api/texture?size=256&map=normal", tc.photo, `{"relief":0.5}`))
			if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" || rec.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("status=%d ct=%q cc=%q body=%s", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Cache-Control"), rec.Body.String())
			}
			img, err := png.Decode(rec.Body)
			if err != nil || img.Bounds().Dx() != 256 || img.Bounds().Dy() != 256 {
				t.Fatalf("bad png: err=%v", err)
			}
		})
	}
}

func TestRateLimit(t *testing.T) {
	h := newServer(t, fakeBuild(nil), func(c *Config) {
		c.RateLimitPerMin = 2
		c.BuildRateLimitPerMin = 1
	})

	const origin = "http://localhost:4321"
	const ipA = "203.0.113.10, 10.0.0.1"
	const ipB = "203.0.113.11"

	// Burst of 2 texture requests succeeds for IP A.
	for i := 0; i < 2; i++ {
		req := jsonPost("/api/texture?size=256&map=normal", `{}`)
		req.Header.Set("X-Forwarded-For", ipA)
		req.Header.Set("Origin", origin)
		if rec := do(h, req); rec.Code != http.StatusOK {
			t.Fatalf("texture #%d: got %d %s", i+1, rec.Code, rec.Body.String())
		}
	}

	// 3rd texture request from IP A gets 429 with Retry-After, rate_limited code, and CORS headers.
	req := jsonPost("/api/texture?size=256&map=normal", `{}`)
	req.Header.Set("X-Forwarded-For", ipA)
	req.Header.Set("Origin", origin)
	rec := do(h, req)
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" {
		t.Fatalf("3rd texture: got %d %s", rec.Code, rec.Body.String())
	}
	ra, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	if err != nil || ra < 1 {
		t.Errorf("invalid Retry-After header: %q", rec.Header().Get("Retry-After"))
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("CORS header missing on 429: got %q, want %q", got, origin)
	}

	// Separate IP is unaffected.
	reqB := jsonPost("/api/texture?size=256&map=normal", `{}`)
	reqB.Header.Set("X-Forwarded-For", ipB)
	if rec := do(h, reqB); rec.Code != http.StatusOK {
		t.Errorf("other IP blocked: %d %s", rec.Code, rec.Body.String())
	}

	// /api/build uses its own bucket: IP A gets 1 build, then 429 on the 2nd.
	buildReq1 := upload(t, "/api/build", testPhoto(t), "")
	buildReq1.Header.Set("X-Forwarded-For", ipA)
	if rec := do(h, buildReq1); rec.Code != http.StatusOK {
		t.Fatalf("1st build: got %d %s", rec.Code, rec.Body.String())
	}
	buildReq2 := upload(t, "/api/build", testPhoto(t), "")
	buildReq2.Header.Set("X-Forwarded-For", ipA)
	buildReq2.Header.Set("Origin", origin)
	recBuild := do(h, buildReq2)
	if recBuild.Code != http.StatusTooManyRequests || errorCode(t, recBuild) != "rate_limited" {
		t.Fatalf("2nd build: got %d %s", recBuild.Code, recBuild.Body.String())
	}
	if recBuild.Header().Get("Access-Control-Allow-Origin") != origin || recBuild.Header().Get("Retry-After") == "" {
		t.Errorf("2nd build headers: %v", recBuild.Header())
	}

	// Read-only endpoints and OPTIONS preflight are unaffected even when IP A is rate-limited.
	for _, path := range []string{"/healthz", "/api/defaults", "/api/mesh"} {
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("X-Forwarded-For", ipA)
		if rec := do(h, r); rec.Code != http.StatusOK {
			t.Errorf("GET %s affected by rate limit: %d", path, rec.Code)
		}
	}
	pre := httptest.NewRequest(http.MethodOptions, "/api/texture", nil)
	pre.Header.Set("X-Forwarded-For", ipA)
	pre.Header.Set("Origin", origin)
	pre.Header.Set("Access-Control-Request-Method", "POST")
	if rec := do(h, pre); rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS preflight affected by rate limit: %d", rec.Code)
	}
}

func TestIPLimiterRefillAndCleanup(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lim := newIPLimiter(2) // 2 req/min => 1 token per 30s
	lim.now = func() time.Time { return now }
	lim.lastCleanup = now
	lim.maxEntries = 3

	if ok, _ := lim.allow("1.1.1.1"); !ok {
		t.Fatal("1st request should pass")
	}
	if ok, _ := lim.allow("1.1.1.1"); !ok {
		t.Fatal("2nd request should pass")
	}
	if ok, retry := lim.allow("1.1.1.1"); ok || retry != 30 {
		t.Fatalf("3rd request: ok=%v retry=%d, want false 30", ok, retry)
	}

	// Advancing 30s refills 1 token.
	now = now.Add(30 * time.Second)
	if ok, _ := lim.allow("1.1.1.1"); !ok {
		t.Fatal("request after refill should pass")
	}

	// Cap maxEntries: adding more distinct IPs evicts the oldest so size never exceeds maxEntries.
	for i, ip := range []string{"2.2.2.2", "3.3.3.3", "4.4.4.4"} {
		now = now.Add(time.Second * time.Duration(i+1))
		lim.allow(ip)
	}
	if len(lim.buckets) > lim.maxEntries {
		t.Fatalf("buckets grew past maxEntries: %d > %d", len(lim.buckets), lim.maxEntries)
	}

	// Advancing past staleAfter evicts all stale entries on next call.
	now = now.Add(lim.staleAfter + time.Second)
	lim.allow("5.5.5.5")
	if len(lim.buckets) != 1 {
		t.Fatalf("expected stale buckets evicted, got %d", len(lim.buckets))
	}
}

func TestClientIP(t *testing.T) {
	cases := []struct {
		name   string
		xff    string
		remote string
		want   string
	}{
		{"xff first ip", "203.0.113.5, 10.0.0.1", "127.0.0.1:9999", "203.0.113.5"},
		{"xff ipv6", "2001:db8::1, 10.0.0.1", "127.0.0.1:9999", "2001:db8::1"},
		{"xff garbage falls back", "not-an-ip, 10.0.0.1", "198.51.100.7:4321", "198.51.100.7"},
		{"remote ipv6 hostport", "", "[::1]:4321", "::1"},
		{"remote bare ip", "", "198.51.100.8", "198.51.100.8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/texture", nil)
			req.RemoteAddr = c.remote
			if c.xff != "" {
				req.Header.Set("X-Forwarded-For", c.xff)
			}
			if got := clientIP(req); got != c.want {
				t.Errorf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}
