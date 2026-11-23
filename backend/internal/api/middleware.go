package api

import (
	"crypto/rand"
	"encoding/hex"
	"math"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
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
			h.Set("Access-Control-Expose-Headers", "Content-Disposition, X-Request-Id, Retry-After")
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

const (
	rateLimitStaleAfter      = 2 * time.Minute
	rateLimitMaxEntries      = 10_000
	rateLimitCleanupThrottle = time.Second
)

type ipBucket struct {
	tokens   float64
	lastSeen time.Time
}

// ipLimiter is an in-memory per-IP token bucket with lazy stale-entry eviction and a hard entry cap.
type ipLimiter struct {
	mu          sync.Mutex
	rate        float64 // tokens per second
	burst       float64 // bucket capacity (full bucket on first seen)
	staleAfter  time.Duration
	maxEntries  int
	lastCleanup time.Time
	buckets     map[string]*ipBucket
	now         func() time.Time
}

func newIPLimiter(rpm int) *ipLimiter {
	now := time.Now
	return &ipLimiter{
		rate:        float64(rpm) / 60.0,
		burst:       float64(rpm),
		staleAfter:  rateLimitStaleAfter,
		maxEntries:  rateLimitMaxEntries,
		lastCleanup: now(),
		buckets:     make(map[string]*ipBucket),
		now:         now,
	}
}

func (l *ipLimiter) allow(ip string) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if now.Sub(l.lastCleanup) >= l.staleAfter {
		l.cleanupLocked(now)
	}
	b, ok := l.buckets[ip]
	if !ok {
		if len(l.buckets) >= l.maxEntries {
			if now.Sub(l.lastCleanup) >= rateLimitCleanupThrottle {
				l.cleanupLocked(now)
			}
			if len(l.buckets) >= l.maxEntries {
				l.evictOldestLocked()
			}
		}
		b = &ipBucket{tokens: l.burst, lastSeen: now}
		l.buckets[ip] = b
	} else {
		if elapsed := now.Sub(b.lastSeen).Seconds(); elapsed > 0 {
			b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
		}
		if now.After(b.lastSeen) {
			b.lastSeen = now
		}
	}
	if b.tokens >= 1 {
		b.tokens -= 1
		return true, 0
	}
	retry := int(math.Ceil((1 - b.tokens) / l.rate))
	if retry < 1 {
		retry = 1
	}
	return false, retry
}

func (l *ipLimiter) cleanupLocked(now time.Time) {
	for ip, b := range l.buckets {
		if now.Sub(b.lastSeen) >= l.staleAfter {
			delete(l.buckets, ip)
		}
	}
	l.lastCleanup = now
}

func (l *ipLimiter) evictOldestLocked() {
	var oldestIP string
	var oldestTime time.Time
	first := true
	for ip, b := range l.buckets {
		if first || b.lastSeen.Before(oldestTime) {
			oldestIP = ip
			oldestTime = b.lastSeen
			first = false
		}
	}
	if !first {
		delete(l.buckets, oldestIP)
	}
}

// clientIP extracts the client's IP address, preferring the rightmost valid IP in X-Forwarded-For
// (Cloud Run / Google Front End appends the real client IP on the right) and falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	values := r.Header.Values("X-Forwarded-For")
	for i := len(values) - 1; i >= 0; i-- {
		parts := strings.Split(values[i], ",")
		for j := len(parts) - 1; j >= 0; j-- {
			entry := strings.TrimSpace(parts[j])
			if entry == "" {
				continue
			}
			if addr, err := netip.ParseAddr(entry); err == nil {
				return addr.Unmap().String()
			}
			if host, _, err := net.SplitHostPort(entry); err == nil {
				if addr, err := netip.ParseAddr(host); err == nil {
					return addr.Unmap().String()
				}
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap().String()
		}
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// rateLimit enforces per-IP rate limits on expensive endpoints (POST /api/texture and POST /api/build).
// Read-only endpoints (/healthz, /api/defaults, /api/mesh) and OPTIONS preflights are exempt.
func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var lim *ipLimiter
		if r.Method == http.MethodPost {
			switch r.URL.Path {
			case "/api/texture":
				lim = s.textureLimiter
			case "/api/build":
				lim = s.buildLimiter
			}
		}
		if lim != nil {
			if ok, retryAfter := lim.allow(clientIP(r)); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				writeError(w, &apiError{http.StatusTooManyRequests, "rate_limited", "too many requests, try again in a few seconds"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
