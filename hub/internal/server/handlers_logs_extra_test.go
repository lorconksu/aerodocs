package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// flusherRecorder wraps httptest.ResponseRecorder and implements http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flusherRecorder) Flush() {
	f.flushed = true
}

// TestHandleTailLog_ViewerWithPermissionNoAgent verifies viewer with permission
// but no agent gets 502 (passes the auth check but fails at agent connectivity).
func TestHandleTailLog_ViewerWithPermissionNoAgent(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	serverID := createTestServer(t, s, adminToken, "srv-tail-perm")

	viewerToken := createViewerAndGetToken(t, s, adminToken)
	meReq := httptest.NewRequest("GET", "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+viewerToken)
	meRec := httptest.NewRecorder()
	s.routes().ServeHTTP(meRec, meReq)

	var viewerUser interface{}
	json.NewDecoder(meRec.Body).Decode(&viewerUser)
	viewerMap := viewerUser.(map[string]interface{})
	viewerID := viewerMap["id"].(string)

	// Grant permission for /var/log on this server
	s.store.CreatePermission(viewerID, serverID, "/var/log")

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/logs/tail?path=/var/log/syslog", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Should fail at agent connectivity (502) not permission (403)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("viewer with permission should not get 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (no agent), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_AdminWithNoAgent verifies admin with valid path but no agent gets 502.
func TestHandleTailLog_AdminWithNoAgent(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "srv-tail-admin")

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/logs/tail?path=/var/log/syslog", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Should reach the agent check and return 502
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}
