package integration

import (
	"context"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/agent/agentclient"
)

func TestAgentConnectAndHeartbeat(t *testing.T) {
	h := StartHarness(t)

	// Setup admin and get access token
	token := h.SetupAdmin(t)

	// Create a server entry via HTTP API
	serverID, regToken := h.CreateServer(t, token, "test-server")
	t.Logf("created server: id=%s", serverID)

	// Create agent client — use 127.0.0.1 (IP) so the agent skips TLS
	agentClient := agentclient.New(agentclient.Config{
		HubAddr:      h.GRPCAddr, // 127.0.0.1:<port>
		ServerID:     "",         // empty for first registration
		Token:        regToken,
		Hostname:     "test-host",
		IPAddress:    "10.0.0.1",
		OS:           "linux",
		AgentVersion: "0.0.0-test",
	})

	// Run agent in background with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agentErrCh := make(chan error, 1)
	go func() {
		agentErrCh <- agentClient.Run(ctx)
	}()

	// Poll ConnMgr until agent appears (5s timeout)
	deadline := time.Now().Add(5 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		ids := h.ConnMgr.ActiveServerIDs()
		for _, id := range ids {
			if id == serverID {
				connected = true
				break
			}
		}
		if connected {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !connected {
		t.Fatal("agent did not appear in ConnMgr within 5s")
	}
	t.Log("agent connected to hub")

	// Verify server status is "online" in store
	srv, err := h.Store.GetServerByID(serverID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if srv.Status != "online" {
		t.Fatalf("expected status 'online', got %q", srv.Status)
	}
	t.Log("server status is online")

	// Record initial last_seen_at
	initialLastSeen := srv.LastSeenAt

	// Wait 12s for heartbeat to update last_seen_at
	// Agent heartbeat interval is 10s, so we wait a bit longer.
	t.Log("waiting 12s for heartbeat...")
	time.Sleep(12 * time.Second)

	// Verify heartbeat updated last_seen_at
	srv2, err := h.Store.GetServerByID(serverID)
	if err != nil {
		t.Fatalf("get server after heartbeat: %v", err)
	}
	if srv2.LastSeenAt == nil {
		t.Fatal("last_seen_at is nil after heartbeat")
	}
	if initialLastSeen != nil && *srv2.LastSeenAt == *initialLastSeen {
		t.Fatal("last_seen_at did not change after heartbeat")
	}
	t.Logf("heartbeat verified: last_seen_at=%s", *srv2.LastSeenAt)

	// Verify agent got its server ID
	if agentClient.ServerID() != serverID {
		t.Fatalf("agent server ID mismatch: got %q, want %q", agentClient.ServerID(), serverID)
	}

	// Cancel context to disconnect agent
	cancel()

	// Wait briefly for disconnect
	select {
	case <-agentErrCh:
		// Agent returned (expected: context canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("agent did not exit within 3s after context cancel")
	}

	// Give the gRPC handler a moment to process the disconnect
	time.Sleep(500 * time.Millisecond)

	// Verify agent disconnected from ConnMgr
	ids := h.ConnMgr.ActiveServerIDs()
	for _, id := range ids {
		if id == serverID {
			t.Fatal("agent still in ConnMgr after disconnect")
		}
	}
	t.Log("agent disconnected successfully")

	// Verify server status is "offline" in store
	srv3, err := h.Store.GetServerByID(serverID)
	if err != nil {
		t.Fatalf("get server after disconnect: %v", err)
	}
	if srv3.Status != "offline" {
		t.Fatalf("expected status 'offline' after disconnect, got %q", srv3.Status)
	}
	t.Log("server status is offline after disconnect")
}
