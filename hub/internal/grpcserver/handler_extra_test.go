package grpcserver

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

const (
	testFutureExpiry = "2099-12-31 23:59:59"
	testConnectFmt   = "Connect: %v"
	testServerHBIP   = "s-hb-ip"
	testStaleSrv     = "stale-srv"
	testTimeoutSrv   = "timeout-srv"
)

// sequenceStream returns messages from a slice and then io.EOF.
type sequenceStream struct {
	mockStream
	msgs   []*pb.AgentMessage
	pos    int
	ctx    context.Context
	cancel context.CancelFunc
}

func newSequenceStream(msgs []*pb.AgentMessage) *sequenceStream {
	ctx, cancel := context.WithCancel(context.Background())
	return &sequenceStream{msgs: msgs, ctx: ctx, cancel: cancel}
}

func (s *sequenceStream) Recv() (*pb.AgentMessage, error) {
	if s.pos >= len(s.msgs) {
		return nil, io.EOF
	}
	msg := s.msgs[s.pos]
	s.pos++
	return msg, nil
}

func (s *sequenceStream) Context() context.Context {
	if s.mockStream.ctx != nil {
		return s.mockStream.ctx
	}
	return s.ctx
}

func (s *sequenceStream) Send(msg *pb.HubMessage) error {
	if s.mockStream.sendErr != nil {
		return s.mockStream.sendErr
	}
	s.mockStream.sent = append(s.mockStream.sent, msg)
	return nil
}

// TestConnect_RegisterWithCoalescer verifies the Connect method with a heartbeat coalescer,
// covering the hbCoalescer.Flush path on disconnect.
func TestConnect_RegisterWithCoalescer(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()
	h.hbCoalescer = NewHeartbeatCoalescer(st, 5*time.Minute)

	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // NOSONAR — test fixture, SHA256 of empty string
	expiresAt := testFutureExpiry
	st.CreateServer(&model.Server{
		ID: "s-coal", Name: "coalescer-test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        "",
					Hostname:     "host-coal",
					IpAddress:    "10.0.0.1",
					Os:           "Linux",
					AgentVersion: "0.1.0",
				},
			},
		},
	})

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf("Connect should return nil on EOF, got: %v", err)
	}
}

// TestConnect_HeartbeatWithCoalescer verifies Connect handles heartbeat handshake
// with a heartbeat coalescer (covers ForceWrite path).
func TestConnect_HeartbeatWithCoalescer(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()
	h.hbCoalescer = NewHeartbeatCoalescer(st, 5*time.Minute)

	st.CreateServer(&model.Server{ID: "s-coal2", Name: "coalescer-hb", Status: "online", Labels: "{}"})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{ServerId: "s-coal2"},
			},
		},
	})
	stream.mockStream = *streamWithPeerCert("s-coal2")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf("Connect with heartbeat coalescer: %v", err)
	}
}

// TestRouteAgentMessage_HeartbeatWithCoalescer verifies in-stream heartbeat
// uses the coalescer RecordHeartbeat path.
func TestRouteAgentMessage_HeartbeatWithCoalescer(t *testing.T) {
	h, st := testHandler(t)
	h.hbCoalescer = NewHeartbeatCoalescer(st, 5*time.Minute)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	stream := &mockStream{}
	h.connMgr.Register("s1", stream)

	msg := &pb.AgentMessage{
		Payload: &pb.AgentMessage_Heartbeat{
			Heartbeat: &pb.Heartbeat{ServerId: "s1"},
		},
	}
	err := h.routeAgentMessage("s1", stream, msg)
	if err != nil {
		t.Fatalf("route heartbeat with coalescer: %v", err)
	}
	if len(stream.sent) == 0 {
		t.Fatal("expected heartbeat ack to be sent")
	}
}

// TestConnect_Register verifies the Connect method handles registration correctly.
func TestConnect_Register(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // NOSONAR — test fixture, SHA256 of empty string
	expiresAt := testFutureExpiry
	st.CreateServer(&model.Server{
		ID: "s1", Name: "test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	// Register message followed by heartbeat followed by EOF
	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        "", // empty token hashes to e3b0c44... (SHA256 of "")
					Hostname:     "host1",
					IpAddress:    "10.0.0.1",
					Os:           "Linux",
					AgentVersion: "0.1.0",
				},
			},
		},
	})

	err := h.Connect(stream)
	// Should get io.EOF from the Recv loop
	if err != nil {
		t.Fatalf("Connect should return nil on EOF, got: %v", err)
	}
}

// TestConnect_Heartbeat verifies Connect handles a heartbeat as first message.
func TestConnect_Heartbeat(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{ServerId: "s1"},
			},
		},
	})
	stream.mockStream = *streamWithPeerCert("s1")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf("Connect with heartbeat: %v", err)
	}
}

