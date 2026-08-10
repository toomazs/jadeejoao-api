package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/jadeejoao/jadeejoao-api/internal/platform"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// RequestIDFromContext returns the request id injected by requestID middleware,
// or "" when absent (e.g. in tests).
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// requestID assigns a UUID to every request, exposing it to handlers via
// context and to clients via the X-Request-Id response header.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewString()
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// logRequests emits one structured JSON line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("http request",
			"request_id", RequestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", clientIP(r),
		)
	})
}

// recoverPanics converts panics into a minimal RFC 9457 response instead of
// killing the connection.
func recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "request_id", RequestIDFromContext(r.Context()), "panic", rec)
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"title":"Internal Server Error","status":500}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// cors implements the allowlist CORS policy: the two Cloudflare Pages origins
// plus localhost dev ports, from CORS_ORIGINS.
func cors(allowed []string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		allowedSet[strings.TrimRight(o, "/")] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && allowedSet[strings.TrimRight(origin, "/")] {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Set("Vary", "Origin")
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				h.Set("Access-Control-Max-Age", "86400")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// publicPostRateLimit guards every public POST (lookup, RSVP, contributions,
// messages) with a per-IP token bucket. Admin routes are exempt — they are
// JWT-guarded. 429 responses use RFC 9457 with a PT-BR detail.
func publicPostRateLimit(limiter *platform.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			limited := r.Method == http.MethodPost &&
				strings.HasPrefix(r.URL.Path, platform.APIBase+"/") &&
				!strings.HasPrefix(r.URL.Path, platform.APIBase+"/admin")
			if limited && !limiter.Allow(clientIP(r)) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"title":"Too Many Requests","status":429,"detail":"Muitas tentativas em pouco tempo. Aguarde um instante e tente de novo."}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP extracts the caller IP. Railway fronts the service with a proxy
// that sets X-Forwarded-For; fall back to RemoteAddr for local runs.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host := r.RemoteAddr
	if i := strings.LastIndex(host, ":"); i > 0 {
		host = host[:i]
	}
	return host
}
