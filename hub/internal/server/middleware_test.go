package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	access, _, _ := auth.GenerateTokenPair(secret, "user-1", "admin", 0)

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

	_, viewerRefresh, _ := auth.GenerateTokenPair(secret, "viewer-1", "viewer", 0)
	_ = viewerRefresh

	// Generate a viewer access token directly
	viewerAccess, _, _ := auth.GenerateTokenPair(secret, "viewer-1", "viewer", 0)

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

	adminAccess, _, _ := auth.GenerateTokenPair(secret, "admin-1", "admin", 0)

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

func TestClientIP_UsesXForwardedFor(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.1")

	ip := clientIP(req)
	if ip != "203.0.113.1" {
		t.Fatalf("expected '203.0.113.1' (first X-Forwarded-For), got '%s'", ip)
	}
}

func TestClientIP_FallsBackToRemoteAddr(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	// No X-Forwarded-For header

	ip := clientIP(req)
	if ip != "10.0.0.1" {
		t.Fatalf("expected '10.0.0.1' (RemoteAddr fallback), got '%s'", ip)
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

// TestEvictOldest verifies that evictOldest removes the entry with the oldest
// last attempt while preserving the skipIP.
func TestEvictOldest(t *testing.T) {
	rl := newRateLimiter(10, time.Minute)

	now := time.Now()

	// Add entries with different timestamps
	rl.attempts["1.1.1.1"] = []time.Time{now.Add(-3 * time.Minute)}
	rl.attempts["2.2.2.2"] = []time.Time{now.Add(-1 * time.Minute)}
	rl.attempts["3.3.3.3"] = []time.Time{now.Add(-2 * time.Minute)}

	// Evict oldest, skipping 1.1.1.1 (oldest but should be preserved)
	rl.evictOldest("1.1.1.1")

	// 1.1.1.1 should still exist (skipped)
	if _, ok := rl.attempts["1.1.1.1"]; !ok {
		t.Fatal("expected 1.1.1.1 to be preserved (skipIP)")
	}
	// 3.3.3.3 should be evicted (oldest after skip)
	if _, ok := rl.attempts["3.3.3.3"]; ok {
		t.Fatal("expected 3.3.3.3 to be evicted")
	}
	// 2.2.2.2 should still exist
	if _, ok := rl.attempts["2.2.2.2"]; !ok {
		t.Fatal("expected 2.2.2.2 to remain")
	}
}

// TestEvictOldest_EmptyMap verifies evictOldest is a no-op on empty map.
func TestEvictOldest_EmptyMap(t *testing.T) {
	rl := newRateLimiter(10, time.Minute)
	rl.evictOldest("1.1.1.1") // should not panic
}

// TestEvictOldest_OnlySkipIP verifies evictOldest when only the skipIP exists.
func TestEvictOldest_OnlySkipIP(t *testing.T) {
	rl := newRateLimiter(10, time.Minute)
	rl.attempts["1.1.1.1"] = []time.Time{time.Now()}
	rl.evictOldest("1.1.1.1")
	if _, ok := rl.attempts["1.1.1.1"]; !ok {
		t.Fatal("expected skipIP to be preserved")
	}
}

// TestRateLimiter_EvictsWhenFull verifies that allow() evicts the oldest entry
// when the tracked IP count reaches maxTrackedIPs.
func TestRateLimiter_EvictsWhenFull(t *testing.T) {
	rl := newRateLimiter(5, time.Minute)

	// Fill the map to maxTrackedIPs
	for i := 0; i < maxTrackedIPs; i++ {
		ip := fmt.Sprintf("10.%d.%d.%d", i/(256*256), (i/256)%256, i%256)
		rl.attempts[ip] = []time.Time{time.Now()}
	}

	if len(rl.attempts) != maxTrackedIPs {
		t.Fatalf("expected %d entries, got %d", maxTrackedIPs, len(rl.attempts))
	}

	// Next allow() should evict one entry to make room
	newIP := "99.99.99.99"
	if !rl.allow(newIP) {
		t.Fatal("expected allow to succeed for new IP")
	}

	// Total should be maxTrackedIPs (one evicted, one added)
	if len(rl.attempts) != maxTrackedIPs {
		t.Fatalf("expected %d entries after eviction, got %d", maxTrackedIPs, len(rl.attempts))
	}
}

// TestRateLimiter_ExpiredEntriesCleanedUp verifies that expired entries are
// removed and the IP key is deleted when all entries expire.
func TestRateLimiter_ExpiredEntriesCleanedUp(t *testing.T) {
	rl := newRateLimiter(5, 100*time.Millisecond)

	rl.allow("1.2.3.4")
	time.Sleep(150 * time.Millisecond) // let the entry expire

	// Next call should clean up the expired entry
	if !rl.allow("1.2.3.4") {
		t.Fatal("expected allow after expiry")
	}

	// The map should have exactly one entry (the new one)
	if len(rl.attempts["1.2.3.4"]) != 1 {
		t.Fatalf("expected 1 attempt after cleanup, got %d", len(rl.attempts["1.2.3.4"]))
	}
}

// TestRateLimiter_Middleware_NoPort verifies the fallback path when
// RemoteAddr has no port (net.SplitHostPort fails).
func TestRateLimiter_Middleware_NoPort(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)

	handler := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// RemoteAddr without port
	req := httptest.NewRequest("POST", "/login", nil)
	req.RemoteAddr = "10.0.0.1" // no port
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Second request same IP (no port) - should be blocked
	req2 := httptest.NewRequest("POST", "/login", nil)
	req2.RemoteAddr = "10.0.0.1"
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}
}

// TestClientIP_SingleXFF verifies clientIP with a single X-Forwarded-For value (no comma).
func TestClientIP_SingleXFF(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.5")

	ip := clientIP(req)
	if ip != "203.0.113.5" {
		t.Fatalf("expected '203.0.113.5', got '%s'", ip)
	}
}

// TestClientIP_RemoteAddrNoPort verifies clientIP when RemoteAddr has no port.
func TestClientIP_RemoteAddrNoPort(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1" // no port

	ip := clientIP(req)
	if ip != "192.168.1.1" {
		t.Fatalf("expected '192.168.1.1', got '%s'", ip)
	}
}

// TestAuthMiddleware_BlacklistedToken verifies that a blacklisted token is rejected.
func TestAuthMiddleware_BlacklistedToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	bl := auth.NewTokenBlacklist()
	s := &Server{jwtSecret: secret, tokenBlacklist: bl}

	access, _, _ := auth.GenerateTokenPair(secret, "user-1", "admin", 0)

	// Parse the token to get its JTI and blacklist it
	claims, err := auth.ValidateToken(secret, access)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	bl.Add(claims.ID, claims.ExpiresAt.Time)

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called for blacklisted token")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
