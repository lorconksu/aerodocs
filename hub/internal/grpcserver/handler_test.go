package grpcserver

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

// mockStream implements pb.AgentService_ConnectServer for testing.
type mockStream struct {
	ctx     context.Context
	sent    []*pb.HubMessage
	sendErr error
}

func (m *mockStream) Send(msg *pb.HubMessage) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockStream) Recv() (*pb.AgentMessage, error) {
	return nil, nil
}

func (m *mockStream) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

func (m *mockStream) SendMsg(msg interface{}) error  { return nil }
func (m *mockStream) RecvMsg(msg interface{}) error  { return nil }
func (m *mockStream) SetHeader(metadata.MD) error    { return nil }
func (m *mockStream) SendHeader(metadata.MD) error   { return nil }
func (m *mockStream) SetTrailer(metadata.MD)         {}
func (m *mockStream) SendAndClose(*pb.HubMessage) error { return nil }

func testHandler(t *testing.T) (*Handler, *store.Store) {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	cm := connmgr.New()
	h := &Handler{
		store:   st,
		connMgr: cm,
	}
	return h, st
}

func TestHandleRegister_ValidToken(t *testing.T) {
	h, st := testHandler(t)
	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	expiresAt := "2099-12-31 23:59:59"
	st.CreateServer(&model.Server{
		ID: "s1", Name: "test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})
	serverID, err := h.handleRegister("", "host1", "10.0.0.1", "Linux", "0.1.0")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if serverID != "s1" {
		t.Fatalf("expected 's1', got '%s'", serverID)
	}
	srv, _ := st.GetServerByID("s1")
	if srv.Status != "online" {
		t.Fatalf("expected 'online', got '%s'", srv.Status)
	}
}

func TestHandleRegister_InvalidToken(t *testing.T) {
	h, _ := testHandler(t)
	_, err := h.handleRegister("totally-fake", "host1", "10.0.0.1", "Linux", "0.1.0")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestHandleHeartbeat_ValidServer(t *testing.T) {
	h, st := testHandler(t)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "offline", Labels: "{}"})
	err := h.handleHeartbeat("s1")
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	srv, _ := st.GetServerByID("s1")
	if srv.Status != "online" {
		t.Fatalf("expected 'online', got '%s'", srv.Status)
	}
}

func TestHandleHeartbeat_UnknownServer(t *testing.T) {
	h, _ := testHandler(t)
	err := h.handleHeartbeat("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}

func TestHandleHeartbeat_AlreadyOnline(t *testing.T) {
	h, st := testHandler(t)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})
	err := h.handleHeartbeat("s1")
	if err != nil {
		t.Fatalf("heartbeat for online server: %v", err)
	}
}

func TestHandleRegister_ExpiredToken(t *testing.T) {
	h, st := testHandler(t)
	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	expiresAt := "2000-01-01 00:00:00" // expired
	st.CreateServer(&model.Server{
		ID: "s1", Name: "test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})
	_, err := h.handleRegister("", "host1", "10.0.0.1", "Linux", "0.1.0")
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestRouteAgentMessage_Heartbeat(t *testing.T) {
	h, st := testHandler(t)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	// Register s1 in connMgr
	stream := &mockStream{}
	h.connMgr.Register("s1", stream)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_Heartbeat{
			Heartbeat: &pb.Heartbeat{ServerId: "s1"},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route heartbeat: %v", err)
	}
	if len(stream.sent) == 0 {
		t.Fatal("expected heartbeat ack to be sent")
	}
}

func TestRouteAgentMessage_FileListResponse(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()

	stream := &mockStream{}
	reqID := "req-file-list-1"
	ch := h.pending.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileListResponse{
			FileListResponse: &pb.FileListResponse{RequestId: reqID},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route file list response: %v", err)
	}

	select {
	case delivered := <-ch:
		if delivered == nil {
			t.Fatal("expected delivered message")
		}
	default:
		t.Fatal("expected message to be delivered to pending channel")
	}
}

func TestRouteAgentMessage_FileReadResponse(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()

	stream := &mockStream{}
	reqID := "req-file-read-1"
	ch := h.pending.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileReadResponse{
			FileReadResponse: &pb.FileReadResponse{RequestId: reqID},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route file read response: %v", err)
	}

	select {
	case delivered := <-ch:
		if delivered == nil {
			t.Fatal("expected delivered message")
		}
	default:
		t.Fatal("expected message to be delivered to pending channel")
	}
}

func TestRouteAgentMessage_FileUploadAck(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()

	stream := &mockStream{}
	reqID := "req-upload-1"
	ch := h.pending.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileUploadAck{
			FileUploadAck: &pb.FileUploadAck{RequestId: reqID},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route upload ack: %v", err)
	}

	select {
	case delivered := <-ch:
		if delivered == nil {
			t.Fatal("expected delivered message")
		}
	default:
		t.Fatal("expected message to be delivered to pending channel")
	}
}

func TestRouteAgentMessage_FileDeleteResponse(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()

	stream := &mockStream{}
	reqID := "req-delete-1"
	ch := h.pending.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileDeleteResponse{
			FileDeleteResponse: &pb.FileDeleteResponse{RequestId: reqID},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route file delete response: %v", err)
	}

	select {
	case delivered := <-ch:
		if delivered == nil {
			t.Fatal("expected delivered message")
		}
	default:
		t.Fatal("expected message to be delivered to pending channel")
	}
}

func TestRouteAgentMessage_UnregisterAck(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()

	stream := &mockStream{}
	reqID := "req-unregister-1"
	ch := h.pending.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_UnregisterAck{
			UnregisterAck: &pb.UnregisterAck{RequestId: reqID},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route unregister ack: %v", err)
	}

	select {
	case delivered := <-ch:
		if delivered == nil {
			t.Fatal("expected delivered message")
		}
	default:
		t.Fatal("expected message to be delivered to pending channel")
	}
}

func TestRouteAgentMessage_LogStreamChunk(t *testing.T) {
	h, _ := testHandler(t)
	h.logSessions = NewLogSessions()

	stream := &mockStream{}
	reqID := "req-log-1"
	ch := h.logSessions.Register(reqID)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_LogStreamChunk{
			LogStreamChunk: &pb.LogStreamChunk{RequestId: reqID, Data: []byte("log data")},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route log stream chunk: %v", err)
	}

	select {
	case data := <-ch:
		if string(data) != "log data" {
			t.Fatalf("expected 'log data', got '%s'", string(data))
		}
	default:
		t.Fatal("expected log data to be delivered")
	}
}

func TestRouteAgentMessage_NilPending(t *testing.T) {
	h, _ := testHandler(t)
	// Leave pending as nil — should not panic

	stream := &mockStream{}

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_FileListResponse{
			FileListResponse: &pb.FileListResponse{RequestId: "req-1"},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route with nil pending should not error: %v", err)
	}
}

func TestRouteAgentMessage_UnknownType(t *testing.T) {
	h, _ := testHandler(t)
	stream := &mockStream{}

	// Use a Register message which is not handled in routeAgentMessage
	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_Register{
			Register: &pb.RegisterAgent{Token: "some-token"},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("unhandled message type should not return error: %v", err)
	}
}

func TestHandleStreamHeartbeat_NoConn(t *testing.T) {
	h, st := testHandler(t)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	// Don't register in connMgr — GetConn returns nil
	stream := &mockStream{}
	err := h.handleStreamHeartbeat("s1", stream)
	if err != nil {
		t.Fatalf("handleStreamHeartbeat with no conn: %v", err)
	}
}
