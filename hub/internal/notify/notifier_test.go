package notify

import (
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testStoreAndUser(t *testing.T) *store.Store {
	t.Helper()
	// Use a temporary file-based database to avoid WAL + in-memory SQLite
	// connection-pool reuse issues with the modernc driver under -count > 1.
	f, err := os.CreateTemp("", "notifytest-*.db")
	if err != nil {
		t.Fatalf("create temp db file: %v", err)
	}
	dbPath := f.Name()
	f.Close()
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() {
		st.Close()
		os.Remove(dbPath)
	})
	st.CreateUser(&model.User{
		ID: "user1", Username: "admin", Email: "admin@test.com",
		PasswordHash: "$2a$12$dummy", Role: model.RoleAdmin,
		TOTPEnabled: true,
	})
	return st
}

// TestNotifier_NoopWhenDisabled verifies that when SMTP is not configured,
// Notify returns without panicking and no notification log entries are created.
func TestNotifier_NoopWhenDisabled(t *testing.T) {
	st := testStoreAndUser(t)
	n := New(st)
	defer n.Close()

	// SMTP is not configured in the store, so Notify should be a no-op
	n.Notify(model.NotifyAgentOffline, map[string]string{
		"server_name": "test-agent",
	})

	// Allow the worker a moment to process anything that might have been enqueued
	time.Sleep(20 * time.Millisecond)

	// Verify no notification log entries were created
	entries, total, err := st.ListNotificationLog(100, 0)
	if err != nil {
		t.Fatalf("list notification log: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 log entries, got %d: %v", total, entries)
	}
}

// TestNotifier_SendsWhenConfigured verifies that when SMTP is fully configured and enabled,
// Notify enqueues a job, the worker sends the email via the mock SMTP server,
// and a "sent" entry appears in the notification log.
func TestNotifier_SendsWhenConfigured(t *testing.T) {
	st := testStoreAndUser(t)

	// Start a mock SMTP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start mock SMTP server: %v", err)
	}
	defer ln.Close()

	received := make(chan string, 1)
	go mockSMTPServer(t, ln, received)

	addr := ln.Addr().(*net.TCPAddr)

	// Configure SMTP in the store
	smtpCfgs := []struct{ k, v string }{
		{"smtp_host", "127.0.0.1"},
		{"smtp_port", strconv.Itoa(addr.Port)},
		{"smtp_from", "noreply@aerodocs.local"},
		{"smtp_enabled", "true"},
	}
	for _, c := range smtpCfgs {
		if err := st.SetConfig(c.k, c.v); err != nil {
			t.Fatalf("SetConfig %s: %v", c.k, err)
		}
	}

	// NotifyAgentOffline is default-on, so all users receive it without explicit preference.
	n := New(st)
	defer n.Close()

	n.Notify(model.NotifyAgentOffline, map[string]string{
		"server_name": "test-agent",
		"server_id":   "srv-1",
		"timestamp":   time.Now().UTC().Format(model.NotifyTimestampFormat),
	})

	// Wait for the mock SMTP server to receive the message
	select {
	case data := <-received:
		t.Logf("mock SMTP received: %q", data)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for mock SMTP server to receive email")
	}

	// Allow the worker goroutine to finish logging
	time.Sleep(50 * time.Millisecond)

	entries, total, err := st.ListNotificationLog(100, 0)
	if err != nil {
		t.Fatalf("ListNotificationLog: %v", err)
	}
	if total == 0 {
		t.Fatal("expected at least one notification log entry, got 0")
	}

	found := false
	for _, e := range entries {
		if e.Status == "sent" && e.EventType == model.NotifyAgentOffline {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'sent' log entry for %s, got: %+v", model.NotifyAgentOffline, entries)
	}
}

// TestNotifier_SendFailure verifies that when SMTP send fails, a "failed" log entry is created.
func TestNotifier_SendFailure(t *testing.T) {
	st := testStoreAndUser(t)

	// Configure SMTP with an unreachable port to force send failure
	smtpCfgs := []struct{ k, v string }{
		{"smtp_host", "127.0.0.1"},
		{"smtp_port", "1"}, // port 1 is unreachable
		{"smtp_from", "test@test.com"},
		{"smtp_enabled", "true"},
		{"smtp_tls", "false"},
	}
	for _, c := range smtpCfgs {
		if err := st.SetConfig(c.k, c.v); err != nil {
			t.Fatalf("SetConfig %s: %v", c.k, err)
		}
	}

	n := New(st)
	defer n.Close()

	n.Notify(model.NotifyAgentOffline, map[string]string{
		"server_name": "web-01",
		"server_id":   "srv-1",
		"timestamp":   "2026-03-29 12:00:00 UTC",
	})

	// Wait for worker to process and log the failure
	time.Sleep(500 * time.Millisecond)

	entries, total, err := st.ListNotificationLog(50, 0)
	if err != nil {
		t.Fatalf("ListNotificationLog: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 log entry, got %d", total)
	}
	if entries[0].Status != "failed" {
		t.Fatalf("expected 'failed' status, got %q", entries[0].Status)
	}
}

// TestNotifier_Close verifies that Close() doesn't hang or deadlock.
func TestNotifier_Close(t *testing.T) {
	st := testStoreAndUser(t)
	n := New(st)

	done := make(chan struct{})
	go func() {
		n.Close()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out — possible deadlock")
	}
}
