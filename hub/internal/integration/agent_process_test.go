package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// StartAgentProcess launches the coverage-instrumented agent binary as a subprocess.
// It sets GOCOVERDIR so the binary writes coverage data on exit.
// The returned cancel function sends SIGINT for a graceful shutdown (which triggers
// coverage data flush) and waits for the process to exit.
//
// This function lives in a _test.go file because it references agentBinaryPath and
// agentCovDir which are defined in main_test.go (test-only scope).
func (h *TestHarness) StartAgentProcess(t *testing.T, token string) (cancel func()) {
	t.Helper()

	if agentBinaryPath == "" {
		t.Fatal("agentBinaryPath not set — TestMain must build the agent binary first")
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "agent.conf")

	ctx, ctxCancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, agentBinaryPath,
		"--hub", h.GRPCAddr,
		"--token", token,
		"--config", configPath,
	)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+agentCovDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		ctxCancel()
		t.Fatalf("start agent process: %v", err)
	}
	t.Logf("started agent process: pid=%d", cmd.Process.Pid)

	return func() {
		// Send SIGINT for graceful shutdown so coverage data is written
		if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
			t.Logf("warning: sending SIGINT to agent: %v", err)
		}
		// Wait for the process to exit (with a timeout via context)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
			// Process exited
		case <-time.After(10 * time.Second):
			t.Log("warning: agent did not exit within 10s after SIGINT, killing")
			cmd.Process.Kill()
		}
		ctxCancel()
	}
}
