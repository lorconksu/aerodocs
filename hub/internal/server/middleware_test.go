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