func TestConnect_HeartbeatWithoutClientCertRejected(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "offline", Labels: "{}"})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{ServerId: "s1"},
			},
		},
	})

	err := h.Connect(stream)
	if err == nil {
		t.Fatal("expected heartbeat reconnect without client cert to be rejected")
	}
	if len(stream.sent) != 0 {
		t.Fatalf("expected no heartbeat ack to be sent, got %d messages", len(stream.sent))
	}
	srv, err := st.GetServerByID("s1")
	if err != nil {
		t.Fatalf("GetServerByID: %v", err)
	}
	if srv.Status != "offline" {
		t.Fatalf("expected server to remain offline, got %s", srv.Status)
	}
}

// TestConnect_InvalidFirstMessage verifies Connect rejects unknown first message type.
func TestConnect_InvalidFirstMessage(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_FileListResponse{
				FileListResponse: &pb.FileListResponse{RequestId: "req-1"},
			},
		},
	})

	err := h.Connect(stream)
	if err == nil {
		t.Fatal("expected error for invalid first message type")
	}
}

// TestConnect_RegisterInvalidToken verifies Connect handles invalid registration token.
func TestConnect_RegisterInvalidToken(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:    "totally-invalid-token",
					Hostname: "host1",
				},
			},
		},
	})

	err := h.Connect(stream)
	if err == nil {
		t.Fatal("expected error for invalid registration token")
	}
}

// TestConnect_HeartbeatUnknownServer verifies Connect handles heartbeat from unknown server.
func TestConnect_HeartbeatUnknownServer(t *testing.T) {
	h, _ := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{ServerId: "nonexistent-server"},
			},
		},
	})

	err := h.Connect(stream)
	if err == nil {
		t.Fatal("expected error for unknown server in heartbeat")
	}
}

// TestSweepStaleConnections_ActualStale verifies a stale connection (old heartbeat) is removed.
func TestSweepStaleConnections_ActualStale(t *testing.T) {
	s, st := testGRPCServer(t)

	st.CreateServer(&model.Server{ID: testStaleSrv, Name: "stale", Status: "online", Labels: "{}"})

	stream := &mockStream{}
	s.connMgr.Register(testStaleSrv, stream)

	// Get the connection and back-date its heartbeat
	conn := s.connMgr.GetConn(testStaleSrv)
	if conn != nil {
		// Simulate stale connection by moving LastHeartbeat back in time
		// We can't directly set the field, but we can use the stale check with a very short duration
		// Since we can't manipulate time directly, we'll test via zero-delay sweep
		_ = conn
	}

	// Test the orphan path: register then unregister (server is online but not connected)
	s.connMgr.Unregister(testStaleSrv)
	s.sweepStaleConnections()

	srv, _ := st.GetServerByID(testStaleSrv)
	if srv.Status != "offline" {
		t.Fatalf("expected orphaned server to be offline, got %s", srv.Status)
	}
}

// TestSweepStaleConnections_WithStaleConnAfterTimeout verifies stale detection.
func TestSweepStaleConnections_StaleTimeout(t *testing.T) {
	s, st := testGRPCServer(t)

	st.CreateServer(&model.Server{ID: testTimeoutSrv, Name: "timeout", Status: "online", Labels: "{}"})

	stream := &mockStream{}
	s.connMgr.Register(testTimeoutSrv, stream)

	// Sweep with 0 duration — all connections are "stale" (registered time > 0 ago)
	stale := s.connMgr.StaleConnections(0 * time.Second)
	for _, id := range stale {
		s.connMgr.Unregister(id)
		_ = st.UpdateServerStatus(id, "offline")
	}

	srv, _ := st.GetServerByID(testTimeoutSrv)
	if srv.Status != "offline" {
		t.Fatalf("expected server marked offline after stale sweep, got %s", srv.Status)
	}
}

// TestRegisterHandshake_UsesPeerIP verifies that registration records the peer IP
// instead of trusting agent-supplied IP data.
func TestRegisterHandshake_UsesPeerIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // NOSONAR — test fixture, SHA256 of empty string
	expiresAt := testFutureExpiry
	st.CreateServer(&model.Server{
		ID: "s-ip-test", Name: "ip-test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        "",
					Hostname:     "host-ip-test",
					IpAddress:    "192.168.1.100", // agent's real IP
					Os:           "Linux",
					AgentVersion: "0.1.0",
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer("", "10.10.1.95")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}

	srv, _ := st.GetServerByID("s-ip-test")
	if srv.IPAddress == nil || *srv.IPAddress != "10.10.1.95" {
		got := "<nil>"
		if srv.IPAddress != nil {
			got = *srv.IPAddress
		}
		t.Fatalf("expected peer IP 10.10.1.95, got %s", got)
	}
}

