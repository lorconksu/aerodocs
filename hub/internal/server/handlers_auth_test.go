package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	jwtSecret, err := InitJWTSecret(st)
	if err != nil {
		t.Fatalf("init jwt secret: %v", err)
	}

	return New(Config{
		Addr:      ":0",
		Store:     st,
		JWTSecret: jwtSecret,
		IsDev:     true,
	})
}

func TestAuthStatus_NotInitialized(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.AuthStatusResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Initialized {
		t.Fatal("expected initialized=false")
	}
}

func TestRegisterFirstUser(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin",
		Email:    "admin@test.com",
		Password: "MyP@ssw0rd!234",
	})

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["setup_token"] == nil {
		t.Fatal("expected setup_token in response")
	}
}

func TestRegisterBlocked_AfterFirstUser(t *testing.T) {
	s := testServer(t)

	// Register first user
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Try to register again
	body2, _ := json.Marshal(model.RegisterRequest{
		Username: "hacker", Email: "hacker@test.com", Password: "MyP@ssw0rd!234",
	})
	req2 := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec2.Code)
	}
}

func TestChangePassword_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "MyP@ssw0rd!234",
		NewPassword:     "NewP@ssw0rd!567",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "password updated" {
		t.Fatalf("expected status 'password updated', got '%s'", resp["status"])
	}
}

func TestChangePassword_WrongCurrent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "WrongP@ssword!1",
		NewPassword:     "NewP@ssw0rd!567",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangePassword_PolicyViolation(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		CurrentPassword: "MyP@ssw0rd!234",
		NewPassword:     "short",
	})
	req := httptest.NewRequest("PUT", "/api/auth/password", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.LoginRequest{
		Username: "nobody", Password: "wrong",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
