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

func registerAndGetAdminToken(t *testing.T, s *Server) string {
	t.Helper()

	// Register first admin
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	var regResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	// Setup TOTP
	req2 := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	req2.Header.Set("Authorization", "Bearer "+setupToken)
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	var totpResp model.TOTPSetupResponse
	json.NewDecoder(rec2.Body).Decode(&totpResp)

	// Generate valid TOTP code and enable
	code, _ := auth.GenerateValidCode(totpResp.Secret)

	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: code})
	req3 := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	req3.Header.Set("Authorization", "Bearer "+setupToken)
	rec3 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec3, req3)

	var authResp model.AuthResponse
	json.NewDecoder(rec3.Body).Decode(&authResp)

	return authResp.AccessToken
}

func TestCreateUser(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1",
		Email:    "viewer@test.com",
		Role:     model.RoleViewer,
	})

	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.CreateUserResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TemporaryPassword == "" {
		t.Fatal("expected temporary_password in response")
	}
	if resp.User.Username != "viewer1" {
		t.Fatalf("expected username 'viewer1', got '%s'", resp.User.Username)
	}
}
