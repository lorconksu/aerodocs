package server

import (
	"fmt"
	"testing"
)

const (
	testViewerID = "viewer-1"
	testAdminID  = "admin-1"
)

func TestTryAcquireLogTailSlot_ViewerLimit(t *testing.T) {
	s := &Server{logTailSessions: make(map[string]int)}

	for i := 0; i < maxViewerLogStreamsPerUser; i++ {
		if !s.tryAcquireLogTailSlot("srv-1", testViewerID, "viewer") {
			t.Fatalf("expected viewer slot %d to be granted", i+1)
		}
	}
	if s.tryAcquireLogTailSlot("srv-1", testViewerID, "viewer") {
		t.Fatal("expected viewer log-tail quota to be enforced")
	}

	for i := 0; i < maxViewerLogStreamsPerUser; i++ {
		s.releaseLogTailSlot("srv-1", testViewerID)
	}
	if !s.tryAcquireLogTailSlot("srv-1", testViewerID, "viewer") {
		t.Fatal("expected viewer quota to reset after releases")
	}
}

func TestTryAcquireLogTailSlot_AdminHigherLimit(t *testing.T) {
	s := &Server{logTailSessions: make(map[string]int)}

	for i := 0; i < maxAdminLogStreamsPerUser; i++ {
		if !s.tryAcquireLogTailSlot("srv-1", testAdminID, "admin") {
			t.Fatalf("expected admin slot %d to be granted", i+1)
		}
	}
	if s.tryAcquireLogTailSlot("srv-1", testAdminID, "admin") {
		t.Fatal("expected admin log-tail quota to be enforced")
	}
	if !s.tryAcquireLogTailSlot("srv-1", testViewerID, "viewer") {
		t.Fatal("expected quotas to be tracked independently per user")
	}
}

func TestTryAcquireLogTailSlot_ViewerReserveLeavesAdminHeadroom(t *testing.T) {
	s := &Server{logTailSessions: make(map[string]int)}

	for i := 0; i < maxAgentLogStreams-maxAdminLogStreamsPerUser; i++ {
		userID := fmt.Sprintf("viewer-%d", i)
		if !s.tryAcquireLogTailSlot("srv-1", userID, "viewer") {
			t.Fatalf("expected viewer slot %d to be granted", i+1)
		}
	}
	if s.tryAcquireLogTailSlot("srv-1", "viewer-extra", "viewer") {
		t.Fatal("expected viewer traffic to be capped before exhausting the agent-wide pool")
	}
	if !s.tryAcquireLogTailSlot("srv-1", testAdminID, "admin") {
		t.Fatal("expected reserved admin headroom to remain available")
	}
}
