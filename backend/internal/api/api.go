// Package api is the HTTP layer: it turns requests into calls to the build/photo packages.
//
//	GET  /healthz        liveness probe
//	GET  /api/defaults   default parameters and limits (for the frontend's controls)
//	GET  /api/mesh       the sphere exactly as the game reads it (?lod=0..3), for the 3D preview
//	POST /api/texture    multipart photo (+ params) -> sphere texture as JPEG (?size=256..2048)
//	POST /api/build      multipart photo (+ params) -> the .vpk to import in Deadlock Mod Manager
//
// Uploads are multipart/form-data with a "photo" file and an optional "params" JSON field whose
// keys override photo.DefaultParams(). Photos are only ever held in memory, never written to disk.
package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/KJulo/Deadlock-picture-mod/backend/internal/build"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/mesh"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/photo"
	"github.com/KJulo/Deadlock-picture-mod/backend/internal/vmdl"
)

// Config holds the server's limits.
type Config struct {
	MaxUploadBytes int64         // largest accepted photo file
	MaxConcurrent  int           // builds/textures running at once (each one uses every core)
	QueueTimeout   time.Duration // how long a request may wait for a free slot before 503
	AllowedOrigins []string      // CORS: frontend origins allowed to call the API ("*" = any)
	Logger         *slog.Logger
}

// BuildFunc builds the mod. In production it is build.Mod with the loaded game files; tests pass
// a fake so they don't need the game files.
type BuildFunc func(photoData []byte, p photo.Params) (*build.Result, error)

// Server serves the API.
type Server struct {
	cfg   Config
	build BuildFunc
	slots chan struct{} // semaphore: a full channel means every slot is busy
	mesh  [][]byte      // /api/mesh response per LOD, computed once
}

// New creates a server.
func New(cfg Config, b BuildFunc) (*Server, error) {
	if cfg.MaxConcurrent < 1 || cfg.MaxUploadBytes < 1 {
		return nil, errors.New("api: MaxConcurrent and MaxUploadBytes must be positive")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	meshes := make([][]byte, len(vmdl.LODs))
	for lod := range vmdl.LODs {
		m, err := meshJSON(lod)
		if err != nil {
			return nil, err
		}
		meshes[lod] = m
	}
	return &Server{cfg: cfg, build: b, slots: make(chan struct{}, cfg.MaxConcurrent), mesh: meshes}, nil
}

// Handler returns the routes wrapped in the middlewares (outermost first: recover, log, CORS).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("GET /api/defaults", s.defaults)
	mux.HandleFunc("GET /api/mesh", s.meshHandler)
	mux.HandleFunc("POST /api/texture", s.texture)
	mux.HandleFunc("POST /api/build", s.buildHandler)
	return s.recoverer(s.logger(s.cors(securityHeaders(mux))))
}

// ---------------------------------------------------------------------------------------------
// Errors: every failure is JSON {"error": {"code": "...", "message": "..."}}.
// ---------------------------------------------------------------------------------------------

type apiError struct {
	status  int
	code    string
	message string
}

func (e *apiError) Error() string { return e.message }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, e *apiError) {
	if e.status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "10")
	}
	writeJSON(w, e.status, map[string]any{"error": map[string]string{"code": e.code, "message": e.message}})
}

// classify maps errors from the photo/build packages to HTTP errors. Anything unexpected is a
// 500 whose details are logged but not sent to the client.
func (s *Server) classify(r *http.Request, err error) *apiError {
	var ae *apiError
	switch {
	case errors.As(err, &ae):
		return ae
	case errors.Is(err, photo.ErrBadParams):
		return &apiError{http.StatusBadRequest, "bad_params", err.Error()}
	case errors.Is(err, photo.ErrBadPhoto):
		return &apiError{http.StatusUnsupportedMediaType, "bad_photo", "the photo could not be read; use JPEG, PNG, GIF or WebP"}
	case errors.Is(err, photo.ErrPhotoTooLarge):
		return &apiError{http.StatusRequestEntityTooLarge, "photo_too_large", err.Error()}
	}
	s.cfg.Logger.ErrorContext(r.Context(), "internal error", "err", err, "path", r.URL.Path)
	return &apiError{http.StatusInternalServerError, "internal", "internal error"}
}

// ---------------------------------------------------------------------------------------------
// Upload parsing
// ---------------------------------------------------------------------------------------------

const maxParamsBytes = 16 << 10

// readUpload streams the multipart body: the photo goes straight into memory (r.ParseMultipartForm
// would spill large files to temporary files on disk, which we never want for user photos).
func (s *Server) readUpload(w http.ResponseWriter, r *http.Request) ([]byte, photo.Params, error) {
	p := photo.DefaultParams()
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes+maxParamsBytes+64<<10)
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, p, &apiError{http.StatusBadRequest, "bad_request", "expected multipart/form-data with a 'photo' file"}
	}
	var data []byte
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, p, s.bodyError(err)
		}
		switch part.FormName() {
		case "photo":
			data, err = io.ReadAll(io.LimitReader(part, s.cfg.MaxUploadBytes+1))
			if err != nil {
				return nil, p, s.bodyError(err)
			}
			if int64(len(data)) > s.cfg.MaxUploadBytes {
				return nil, p, &apiError{http.StatusRequestEntityTooLarge, "photo_too_large",
					fmt.Sprintf("the photo must be at most %d MB", s.cfg.MaxUploadBytes>>20)}
			}
		case "params":
			raw, err := io.ReadAll(io.LimitReader(part, maxParamsBytes+1))
			if err != nil {
				return nil, p, s.bodyError(err)
			}
			if len(raw) > maxParamsBytes {
				return nil, p, &apiError{http.StatusBadRequest, "bad_params", "params too long"}
			}
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields() // a typo in a field name is an error, not silently ignored
			if err := dec.Decode(&p); err != nil {
				return nil, p, &apiError{http.StatusBadRequest, "bad_params", "params: " + err.Error()}
			}
		}
		part.Close()
	}
	if len(data) == 0 {
		return nil, p, &apiError{http.StatusBadRequest, "missing_photo", "no 'photo' file in the form"}
	}
	if err := p.Validate(); err != nil {
		return nil, p, err
	}
	return data, p, nil
}

