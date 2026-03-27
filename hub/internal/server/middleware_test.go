package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	access, _, _ := auth.GenerateTokenPair(secret, "user-1", "admin")

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := UserIDFromContext(r.Context())
		if uid != "user-1" {
			t.Fatalf("expected user-1, got %s", uid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	s := &Server{jwtSecret: "secret"}

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_WrongTokenType(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	// Generate a setup token, try to use it on an access-required endpoint
	setupToken, _ := auth.GenerateSetupToken(secret, "user-1", "admin")

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+setupToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}

	if rl.allow("1.2.3.4") {
		t.Fatal("4th attempt should be blocked")
	}

	// Different IP should still be allowed
	if !rl.allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}

func TestRateLimiter_Middleware_Blocked(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)

	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request using RemoteAddr with port
	req1 := httptest.NewRequest("POST", "/login", nil)
	req1.RemoteAddr = "10.0.0.1:12345"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request same IP, different port - should still be blocked
	req2 := httptest.NewRequest("POST", "/login", nil)
	req2.RemoteAddr = "10.0.0.1:12346"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: expected 429, got %d", rec2.Code)
	}
}

// TestRateLimiter_Middleware_XFFSpoofIgnored verifies that spoofed
// X-Forwarded-For headers do NOT bypass rate limiting.
func TestRateLimiter_Middleware_XFFSpoofIgnored(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)

	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request exhausts the limit for RemoteAddr 10.0.0.1
	req1 := httptest.NewRequest("POST", "/login", nil)
	req1.RemoteAddr = "10.0.0.1:40000"
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", rec1.Code)
	}

	// Second request: same RemoteAddr but spoofed X-Forwarded-For
	// Should still be blocked because we ignore XFF
	req2 := httptest.NewRequest("POST", "/login", nil)
	req2.RemoteAddr = "10.0.0.1:40001"
	req2.Header.Set("X-Forwarded-For", "99.99.99.99")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("spoofed XFF should not bypass rate limit: expected 429, got %d", rec2.Code)
	}
}

// TestRateLimiter_Middleware_PortStripped verifies that the same IP
// with different ports is correctly rate-limited as one client.
func TestRateLimiter_Middleware_PortStripped(t *testing.T) {
	rl := newRateLimiter(2, time.Minute)

	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Two requests from the same IP but different ports
	for i, addr := range []string{"192.168.1.1:50000", "192.168.1.1:50001"} {
		req := httptest.NewRequest("POST", "/login", nil)
		req.RemoteAddr = addr
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Third request from same IP, different port - should be blocked
	req3 := httptest.NewRequest("POST", "/login", nil)
	req3.RemoteAddr = "192.168.1.1:50002"
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("third request with different port: expected 429, got %d", rec3.Code)
	}
}

func TestCORSMiddleware_Dev(t *testing.T) {
	s := &Server{isDev: true}

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5173" {
		t.Fatal("expected CORS header in dev mode")
	}
}

func TestCORSMiddleware_DevOptions(t *testing.T) {
	s := &Server{isDev: true}

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for OPTIONS preflight")
	}))

	req := httptest.NewRequest("OPTIONS", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for OPTIONS, got %d", rec.Code)
	}
}

func TestCORSMiddleware_Production(t *testing.T) {
	s := &Server{isDev: false}

	handler := s.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("expected no CORS header in production mode")
	}
}

func TestAdminOnly_Viewer(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	_, viewerRefresh, _ := auth.GenerateTokenPair(secret, "viewer-1", "viewer")
	_ = viewerRefresh

	// Generate a viewer access token directly
	viewerAccess, _, _ := auth.GenerateTokenPair(secret, "viewer-1", "viewer")

	handler := s.authMiddleware(auth.TokenTypeAccess,
		s.adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatal("handler should not be called for viewer")
		})))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+viewerAccess)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAdminOnly_Admin(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	adminAccess, _, _ := auth.GenerateTokenPair(secret, "admin-1", "admin")

	called := false
	handler := s.authMiddleware(auth.TokenTypeAccess,
		s.adminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})))

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+adminAccess)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatal("expected handler to be called for admin")
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	s := &Server{jwtSecret: "test-secret"}

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer totally-invalid-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	called := false
	handler := loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestClientIP_IgnoresXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")

	ip := clientIP(req)
	if ip != "10.0.0.1" {
		t.Fatalf("expected '10.0.0.1' (RemoteAddr), got '%s'", ip)
	}
}

func TestClientIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	ip := clientIP(req)
	if ip != "192.168.1.1" {
		t.Fatalf("expected '192.168.1.1', got '%s'", ip)
	}
}

func TestSecurityHeaders(t *testing.T) {
	dummy := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := securityHeaders(dummy)

	t.Run("common headers on API request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/servers", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		checks := map[string]string{
			"X-Frame-Options":       "DENY",
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "strict-origin-when-cross-origin",
			"Permissions-Policy":     "camera=(), microphone=(), geolocation=()",
		}
		for header, want := range checks {
			if got := rec.Header().Get(header); got != want {
				t.Errorf("%s = %q, want %q", header, got, want)
			}
		}

		if csp := rec.Header().Get("Content-Security-Policy"); csp != "" {
			t.Errorf("CSP should not be set on /api/ paths, got %q", csp)
		}
	})

	t.Run("CSP on root path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Error("expected Content-Security-Policy on root path, got empty")
		}
	})

	t.Run("CSP on non-API path", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dashboard", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Header().Get("Content-Security-Policy") == "" {
			t.Error("expected Content-Security-Policy on non-API path, got empty")
		}
	})
}

func TestContextHelpers(t *testing.T) {
	ctx := httptest.NewRequest("GET", "/", nil).Context()

	if UserIDFromContext(ctx) != "" {
		t.Fatal("expected empty user ID from bare context")
	}
	if UserRoleFromContext(ctx) != "" {
		t.Fatal("expected empty role from bare context")
	}
	if TokenTypeFromContext(ctx) != "" {
		t.Fatal("expected empty token type from bare context")
	}
}
