package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHandleUploadFile_NoFile verifies that uploading without a file field returns 400.
func TestHandleUploadFile_NoFile(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Send a multipart form without any file field
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/s1/upload", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no file, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUploadFile_NoAgent verifies that a valid upload request without a connected agent returns 502.
func TestHandleUploadFile_NoAgent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Build a valid multipart form with a file
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write([]byte("hello world"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/s1/upload", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzone_NoFilename verifies that a missing filename param returns 400.
func TestHandleDeleteDropzone_NoFilename(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("DELETE", "/api/servers/s1/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing filename, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzone_NoAgent verifies that with a valid filename but no connected agent returns 502.
func TestHandleDeleteDropzone_NoAgent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("DELETE", "/api/servers/s1/dropzone?filename=test.txt", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleListDropzone_NoAgent verifies that listing dropzone without a connected agent returns 502.
func TestHandleListDropzone_NoAgent(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/s1/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for no agent, got %d: %s", rec.Code, rec.Body.String())
	}
}
