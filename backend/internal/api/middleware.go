package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"runtime/debug"
	"slices"
	"time"
)

// statusRecorder remembers the status code and size a handler wrote, for the access log.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.status == 0 {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func requestID() string {
	var b [8]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// logger writes one structured line per request. It never logs bodies (user photos).
func (s *Server) logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := requestID()
		w.Header().Set("X-Request-Id", id)
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		s.cfg.Logger.InfoContext(r.Context(), "request",
			"id", id, "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"bytes", rec.bytes, "ms", time.Since(start).Milliseconds())
	})
}

// recoverer turns a panic in a handler into a 500 instead of killing the whole server.
func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v) // the client went away; let net/http handle it
				}
				s.cfg.Logger.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				writeError(w, &apiError{http.StatusInternalServerError, "internal", "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors lets the frontend (another origin, e.g. https://catmod.pages.dev) call the API from the
// browser. Preflight OPTIONS requests are answered here.
func (s *Server) cors(next http.Handler) http.Handler {
	anyOrigin := slices.Contains(s.cfg.AllowedOrigins, "*")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := origin != "" && (anyOrigin || slices.Contains(s.cfg.AllowedOrigins, origin))
		if allowed {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "Content-Disposition, X-Request-Id")
		}
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if allowed {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
