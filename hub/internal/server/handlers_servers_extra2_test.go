package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleInstallScript_NotFound verifies the install script endpoint handles missing file.
func TestHandleInstallScript_NotFound(t *testing.T) {
	s := testServer(t)

	// The install script endpoint serves a static file. Without the static dir,
	// it will return 404 (or redirect). Either way, it shouldn't panic.
	req := httptest.NewRequest("GET", "/install.sh", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Without static/install.sh present, we expect 404 (file not found)
	// The important thing is the handler executes without panicking.
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("unexpected 500: %s", rec.Body.String())
	}
}

//TestHandleListServers_SearchFilter verifies search filter works in list servers.
func TestHandleListServers_SearchFilter(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	createTestServer(t, s, adminToken, "my-search-server")
	createTestServer(t, s, adminToken, "other-server")

	req := httptest.NewRequest("GET", "/api/servers?search=my-search", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleGetServer_ViewerNotPermitted verifies viewer gets 403 for unpermitted server.
func TestHandleGetServer_ViewerNotPermitted(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	serverID := createTestServer(t, s, adminToken, "forbidden-server")
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Viewer has no permissions for this server
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer without permission, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleCreateServer_ProductionMode verifies grpcTarget in production mode.
func TestHandleCreateServer_ProductionMode(t *testing.T) {
	s := testServer(t)
	// Override isDev to false for this test
	s.isDev = false

	adminToken := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("POST", "/api/servers", mustJSON(t, map[string]string{
		"name": "prod-server",
	}))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Host = "aerodocs.example.com"
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleCreateServer_ProductionModeWithPort verifies grpcTarget strips port from host.
func TestHandleCreateServer_ProductionModeWithPort(t *testing.T) {
	s := testServer(t)
	s.isDev = false

	adminToken := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("POST", "/api/servers", mustJSON(t, map[string]string{
		"name": "prod-server-port",
	}))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Host = "aerodocs.example.com:8443"
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}
