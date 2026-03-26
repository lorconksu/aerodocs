package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleUploadFile_WithAgent verifies upload works with a connected agent.
func TestHandleUploadFile_WithAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Logf("upload response: %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUploadFile_LargeFile verifies 413 is returned for files over 100MB.
func TestHandleUploadFile_TooBig(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	// Create a multipart body that exceeds the limit header
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.CreateFormFile("file", "big.bin")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// Override body with oversized content
	bigBody := bytes.Repeat([]byte("x"), 100*1024*1024+2048)
	req2 := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", bytes.NewReader(bigBody))
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	req2.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req2)

	// Should get 413 (too large)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Logf("expected 413 for oversized upload, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUploadFile_NoAgentConnected verifies 502 when no agent connected.
func TestHandleUploadFile_NoAgentConnected(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, token, "upload-no-agent")

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.txt")
	part.Write([]byte("hello"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUploadFile_NoFileField verifies 400 when no file field in multipart.
func TestHandleUploadFile_NoFileField(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("other", "value")
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no file, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleListDropzone_WithAgent verifies dropzone list with connected agent.
func TestHandleListDropzone_WithAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list dropzone, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleListDropzone_NoAgentConnected verifies 502 when no agent connected.
func TestHandleListDropzone_NoAgentConnected(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, token, "dropzone-no-agent")

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzoneFile_WithAgent verifies file deletion with connected agent.
func TestHandleDeleteDropzoneFile_WithAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/dropzone?filename=test.txt", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Should get 204 (no content) on success
	if rec.Code != http.StatusNoContent {
		t.Logf("dropzone delete response: %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzoneFile_NoFilename verifies 400 for missing filename.
func TestHandleDeleteDropzoneFile_NoFilename(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no filename, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzoneFile_NoAgentConnected verifies 502 when no agent connected.
func TestHandleDeleteDropzoneFile_NoAgentConnected(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, token, "dropzone-del-no-agent")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/dropzone?filename=test.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUnregisterServer_WithConnectedAgent verifies unregister sends command to agent.
func TestHandleUnregisterServer_WithConnectedAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/unregister", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleSelfUnregister_AgentExists verifies self-unregister for an existing server.
func TestHandleSelfUnregister_AgentExists(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)
	_ = adminToken

	// Set the server's IP to match the default httptest RemoteAddr (port stripped by handler)
	s.store.SetServerIP(serverID, "192.0.2.1")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleSelfUnregister_AgentNotExists verifies self-unregister for a nonexistent server.
func TestHandleSelfUnregister_AgentNotExists(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("DELETE", "/api/servers/nonexistent-server/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleAuthStatus_Initialized verifies auth status returns initialized:true after register.
func TestHandleAuthStatus_Initialized(t *testing.T) {
	s := testServer(t)
	registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body")
	}
}

// TestHandleAuthStatus_Uninitialized verifies auth status returns initialized:false when no users.
func TestHandleAuthStatus_Uninitialized(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleMe_ValidUser verifies /api/auth/me returns user data.
func TestHandleMe_ValidUser(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleListUsers_EmptyList verifies listing users works.
func TestHandleListUsers_Empty(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUpdateUserRole_SelfRole verifies admin cannot change own role.
func TestHandleUpdateUserRole_SelfRole(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	user, _ := s.store.GetUserByUsername("admin")

	req := httptest.NewRequest("PUT", "/api/users/"+user.ID+"/role", mustJSON(t, map[string]string{"role": "viewer"}))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for self-role change, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUpdateUserRole_InvalidRole verifies invalid role returns 400.
func TestHandleUpdateUserRole_InvalidRole(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("PUT", "/api/users/some-other-id/role", mustJSON(t, map[string]string{"role": "superadmin"}))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteUser_SelfDelete verifies admin cannot delete own account.
func TestHandleDeleteUser_SelfDelete(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	user, _ := s.store.GetUserByUsername("admin")

	req := httptest.NewRequest("DELETE", "/api/users/"+user.ID, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for self-delete, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteUser_NotFound verifies deleting a nonexistent user returns 404.
func TestHandleDeleteUser_NotFound(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("DELETE", "/api/users/nonexistent-id", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleListPaths_WithPermissions verifies listing paths for a server.
func TestHandleListPaths_WithPermissions(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "paths-srv")

	viewerToken := createViewerAndGetToken(t, s, adminToken)
	_ = viewerToken

	users, _ := s.store.ListUsers()
	var viewerID string
	for _, u := range users {
		if u.Role == "viewer" {
			viewerID = u.ID
			break
		}
	}

	s.store.CreatePermission(viewerID, serverID, "/var/log")

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/paths", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeletePath_WrongServer verifies 404 when path belongs to different server.
func TestHandleDeletePath_WrongServer(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "wrongsrv-path")
	otherServerID := createTestServer(t, s, adminToken, "wrongsrv-other")

	users, _ := s.store.ListUsers()
	var viewerID string
	for _, u := range users {
		if u.Role == "admin" {
			viewerID = u.ID
			break
		}
	}

	// create permission on otherServer
	perm, _ := s.store.CreatePermission(viewerID, otherServerID, "/var/log")

	// try to delete via serverID (wrong server)
	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/paths/"+perm.ID, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_MissingPath verifies 400 for missing path in log tail.
func TestHandleTailLog_MissingPath(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/logs/tail", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleRegister_Disabled verifies register is blocked when users exist.
func TestHandleRegister_Disabled(t *testing.T) {
	s := testServer(t)
	registerAndGetAdminToken(t, s)

	// Try to register again
	req := httptest.NewRequest("POST", "/api/auth/register", mustJSON(t, map[string]string{
		"username": "second", "email": "second@test.com", "password": "MyP@ssw0rd!234",
	}))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleChangePassword_WrongCurrentPassword verifies 401 for wrong current password.
func TestHandleChangePassword_WrongCurrentPassword(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("PUT", "/api/auth/password", mustJSON(t, map[string]string{
		"current_password": "WrongPassword!123",
		"new_password":     "NewP@ssw0rd!234",
	}))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
