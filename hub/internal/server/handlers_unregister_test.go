package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// TestHandleUnregisterServer_NotFound verifies that unregistering a non-existent server returns 500.
func TestHandleUnregisterServer_NotFound(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("DELETE", "/api/servers/nonexistent-id/unregister", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// DeleteServer on a non-existent server returns an error → 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for non-existent server, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUnregisterServer_Success verifies that unregistering an existing server works.
func TestHandleUnregisterServer_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a server first
	serverID := createTestServer(t, s, token, "doomed-server")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/unregister", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["status"] != "unregistered" {
		t.Fatalf("expected status 'unregistered', got '%s'", resp["status"])
	}

	// Verify server is gone from the store
	_, err := s.store.GetServerByID(serverID)
	if err == nil {
		t.Fatal("expected server to be deleted after unregister")
	}
}

// TestHandleUnregisterServer_RequiresAdmin verifies that non-admin cannot unregister.
func TestHandleUnregisterServer_RequiresAdmin(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	serverID := createTestServer(t, s, adminToken, "protected-server")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/unregister", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer, got %d", rec.Code)
	}
}

// TestHandleSelfUnregister_NotFound verifies that self-unregister for a non-existent server returns 204.
func TestHandleSelfUnregister_NotFound(t *testing.T) {
	s := testServer(t)

	// No auth required for self-unregister
	req := httptest.NewRequest("DELETE", "/api/servers/nonexistent-id/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Already gone → 204
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204 for already-gone server, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleSelfUnregister_Success verifies that self-unregister for an existing server deletes it
// when the request comes from the matching agent IP.
func TestHandleSelfUnregister_Success(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	// Create a server to self-unregister
	serverID := createTestServer(t, s, adminToken, "self-unregister-server")

	// Set the server's IP to match the default httptest RemoteAddr (port stripped by handler)
	agentIP := "192.0.2.1"
	s.store.SetServerIP(serverID, agentIP)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify server is gone
	_, err := s.store.GetServerByID(serverID)
	if err == nil {
		t.Fatal("expected server to be deleted after self-unregister")
	}
}

// TestHandleSelfUnregister_WrongIP verifies that self-unregister is rejected when the
// request comes from a different IP than the registered agent.
func TestHandleSelfUnregister_WrongIP(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	serverID := createTestServer(t, s, adminToken, "wrong-ip-server")

	// Set a different IP than what httptest will send (192.0.2.1:1234)
	agentIP := "10.0.0.99:5678"
	s.store.SetServerIP(serverID, agentIP)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for wrong IP, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify server still exists
	_, err := s.store.GetServerByID(serverID)
	if err != nil {
		t.Fatal("expected server to still exist after rejected self-unregister")
	}
}

// TestHandleSelfUnregister_NilIP verifies that self-unregister is rejected when the
// server has no IP address recorded (IPAddress is nil).
func TestHandleSelfUnregister_NilIP(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	// Create server via API — IPAddress will be nil by default
	serverID := createTestServer(t, s, adminToken, "nil-ip-server")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/self-unregister", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for nil IP, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify server still exists
	_, err := s.store.GetServerByID(serverID)
	if err != nil {
		t.Fatal("expected server to still exist after rejected self-unregister")
	}
}

// TestHandleUnregisterServer_CleansUpAuditLog verifies an audit entry is created on unregister.
func TestHandleUnregisterServer_CleansUpAuditLog(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, token, "audit-unregister-server")

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/unregister", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify an audit log entry was created for this action
	action := model.AuditServerUnregistered
	entries, total, err := s.store.ListAuditLogs(model.AuditFilter{
		Action: &action,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if total == 0 || len(entries) == 0 {
		t.Fatal("expected at least one audit log entry for server.unregistered")
	}
}
