package server

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

type contextKey string

const (
	ctxUserID    contextKey = "user_id"
	ctxUserRole  contextKey = "user_role"
	ctxTokenType contextKey = "token_type"
)

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserRole).(string)
	return v
}

func TokenTypeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTokenType).(string)
	return v
}

// authMiddleware validates JWT from Authorization header and enforces token type.
func (s *Server) authMiddleware(requiredType string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if tokenStr == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		claims, err := auth.ValidateToken(s.jwtSecret, tokenStr)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if claims.TokenType != requiredType {
			respondError(w, http.StatusForbidden, "invalid token type for this endpoint")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, claims.Subject)
		ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
		ctx = context.WithValue(ctx, ctxTokenType, claims.TokenType)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminOnly wraps a handler to require admin role.
func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserRoleFromContext(r.Context()) != "admin" {
			respondError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// corsMiddleware adds CORS headers for development.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isDev {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter tracks login attempts per IP.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter expired entries
	valid := rl.attempts[ip][:0]
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Clean up empty entries to prevent memory leak
	if len(valid) == 0 {
		delete(rl.attempts, ip)
	} else {
		rl.attempts[ip] = valid
	}

	if len(valid) >= rl.limit {
		return false
	}

	rl.attempts[ip] = append(valid, now)
	return true
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always use RemoteAddr for rate limiting - never trust X-Forwarded-For
		// When behind Traefik, RemoteAddr is set to the real client IP
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr // fallback if no port
		}

		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			respondError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds standard security headers to every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.URL.Path == "/" || !strings.HasPrefix(r.URL.Path, "/api/") {
			// CSP for HTML pages only, not API responses
			w.Header().Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP from the request.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}
