package server

import "testing"

func TestTryAcquireLogTailSlot_ViewerLimit(t *testing.T) {
	s := &Server{logTailSessions: make(map[string]int)}

	for i := 0; i < maxViewerLogStreamsPerUser; i++ {
		if !s.tryAcquireLogTailSlot("srv-1", "viewer-1", "viewer") {
			t.Fatalf("expected viewer slot %d to be granted", i+1)
		}
	}
	if s.tryAcquireLogTailSlot("srv-1", "viewer-1", "viewer") {
		t.Fatal("expected viewer log-tail quota to be enforced")
	}

	for i := 0; i < maxViewerLogStreamsPerUser; i++ {
		s.releaseLogTailSlot("srv-1", "viewer-1")
	}
	if !s.tryAcquireLogTailSlot("srv-1", "viewer-1", "viewer") {
		t.Fatal("expected viewer quota to reset after releases")
	}
}

func TestTryAcquireLogTailSlot_AdminHigherLimit(t *testing.T) {
	s := &Server{logTailSessions: make(map[string]int)}

	for i := 0; i < maxAdminLogStreamsPerUser; i++ {
		if !s.tryAcquireLogTailSlot("srv-1", "admin-1", "admin") {
			t.Fatalf("expected admin slot %d to be granted", i+1)
		}
	}
	if s.tryAcquireLogTailSlot("srv-1", "admin-1", "admin") {
		t.Fatal("expected admin log-tail quota to be enforced")
	}
	if !s.tryAcquireLogTailSlot("srv-1", "viewer-1", "viewer") {
		t.Fatal("expected quotas to be tracked independently per user")
	}
}
