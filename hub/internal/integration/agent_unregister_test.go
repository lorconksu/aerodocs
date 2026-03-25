package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/agent/agentclient"
)

func TestServerUnregister(t *testing.T) {
	h := StartHarness(t)
	token := h.SetupAdmin(t)
	serverID, regToken := h.CreateServer(t, token, "unregister-server")

	// Start agent
	agentClient := agentclient.New(agentclient.Config{
		HubAddr:      h.GRPCAddr,
		ServerID:     "",
		Token:        regToken,
		Hostname:     "test-host",
		IPAddress:    "10.0.0.1",
		OS:           "linux",
		AgentVersion: "0.0.0-test",
	})

	ctx, cancel := context.WithCancel(context.Background())

	agentErrCh := make(chan error, 1)
	go func() {
		agentErrCh <- agentClient.Run(ctx)
	}()

	// Ensure cancel is called at test end regardless
	defer cancel()

	// Wait for agent to connect
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, id := range h.ConnMgr.ActiveServerIDs() {
			if id == serverID {
				goto connected
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("agent did not connect within 5s")
connected:
	t.Log("agent connected")

	// Send DELETE /api/servers/{id}/unregister
	unregURL := fmt.Sprintf("http://%s/api/servers/%s/unregister", h.HTTPAddr, serverID)
	req, err := http.NewRequest("DELETE", unregURL, nil)
	if err != nil {
		t.Fatalf("create unregister request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("unregister request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("unregister: status=%d body=%s", resp.StatusCode, body)
	}
	t.Log("unregister response: 200 OK")

	// Cancel the agent context immediately to prevent the agent's selfCleanup()
	// from calling syscall.Exec (which would replace the test process).
	// The agent has a 2-second delay before cleanup — cancelling now prevents it.
	cancel()

	// Wait for agent goroutine to exit
	select {
	case <-agentErrCh:
		t.Log("agent goroutine exited")
	case <-time.After(5 * time.Second):
		t.Fatal("agent goroutine did not exit within 5s")
	}

	// Wait for agent to disconnect from ConnMgr
	disconnectDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(disconnectDeadline) {
		ids := h.ConnMgr.ActiveServerIDs()
		found := false
		for _, id := range ids {
			if id == serverID {
				found = true
				break
			}
		}
		if !found {
			goto disconnected
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("agent did not disconnect from ConnMgr within 5s after unregister")
disconnected:
	t.Log("agent disconnected from ConnMgr")

	// Verify server no longer exists in store
	_, err = h.Store.GetServerByID(serverID)
	if err == nil {
		t.Fatal("server still exists in store after unregister")
	}
	t.Logf("server %s verified deleted from store", serverID)
}