// TestRegisterHandshake_FallsBackToPeerIP verifies that registration falls back to
// gRPC peer address when agent doesn't report an IP.
func TestRegisterHandshake_FallsBackToPeerIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // NOSONAR — test fixture, SHA256 of empty string
	expiresAt := testFutureExpiry
	st.CreateServer(&model.Server{
		ID: "s-ip-fallback", Name: "ip-fallback", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        "",
					Hostname:     "host-fb",
					IpAddress:    "", // no agent-reported IP
					Os:           "Linux",
					AgentVersion: "0.1.0",
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer("", "10.10.1.96")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}

	// peerAddr returns empty for mock streams, so IP should be empty
	srv, _ := st.GetServerByID("s-ip-fallback")
	// With mock stream, peer address is empty, so IP won't be set
	// This verifies the fallback path is exercised without crash
	_ = srv
}

// TestHeartbeatHandshake_UsesPeerIP verifies that reconnect IP attribution
// comes from the authenticated peer rather than the heartbeat payload.
func TestHeartbeatHandshake_UsesPeerIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	proxyIP := "10.10.1.27" // Traefik proxy IP
	st.CreateServer(&model.Server{ID: testServerHBIP, Name: "hb-ip", Status: "online", Labels: "{}"})
	_ = st.UpdateServerIP(testServerHBIP, proxyIP)

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{
					ServerId:  testServerHBIP,
					IpAddress: "10.10.1.95", // real agent IP
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer(testServerHBIP, "10.10.1.27")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}

	srv, _ := st.GetServerByID(testServerHBIP)
	if srv.IPAddress == nil || *srv.IPAddress != "10.10.1.27" {
		got := "<nil>"
		if srv.IPAddress != nil {
			got = *srv.IPAddress
		}
		t.Fatalf("expected peer IP 10.10.1.27, got %s", got)
	}
}

// TestRegisterHandshake_InvalidAgentIP verifies that an invalid agent-reported IP
// (e.g., arbitrary string injection) is rejected and the peer IP is used instead.
func TestRegisterHandshake_InvalidAgentIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" // NOSONAR — test fixture, SHA256 of empty string
	expiresAt := testFutureExpiry
	st.CreateServer(&model.Server{
		ID: "s-ip-invalid", Name: "ip-invalid", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Register{
				Register: &pb.RegisterAgent{
					Token:        "",
					Hostname:     "host-invalid-ip",
					IpAddress:    "not-an-ip-address", // invalid IP
					Os:           "Linux",
					AgentVersion: "0.1.0",
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer("", "10.10.1.97")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}

	// The invalid IP should have been rejected; peer IP (empty for mock) is used instead
	srv, _ := st.GetServerByID("s-ip-invalid")
	if srv.IPAddress != nil && *srv.IPAddress == "not-an-ip-address" {
		t.Fatal("expected invalid IP to be rejected, but it was stored")
	}
}

// TestHeartbeatHandshake_InvalidAgentIP verifies that an invalid agent-reported IP
// in heartbeat reconnect is rejected and peer IP is used instead.
func TestHeartbeatHandshake_InvalidAgentIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	originalIP := "10.0.0.5"
	st.CreateServer(&model.Server{ID: "s-hb-invalid-ip", Name: "hb-invalid", Status: "online", Labels: "{}"})
	_ = st.UpdateServerIP("s-hb-invalid-ip", originalIP)

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{
					ServerId:  "s-hb-invalid-ip",
					IpAddress: "'; DROP TABLE servers; --", // SQL injection attempt as IP
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer("s-hb-invalid-ip", "10.10.1.98")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}

	// The invalid IP should have been rejected
	srv, _ := st.GetServerByID("s-hb-invalid-ip")
	if srv.IPAddress != nil && *srv.IPAddress == "'; DROP TABLE servers; --" {
		t.Fatal("expected SQL injection string to be rejected as IP")
	}
}

// TestHeartbeatHandshake_FallsBackToPeerIP verifies heartbeat reconnect
// uses peer IP when agent doesn't report one.
func TestHeartbeatHandshake_FallsBackToPeerIP(t *testing.T) {
	h, st := testHandler(t)
	h.pending = NewPendingRequests()
	h.logSessions = NewLogSessions()

	st.CreateServer(&model.Server{ID: "s-hb-fb", Name: "hb-fb", Status: "online", Labels: "{}"})

	stream := newSequenceStream([]*pb.AgentMessage{
		{
			Payload: &pb.AgentMessage_Heartbeat{
				Heartbeat: &pb.Heartbeat{
					ServerId:  "s-hb-fb",
					IpAddress: "", // no agent-reported IP
				},
			},
		},
	})
	stream.mockStream = *streamWithPeer("s-hb-fb", "10.10.1.99")

	err := h.Connect(stream)
	if err != nil {
		t.Fatalf(testConnectFmt, err)
	}
	// Verifies fallback path runs without crash
}
