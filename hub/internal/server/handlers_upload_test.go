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

// TestDropzoneDeletePathTraversal verifies that path traversal filenames are sanitized.
func TestDropzoneDeletePathTraversal(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	traversalNames := []string{
		"../../etc/passwd",
		"../../../etc/shadow",
		"..%2F..%2Fetc%2Fpasswd", // won't be decoded by Go but test anyway
		"/etc/passwd",
		"foo/../../../etc/passwd",
	}

	for _, name := range traversalNames {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest("DELETE", "/api/servers/s1/dropzone?filename="+name, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, req)

			// After sanitization with filepath.Base, the filename should be safe.
			// The request should proceed past validation (502 = no agent, which means
			// the filename was accepted but stripped of directory components).
			// It should NOT return 400 "invalid filename" for these cases because
			// filepath.Base extracts a valid base name (e.g. "passwd").
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected 502 (no agent) after sanitization for %q, got %d: %s",
					name, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestDropzoneDeleteDotFilename verifies that "." and "/" filenames are rejected.
func TestDropzoneDeleteDotFilename(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// filepath.Base(".") returns ".", filepath.Base("/") returns "/"
	// but we only get "." from query param since "/" is part of URL path
	req := httptest.NewRequest("DELETE", "/api/servers/s1/dropzone?filename=.", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for dot filename, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestDropzoneUploadPathTraversal verifies that upload filenames with path traversal are sanitized.
func TestDropzoneUploadPathTraversal(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	traversalNames := []string{
		"../../etc/passwd",
		"../../../etc/shadow",
		"/etc/passwd",
		"foo/../../../etc/passwd",
	}

	for _, name := range traversalNames {
		t.Run(name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", name)
			if err != nil {
				t.Fatalf("create form file: %v", err)
			}
			part.Write([]byte("malicious content"))
			writer.Close()

			req := httptest.NewRequest("POST", "/api/servers/s1/upload", body)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			rec := httptest.NewRecorder()
			s.routes().ServeHTTP(rec, req)

			// After filepath.Base sanitization, the filename is safe and the request
			// proceeds to agent send (502 = no agent connected).
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("expected 502 (no agent) after sanitization for %q, got %d: %s",
					name, rec.Code, rec.Body.String())
			}
		})
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
