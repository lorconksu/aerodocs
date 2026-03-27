package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/grpcserver"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
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

// TestHandleTailLog_GrepTooLong verifies that a grep filter exceeding 256 characters returns 400.
func TestHandleTailLog_GrepTooLong(t *testing.T) {
	s := testServer(t)
	adminToken := registerAndGetAdminToken(t, s)

	longGrep := strings.Repeat("x", 257)
	req := httptest.NewRequest("GET", "/api/servers/s1/logs/tail?path=/var/log/syslog&grep="+longGrep, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for long grep filter, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleTailLog_OverflowSSE verifies the overflow SSE event is emitted when the log
// channel is full and data is dropped.
func TestHandleTailLog_OverflowSSE(t *testing.T) {
	s, adminToken, serverID := testServerWithAgentAndLogOverflow(t)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	req := httptest.NewRequest("GET", "/api/servers/"+serverID+"/logs/tail?path=/var/log/syslog", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req = req.WithContext(ctx)

	rec := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "event: overflow") {
		t.Fatalf("expected overflow SSE event in body, got: %s", body)
	}
}

// testServerWithAgentAndLogOverflow creates a test server with a mock agent stream
// that fills the log channel buffer to trigger overflow, then delivers one more chunk.
func testServerWithAgentAndLogOverflow(t *testing.T) (s *Server, adminToken, serverID string) {
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
	serverID = createTestServer(t, s, adminToken, "overflow-test-srv")

	stream := &mockGRPCStreamOverflow{
		mockGRPCStream: mockGRPCStream{pending: pending, serverID: serverID},
		logSessions:    logSessions,
	}
	cm.Register(serverID, stream)
	t.Cleanup(func() { cm.Unregister(serverID) })

	return s, adminToken, serverID
}

// mockGRPCStreamOverflow fills the log channel buffer to trigger overflow.
type mockGRPCStreamOverflow struct {
	mockGRPCStream
	logSessions *grpcserver.LogSessions
}

func (m *mockGRPCStreamOverflow) Send(msg *pb.HubMessage) error {
	m.sent = append(m.sent, msg)
	if m.pending != nil {
		switch p := msg.Payload.(type) {
		case *pb.HubMessage_LogStreamRequest:
			if m.logSessions != nil {
				go func() {
					reqID := p.LogStreamRequest.RequestId
					time.Sleep(10 * time.Millisecond)
					// Fill the channel buffer (capacity 64) to trigger overflow
					for i := 0; i < 66; i++ {
						m.logSessions.Deliver(m.serverID, reqID, []byte("filling buffer\n"))
					}
					// Give time for the SSE handler to read and detect overflow
					time.Sleep(200 * time.Millisecond)
					m.logSessions.Remove(m.serverID, reqID)
				}()
			}
		}
	}
	return nil
}
