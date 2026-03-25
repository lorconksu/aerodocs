package server

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/grpcserver"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

// mockGRPCStreamDropzoneError is a mock that returns a FileListResponse with an error,
// to test the "dropzone dir not found → return empty list" branch.
type mockGRPCStreamDropzoneError struct {
	mockGRPCStream
}

func (m *mockGRPCStreamDropzoneError) Send(msg *pb.HubMessage) error {
	if m.pending != nil {
		switch p := msg.Payload.(type) {
		case *pb.HubMessage_FileListRequest:
			go func() {
				// Return an error to simulate the dropzone dir not existing
				resp := &pb.FileListResponse{
					RequestId: p.FileListRequest.RequestId,
					Error:     "no such file or directory",
				}
				m.pending.Deliver(p.FileListRequest.RequestId, resp)
			}()
		case *pb.HubMessage_FileDeleteRequest:
			go func() {
				resp := &pb.FileDeleteResponse{
					RequestId: p.FileDeleteRequest.RequestId,
					Success:   false,
					Error:     "file not found",
				}
				m.pending.Deliver(p.FileDeleteRequest.RequestId, resp)
			}()
		case *pb.HubMessage_FileUploadRequest:
			if p.FileUploadRequest.Done {
				go func() {
					resp := &pb.FileUploadAck{
						RequestId: p.FileUploadRequest.RequestId,
						Success:   false,
						Error:     "disk full",
					}
					m.pending.Deliver(p.FileUploadRequest.RequestId, resp)
				}()
			}
		}
	}
	return nil
}

// testServerWithErrorAgent creates a test server whose mock agent returns errors for file operations.
func testServerWithErrorAgent(t *testing.T) (s *Server, adminToken, serverID string) {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	jwtSecret, err := InitJWTSecret(st)
	if err != nil {
		t.Fatalf("init jwt secret: %v", err)
	}

	cm := connmgr.New()
	pending := grpcserver.NewPendingRequests()
	logSessions := grpcserver.NewLogSessions()

	s = New(Config{
		Addr:        ":0",
		Store:       st,
		JWTSecret:   jwtSecret,
		IsDev:       true,
		ConnMgr:     cm,
		Pending:     pending,
		LogSessions: logSessions,
	})

	adminToken = registerAndGetAdminToken(t, s)
	serverID = createTestServer(t, s, adminToken, "error-agent-srv")

	stream := &mockGRPCStreamDropzoneError{}
	stream.pending = pending
	cm.Register(serverID, stream)
	t.Cleanup(func() { cm.Unregister(serverID) })

	return s, adminToken, serverID
}

// TestHandleListDropzone_WithErrorResponse verifies that a dropzone dir-not-found
// agent error returns 200 with empty file list (graceful fallback).
func TestHandleListDropzone_WithErrorResponse(t *testing.T) {
	s, adminToken, serverID := testServerWithErrorAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/dropzone", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Dropzone error → should return 200 with empty files list (graceful)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for dropzone error (graceful fallback), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleDeleteDropzone_FailureResponse verifies 500 when agent reports deletion failure.
func TestHandleDeleteDropzone_FailureResponse(t *testing.T) {
	s, adminToken, serverID := testServerWithErrorAgent(t)

	req := httptest.NewRequest("DELETE", "/api/servers/"+serverID+"/dropzone?filename=test.txt", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Agent returned Success=false → expect 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failed dropzone delete, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUploadFile_AgentFailure verifies 500 when agent returns upload failure.
func TestHandleUploadFile_AgentFailure(t *testing.T) {
	s, adminToken, serverID := testServerWithErrorAgent(t)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "fail.txt")
	part.Write([]byte("data"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/servers/"+serverID+"/upload", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Agent returned Success=false → expect 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failed upload, got %d: %s", rec.Code, rec.Body.String())
	}
}
