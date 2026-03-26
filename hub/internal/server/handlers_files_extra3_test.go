package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/grpcserver"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

// mockGRPCStreamReadError responds with a file read error.
type mockGRPCStreamReadError struct {
	mockGRPCStream
}

func (m *mockGRPCStreamReadError) Send(msg *pb.HubMessage) error {
	if m.pending != nil {
		switch p := msg.Payload.(type) {
		case *pb.HubMessage_FileReadRequest:
			go func() {
				resp := &pb.FileReadResponse{
					RequestId: p.FileReadRequest.RequestId,
					Error:     "file not found",
				}
				m.pending.Deliver(m.serverID, p.FileReadRequest.RequestId, resp)
			}()
		}
	}
	return nil
}

// testServerWithReadErrorAgent creates a test server whose mock agent returns read errors.
func testServerWithReadErrorAgent(t *testing.T) (s *Server, adminToken, serverID string) {
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
	serverID = createTestServer(t, s, adminToken, "read-error-srv")

	stream := &mockGRPCStreamReadError{}
	stream.pending = pending
	stream.serverID = serverID
	cm.Register(serverID, stream)
	t.Cleanup(func() { cm.Unregister(serverID) })

	return s, adminToken, serverID
}

// TestHandleReadFile_AgentReturnsError verifies 404 when agent reports a read error.
func TestHandleReadFile_AgentReturnsError(t *testing.T) {
	s, adminToken, serverID := testServerWithReadErrorAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/files/read?path=/nonexistent/file.txt", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for agent read error, got %d: %s", rec.Code, rec.Body.String())
	}
}
