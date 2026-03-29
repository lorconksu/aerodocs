package notify

import (
	"os"
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