func (s *Server) bodyError(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return &apiError{http.StatusRequestEntityTooLarge, "photo_too_large",
			fmt.Sprintf("the upload must be at most %d MB", s.cfg.MaxUploadBytes>>20)}
	}
	return &apiError{http.StatusBadRequest, "bad_request", "could not read the upload"}
}

// acquire waits for a free build slot; release must be called when done.
func (s *Server) acquire(ctx context.Context) (release func(), err error) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.QueueTimeout)
	defer cancel()
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-ctx.Done():
		return nil, &apiError{http.StatusServiceUnavailable, "busy", "the server is busy, try again in a few seconds"}
	}
}

// ---------------------------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------------------------

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) defaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"params":           photo.DefaultParams(),
		"max_upload_bytes": s.cfg.MaxUploadBytes,
		"texture_size":     photo.TexSize,
	})
}

func (s *Server) meshHandler(w http.ResponseWriter, r *http.Request) {
	lod := 0
	if v := r.URL.Query().Get("lod"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(s.mesh) {
			writeError(w, &apiError{http.StatusBadRequest, "bad_lod", fmt.Sprintf("lod must be within 0..%d", len(s.mesh)-1)})
			return
		}
		lod = n
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	writeBody(w, r, "application/json", s.mesh[lod])
}

func (s *Server) texture(w http.ResponseWriter, r *http.Request) {
	size := 1024
	if v := r.URL.Query().Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 256 || n > photo.TexSize || n%4 != 0 {
			writeError(w, &apiError{http.StatusBadRequest, "bad_size", "size must be a multiple of 4 within 256..2048"})
			return
		}
		size = n
	}
	data, p, err := s.readUpload(w, r)
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	release, err := s.acquire(r.Context())
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	defer release()

	img, err := photo.Load(data, p)
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	tex, err := photo.SphereTexture(img, p, size)
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, tex, &jpeg.Options{Quality: 90}); err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(buf.Bytes())
}

func (s *Server) buildHandler(w http.ResponseWriter, r *http.Request) {
	data, p, err := s.readUpload(w, r)
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	release, err := s.acquire(r.Context())
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	defer release()

	res, err := s.build(data, p)
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	pak, err := res.VPK()
	if err != nil {
		writeError(w, s.classify(r, err))
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="catmod_soul_container.vpk"`)
	w.Header().Set("Cache-Control", "no-store")
	writeBody(w, r, "application/octet-stream", pak)
}

// acceptsGzip reports whether the client accepts gzip (browsers always do).
func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if strings.EqualFold(strings.TrimSpace(token), "gzip") {
			return strings.ReplaceAll(params, " ", "") != "q=0"
		}
	}
	return false
}

// writeBody sends data, gzip-compressed when the client accepts it. The VPK shrinks from 14 MB to
// well under 1 MB (two of its textures are a constant colour), which saves bandwidth = money.
func writeBody(w http.ResponseWriter, r *http.Request, contentType string, data []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Add("Vary", "Accept-Encoding")
	if acceptsGzip(r) && len(data) > 1024 {
		var buf bytes.Buffer
		zw, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
		zw.Write(data)
		if zw.Close() == nil {
			w.Header().Set("Content-Encoding", "gzip")
			data = buf.Bytes()
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Write(data)
}

// meshJSON renders one LOD of the sphere as the game decodes it: positions (Source units, Z up),
// UVs decoded from the stored half floats, and only the real (non-padding) triangles.
func meshJSON(lodIndex int) ([]byte, error) {
	lod := vmdl.LODs[lodIndex]
	vb, idx, grid, err := mesh.BuildBuffers(lod.VertexCount, lod.IndexCount, vmdl.SphereRadius)
	if err != nil {
		return nil, err
	}
	n := grid.Vertices()
	positions, uvs := make([]float32, 0, n*3), make([]float64, 0, n*2)
	for k := 0; k < n; k++ {
		v := vb[k*mesh.VertexSize:]
		for c := 0; c < 3; c++ {
			positions = append(positions, math.Float32frombits(binary.LittleEndian.Uint32(v[4*c:])))
		}
		uvs = append(uvs, mesh.HalfToFloat(binary.LittleEndian.Uint16(v[12:])),
			mesh.HalfToFloat(binary.LittleEndian.Uint16(v[14:])))
	}
	return json.Marshal(map[string]any{
		"lod": lodIndex, "grid": []int{grid.NU, grid.NV}, "up": "z", "radius": vmdl.SphereRadius,
		"positions": positions, "uvs": uvs, "indices": idx[:3*grid.Triangles()],
	})
}
