package httpapi

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// clientIP returns the caller's address. Behind Fly's proxy the original
// client is in Fly-Client-IP, which the proxy always overwrites.
func clientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("Fly-Client-IP")); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// RateLimiter is a per-key token bucket. Limits are per machine, which is
// fine: they exist to blunt abuse, not to meter usage precisely.
type RateLimiter struct {
	mu      sync.Mutex
	every   rate.Limit
	burst   int
	buckets map[string]*bucket
}

type bucket struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewRateLimiter allows perSecond sustained requests with the given burst.
func NewRateLimiter(perSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{every: rate.Limit(perSecond), burst: burst, buckets: map[string]*bucket{}}
	go rl.janitor()
	return rl
}

// Allow reports whether key may make another request now.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[key]
	if !ok {
		b = &bucket{limiter: rate.NewLimiter(rl.every, rl.burst)}
		rl.buckets[key] = b
	}
	b.lastSeen = time.Now()
	return b.limiter.Allow()
}

func (rl *RateLimiter) janitor() {
	for range time.Tick(time.Minute) {
		rl.mu.Lock()
		for k, b := range rl.buckets {
			if time.Since(b.lastSeen) > 5*time.Minute {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

func limit(rl *RateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !rl.Allow(clientIP(r)) {
			w.Header().Set("Retry-After", "1")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(https bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		if https {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Flush keeps streaming responses (Connect server streams) working.
func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// logRequests writes one structured line per request. It deliberately logs
// the route pattern rather than the raw URL so tokens in query strings
// (OAuth callbacks) never reach the logs.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				logger.Error("panic", "panic", p, "route", r.Pattern)
				if rec.status == http.StatusOK {
					writeError(rec, http.StatusInternalServerError, "internal error")
				}
				rec.status = http.StatusInternalServerError
			}
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			logger.Info("request",
				"method", r.Method,
				"route", route,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"proto", r.Proto,
			)
		}()
		next.ServeHTTP(rec, r)
	})
}
