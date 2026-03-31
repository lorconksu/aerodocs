package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

// --- Vulnerability #30: Refresh token rotation ---

func TestRefreshToken_RotatesGeneration(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	user, _ := s.store.GetUserByUsername("admin")
	_, refreshToken, _ := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role), user.TokenGeneration)

	// First refresh should succeed
	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: refreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Using the SAME refresh token a second time should fail (generation was incremented)
	body2, _ := json.Marshal(model.RefreshRequest{RefreshToken: refreshToken})
	req2 := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("reuse of old refresh token: expected 401, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestRefreshToken_NewTokenWorks(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	user, _ := s.store.GetUserByUsername("admin")
	_, refreshToken, _ := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role), user.TokenGeneration)

	// First refresh
	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: refreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("first refresh: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var pair model.TokenPair
	json.NewDecoder(rec.Body).Decode(&pair)

	// The new refresh token from the response should work
	body2, _ := json.Marshal(model.RefreshRequest{RefreshToken: pair.RefreshToken})
	req2 := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("second refresh with new token: expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestRefreshToken_StaleGeneration_Rejected(t *testing.T) {
	s := testServer(t)
	_ = registerAndGetAdminToken(t, s)

	user, _ := s.store.GetUserByUsername("admin")

	// Create a refresh token with generation 0
	_, refreshToken, _ := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role), 0)

	// Manually increment the generation in the DB (simulating another refresh or admin action)
	s.store.IncrementTokenGeneration(user.ID)

	// The old refresh token should now be rejected
	body, _ := json.Marshal(model.RefreshRequest{RefreshToken: refreshToken})
	req := httptest.NewRequest("POST", "/api/auth/refresh", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("stale generation token: expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// --- Vulnerability #31: Logout token blacklist ---

func TestLogout_BlacklistsAccessToken(t *testing.T) {
	s := testServer(t)
	accessToken := registerAndGetAdminToken(t, s)

	// Verify the access token works before logout
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+accessToken)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("before logout: expected 200, got %d", meRec.Code)
	}

	// Logout with the access token
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+accessToken)
	logoutRec := httptest.NewRecorder()
	s.routes().ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d", logoutRec.Code)
	}

	// The access token should now be rejected
	meReq2 := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq2.Header.Set("Authorization", "Bearer "+accessToken)
	meRec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec2, meReq2)

	if meRec2.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: expected 401, got %d: %s", meRec2.Code, meRec2.Body.String())
	}
}

func TestLogout_WithoutToken_StillClears(t *testing.T) {
	s := testServer(t)

	// Logout without any token should still succeed (just clears cookies)
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutRec := httptest.NewRecorder()
	s.routes().ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout without token: expected 204, got %d", logoutRec.Code)
	}
}

func TestLogout_DifferentToken_StillValid(t *testing.T) {
	s := testServer(t)
	accessToken := registerAndGetAdminToken(t, s)

	// Generate a second token pair for the same user
	user, _ := s.store.GetUserByUsername("admin")
	accessToken2, _, _ := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role), user.TokenGeneration)

	// Logout with the first token
	logoutReq := httptest.NewRequest("POST", "/api/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+accessToken)
	logoutRec := httptest.NewRecorder()
	s.routes().ServeHTTP(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: expected 204, got %d", logoutRec.Code)
	}

	// The second access token should still work (different JTI)
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+accessToken2)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("second token after first logout: expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
}

// Test that JTI is included in generated tokens
func TestTokens_HaveJTI(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	access, refresh, err := auth.GenerateTokenPair(secret, "user-1", "admin", 0)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	accessClaims, _ := auth.ValidateToken(secret, access)
	if accessClaims.ID == "" {
		t.Fatal("access token should have JTI")
	}

	refreshClaims, _ := auth.ValidateToken(secret, refresh)
	if refreshClaims.ID == "" {
		t.Fatal("refresh token should have JTI")
	}

	if accessClaims.ID == refreshClaims.ID {
		t.Fatal("access and refresh tokens should have different JTIs")
	}
}

// Test that token generation is included in claims
func TestTokens_HaveGeneration(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	access, _, err := auth.GenerateTokenPair(secret, "user-1", "admin", 5)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, _ := auth.ValidateToken(secret, access)
	if claims.TokenGeneration != 5 {
		t.Fatalf("expected generation 5, got %d", claims.TokenGeneration)
	}
}
