package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleGetUserPaths_ViewerNoPaths verifies a viewer with no paths for a server gets empty list.
func TestHandleGetUserPaths_ViewerNoPaths(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "paths-viewer-empty")

	// Create viewer but grant NO permissions for this server
	viewerToken := createViewerAndGetToken(t, s, adminToken)

	req := httptest.NewRequest("GET", testServersPrefix+serverID+"/my-paths", nil)
	req.Header.Set("Authorization", testBearerPrefix+viewerToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(testExpected200Body, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	paths := resp["paths"].([]interface{})
	if len(paths) != 0 {
		t.Fatalf("expected empty paths for viewer with no permissions, got %v", paths)
	}
}

// TestHandleCreatePath_Conflict verifies 409 when permission already exists.
func TestHandleCreatePath_Conflict(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "path-conflict-srv")
	viewerID, _ := createTestViewer(t, s, adminToken, "pathconflictviewer", "pcv@test.com")

	// Create the permission once
	createReq := httptest.NewRequest("POST", testServersPrefix+serverID+testPathsSuffix, mustJSON(t, map[string]string{
		"user_id": viewerID,
		"path":    testVarLog,
	}))
	createReq.Header.Set("Authorization", testBearerPrefix+adminToken)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("first create should succeed, got %d: %s", createRec.Code, createRec.Body.String())
	}

	// Try to create the same permission again → 409
	createReq2 := httptest.NewRequest("POST", testServersPrefix+serverID+testPathsSuffix, mustJSON(t, map[string]string{
		"user_id": viewerID,
		"path":    testVarLog,
	}))
	createReq2.Header.Set("Authorization", testBearerPrefix+adminToken)
	createRec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec2, createReq2)

	if createRec2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate path, got %d: %s", createRec2.Code, createRec2.Body.String())
	}
}

// TestHandleListPaths_Success verifies listing paths returns all permissions for a server.
func TestHandleListPaths_Success(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "list-paths-srv")
	viewerID, _ := createTestViewer(t, s, adminToken, "listpathsviewer", "lpv@test.com")

	// Grant two paths
	s.store.CreatePermission(viewerID, serverID, testVarLog)
	s.store.CreatePermission(viewerID, serverID, "/etc")

	req := httptest.NewRequest("GET", testServersPrefix+serverID+testPathsSuffix, nil)
	req.Header.Set("Authorization", testBearerPrefix+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(testExpected200Body, rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	paths := resp["paths"].([]interface{})
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d: %v", len(paths), paths)
	}
}

// TestHandleDeletePath_DeleteFails verifies the delete permission failure path.
// We can simulate this by deleting the permission before calling the endpoint.
func TestHandleDeletePath_AlreadyDeleted(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)
	serverID := createTestServer(t, s, adminToken, "path-delete-fail-srv")
	viewerID, _ := createTestViewer(t, s, adminToken, "delfailviewer", "dfv@test.com")

	// Create the permission
	perm, err := s.store.CreatePermission(viewerID, serverID, testVarLog)
	if err != nil {
		t.Fatalf("create permission: %v", err)
	}

	// Delete it via store directly first
	if err := s.store.DeletePermission(perm.ID); err != nil {
		t.Fatalf("store delete: %v", err)
	}

	// Now try to delete via API — GetPermissionByID should return 404
	req := httptest.NewRequest("DELETE", testServersPrefix+serverID+"/paths/"+perm.ID, nil)
	req.Header.Set("Authorization", testBearerPrefix+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for already-deleted permission, got %d: %s", rec.Code, rec.Body.String())
	}
}
