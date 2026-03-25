package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAgentBinaryConnectAndFileList exercises the full agent binary (main → Run →
// connectAndStream → dialHub) as a subprocess, producing cross-module coverage
// via GOCOVERDIR.
func TestAgentBinaryConnectAndFileList(t *testing.T) {
	h := StartHarness(t)
	token := h.SetupAdmin(t)
	serverID, regToken := h.CreateServer(t, token, "binary-test-server")
	t.Logf("created server: id=%s", serverID)

	// Start the coverage-instrumented agent binary as a subprocess
	cancelAgent := h.StartAgentProcess(t, regToken)

	// Poll ConnMgr until agent connects (10s timeout — subprocess startup is slower)
	deadline := time.Now().Add(10 * time.Second)
	connected := false
	for time.Now().Before(deadline) {
		for _, id := range h.ConnMgr.ActiveServerIDs() {
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
		cancelAgent()
		t.Fatal("agent binary did not connect within 10s")
	}
	t.Log("agent binary connected to hub")

	// Verify server status is online
	srv, err := h.Store.GetServerByID(serverID)
	if err != nil {
		cancelAgent()
		t.Fatalf("get server: %v", err)
	}
	if srv.Status != "online" {
		cancelAgent()
		t.Fatalf("expected status 'online', got %q", srv.Status)
	}

	// Create a temp directory with test files for the file list round-trip
	tmpDir := t.TempDir()
	testFiles := []string{"readme.txt", "config.yaml", "data.json"}
	for _, name := range testFiles {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte("test content"), 0o644); err != nil {
			cancelAgent()
			t.Fatalf("write test file %s: %v", name, err)
		}
	}

	// Request file list via HTTP → Hub → gRPC → Agent binary → response
	path := fmt.Sprintf("/api/servers/%s/files?path=%s", serverID, tmpDir)
	resp := h.HTTPGet(t, path, token)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		cancelAgent()
		t.Fatalf("file list: status=%d body=%s", resp.StatusCode, body)
	}

	var result struct {
		Files []struct {
			Name string `json:"name"`
		} `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cancelAgent()
		t.Fatalf("decode file list: %v", err)
	}

	// Verify all test files are present
	nameSet := make(map[string]bool)
	for _, f := range result.Files {
		nameSet[f.Name] = true
	}
	for _, expected := range testFiles {
		if !nameSet[expected] {
			cancelAgent()
			t.Fatalf("expected %q in file list, got %+v", expected, result.Files)
		}
	}
	t.Logf("file list round-trip verified: found all %d test files", len(testFiles))

	// Graceful shutdown — agent writes coverage data on SIGINT
	cancelAgent()

	// Give the gRPC handler a moment to process the disconnect
	time.Sleep(500 * time.Millisecond)

	// Verify agent disconnected from ConnMgr
	for _, id := range h.ConnMgr.ActiveServerIDs() {
		if id == serverID {
			t.Fatal("agent still in ConnMgr after shutdown")
		}
	}
	t.Log("agent binary disconnected successfully")

	// Verify server status is offline
	srv2, err := h.Store.GetServerByID(serverID)
	if err != nil {
		t.Fatalf("get server after disconnect: %v", err)
	}
	if srv2.Status != "offline" {
		t.Fatalf("expected status 'offline' after disconnect, got %q", srv2.Status)
	}
	t.Log("server status is offline after agent binary shutdown")
}
