package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleTailLog_NoPath verifies that a missing path parameter returns 400.
func TestHandleTailLog_NoPath(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/s1/logs/tail", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing path, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_PathTraversal verifies that path traversal is rejected with 400.
func TestHandleTailLog_PathTraversal(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/s1/logs/tail?path=/../etc/passwd", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_NoAgent verifies that a valid path without a connected agent returns 502.
func TestHandleTailLog_NoAgent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/s1/logs/tail?path=/var/log/syslog", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_ViewerPermissionDenied verifies that a viewer with no paths gets 403.
func TestHandleTailLog_ViewerPermissionDenied(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	viewerToken := createViewerAndGetToken(t, s, adminToken)

	req := httptest.NewRequest("GET", "/api/servers/s1/logs/tail?path=/var/log/syslog", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer with no paths, got %d: %s", rec.Code, rec.Body.String())
	}
}
