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

// mustJSON encodes v as JSON and returns a *bytes.Reader. Fails the test on error.
func mustJSON(t *testing.T, v interface{}) *bytes.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return bytes.NewReader(b)
}

// createViewerAndGetToken creates a viewer user and returns a valid access token for them.
func createViewerAndGetToken(t *testing.T, s *Server, adminToken string) string {
	t.Helper()

	// Create viewer user
	body, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewertest", Email: "viewertest@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	createReq.Header.Set("Authorization", "Bearer "+adminToken)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("createViewerAndGetToken: create user failed with %d: %s", createRec.Code, createRec.Body.String())
	}

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	tempPassword := createResp.TemporaryPassword

	// Login with temp password — first need to set up TOTP
	loginBody, _ := json.Marshal(model.LoginRequest{
		Username: "viewertest",
		Password: tempPassword,
	})
	loginReq := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.routes().ServeHTTP(loginRec, loginReq)

	var loginResp model.LoginResponse
	json.NewDecoder(loginRec.Body).Decode(&loginResp)

	// Setup TOTP
	setupReq := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	setupReq.Header.Set("Authorization", "Bearer "+loginResp.SetupToken)
	setupRec := httptest.NewRecorder()
	s.routes().ServeHTTP(setupRec, setupReq)

	var totpResp model.TOTPSetupResponse
	json.NewDecoder(setupRec.Body).Decode(&totpResp)

	// Enable TOTP
	code, _ := auth.GenerateValidCode(totpResp.Secret)
	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: code})
	enableReq := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	enableReq.Header.Set("Authorization", "Bearer "+loginResp.SetupToken)
	enableRec := httptest.NewRecorder()
	s.routes().ServeHTTP(enableRec, enableReq)

	var authResp model.AuthResponse
	json.NewDecoder(enableRec.Body).Decode(&authResp)

	return authResp.AccessToken
}
