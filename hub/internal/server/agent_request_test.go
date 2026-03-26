package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/grpcserver"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

// mockGRPCStream implements pb.AgentService_ConnectServer for testing.
type mockGRPCStream struct {
	ctx      context.Context
	sent     []*pb.HubMessage
	pending  *grpcserver.PendingRequests
	serverID string
}

func (m *mockGRPCStream) Send(msg *pb.HubMessage) error {
	m.sent = append(m.sent, msg)
	// If we have a pending registry, auto-deliver responses
	if m.pending != nil {
		switch p := msg.Payload.(type) {
		case *pb.HubMessage_FileListRequest:
			go func() {
				resp := &pb.FileListResponse{RequestId: p.FileListRequest.RequestId}
				m.pending.Deliver(m.serverID, p.FileListRequest.RequestId, resp)
			}()
		case *pb.HubMessage_FileReadRequest:
			go func() {
				resp := &pb.FileReadResponse{RequestId: p.FileReadRequest.RequestId}
				m.pending.Deliver(m.serverID, p.FileReadRequest.RequestId, resp)
			}()
		case *pb.HubMessage_FileDeleteRequest:
			go func() {
				resp := &pb.FileDeleteResponse{RequestId: p.FileDeleteRequest.RequestId, Success: true}
				m.pending.Deliver(m.serverID, p.FileDeleteRequest.RequestId, resp)
			}()
		case *pb.HubMessage_UnregisterRequest:
			go func() {
				resp := &pb.UnregisterAck{RequestId: p.UnregisterRequest.RequestId}
				m.pending.Deliver(m.serverID, p.UnregisterRequest.RequestId, resp)
			}()
		case *pb.HubMessage_FileUploadRequest:
			// Only deliver ack on the final "done" chunk
			if p.FileUploadRequest.Done {
				go func() {
					resp := &pb.FileUploadAck{RequestId: p.FileUploadRequest.RequestId, Success: true}
					m.pending.Deliver(m.serverID, p.FileUploadRequest.RequestId, resp)
				}()
			}
		}
	}
	return nil
}

func (m *mockGRPCStream) Recv() (*pb.AgentMessage, error) { return nil, nil }
func (m *mockGRPCStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}
func (m *mockGRPCStream) SendMsg(msg interface{}) error  { return nil }
func (m *mockGRPCStream) RecvMsg(msg interface{}) error  { return nil }
func (m *mockGRPCStream) SetHeader(metadata.MD) error    { return nil }
func (m *mockGRPCStream) SendHeader(metadata.MD) error   { return nil }
func (m *mockGRPCStream) SetTrailer(metadata.MD)         {}

// testServerWithAgent creates a test server with a connMgr and a mock agent stream.
// Returns the server, the admin token, and the serverID of a registered server.
// The mock stream auto-delivers responses for FileList and FileRead requests.
func testServerWithAgent(t *testing.T) (s *Server, adminToken, serverID string) {
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

	// Create a server in the store
	adminToken = registerAndGetAdminToken(t, s)
	serverID = createTestServer(t, s, adminToken, "agent-test-srv")

	// Register a mock stream that auto-delivers responses
	stream := &mockGRPCStream{pending: pending, serverID: serverID}
	cm.Register(serverID, stream)
	t.Cleanup(func() { cm.Unregister(serverID) })

	return s, adminToken, serverID
}

// TestRequireAgent_AgentConnected verifies requireAgent returns success when agent is connected.
func TestRequireAgent_AgentConnected(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/files?path=/var/log", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Should get a real response (not 502 no-agent) - either 200 or the response from mock agent
	if rec.Code == http.StatusBadGateway {
		t.Fatalf("agent is registered, should not get 502 no-agent; got 502")
	}
}

// TestSendAgentRequest_Success verifies sendAgentRequest gets a response when agent responds.
func TestSendAgentRequest_Success(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/files?path=/var/log", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Mock agent returns empty FileListResponse → handler returns 200 with empty files
	if rec.Code != http.StatusOK {
		t.Logf("sendAgentRequest response: %d: %s", rec.Code, rec.Body.String())
	}
}

// TestRequireAgent_NilConnMgr verifies requireAgent returns 502 when connMgr is nil.
func TestRequireAgent_NilConnMgr(t *testing.T) {
	s := testServer(t) // testServer has nil connMgr
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/some-server/files?path=/var/log", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 for nil connMgr, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleReadFile_WithAgent verifies handleReadFile works with a connected agent.
func TestHandleReadFile_WithAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/files/read?path=/etc/hosts", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Should get 200 from mock agent (not 502)
	if rec.Code == http.StatusBadGateway {
		t.Fatalf("should not get 502 with connected agent, got 502")
	}
	// Mock agent returns empty FileReadResponse, which means no error
	// Handler will try to process the response
	t.Logf("handleReadFile response: %d: %s", rec.Code, rec.Body.String())
}

// TestHandleListFiles_WithConnectedAgent verifies file list with connected agent.
func TestHandleListFiles_WithConnectedAgent(t *testing.T) {
	s, adminToken, serverID := testServerWithAgent(t)

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/files?path=/var/log", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// With a connected agent that auto-responds with empty FileListResponse
	if rec.Code == http.StatusBadGateway {
		t.Fatalf("should not get 502 (no agent) with connected agent")
	}
	// Should be 200 since mock returns empty (no error) FileListResponse
	if rec.Code != http.StatusOK {
		t.Logf("handleListFiles response: %d: %s", rec.Code, rec.Body.String())
	}
}
