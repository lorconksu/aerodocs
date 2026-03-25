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

func TestUpdateUserRole_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a viewer user first
	createBody, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)

	// Update role to admin
	body, _ := json.Marshal(model.UpdateRoleRequest{Role: model.RoleAdmin})
	req := httptest.NewRequest("PUT", "/api/users/"+createResp.User.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	user := resp["user"].(map[string]interface{})
	if user["role"] != "admin" {
		t.Fatalf("expected role 'admin', got '%v'", user["role"])
	}
}

func TestUpdateUserRole_CannotChangeOwnRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Get own user ID from /api/auth/me
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var me model.User
	json.NewDecoder(meRec.Body).Decode(&me)

	// Try to change own role
	body, _ := json.Marshal(model.UpdateRoleRequest{Role: model.RoleViewer})
	req := httptest.NewRequest("PUT", "/api/users/"+me.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRole_InvalidRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a user first
	createBody, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)

	// Try invalid role
	body := []byte(`{"role": "superadmin"}`)
	req := httptest.NewRequest("PUT", "/api/users/"+createResp.User.ID+"/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
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

func TestCreateUser_InvalidJSON(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateUser_InvalidUsername(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateUserRequest{
		Username: "ab", // too short
		Email:    "test@test.com",
		Role:     model.RoleViewer,
	})
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body := []byte(`{"username":"validuser","email":"test@test.com","role":"superadmin"}`)
	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateUser_Duplicate(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	req1 := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req1.Header.Set("Authorization", "Bearer "+token)
	s.routes().ServeHTTP(httptest.NewRecorder(), req1)

	body2, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer2@test.com", Role: model.RoleViewer,
	})
	req2 := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body2))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestListUsers(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["users"] == nil {
		t.Fatal("expected users in response")
	}
}

func TestDeleteUser_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a viewer user
	createBody, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1", Email: "viewer@test.com", Role: model.RoleViewer,
	})
	createReq := httptest.NewRequest("POST", "/api/users", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateUserResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)

	// Delete user
	delReq := httptest.NewRequest("DELETE", "/api/users/"+createResp.User.ID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delRec := httptest.NewRecorder()
	s.routes().ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", delRec.Code, delRec.Body.String())
	}
}

func TestDeleteUser_CannotDeleteSelf(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var me model.User
	json.NewDecoder(meRec.Body).Decode(&me)

	delReq := httptest.NewRequest("DELETE", "/api/users/"+me.ID, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delRec := httptest.NewRecorder()
	s.routes().ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", delRec.Code, delRec.Body.String())
	}
}

func TestDeleteUser_NotFound(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	delReq := httptest.NewRequest("DELETE", "/api/users/nonexistent-id", nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delRec := httptest.NewRecorder()
	s.routes().ServeHTTP(delRec, delReq)

	if delRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", delRec.Code, delRec.Body.String())
	}
}

func TestUpdateUserRole_NotFound(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.UpdateRoleRequest{Role: model.RoleAdmin})
	req := httptest.NewRequest("PUT", "/api/users/nonexistent-id/role", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateUserRole_InvalidJSON(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("PUT", "/api/users/some-id/role", bytes.NewReader([]byte("not-json")))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestViewerCannotAccessAdminEndpoints(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	viewerToken := createViewerAndGetToken(t, s, adminToken)

	// Viewer tries to list users (admin-only)
	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
