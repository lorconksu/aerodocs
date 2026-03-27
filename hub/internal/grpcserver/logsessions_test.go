package grpcserver

import (
	"testing"
	"time"
)

func TestLogSessions_RegisterAndDeliver(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("srv-1", "req-1")
	defer ls.Remove("srv-1", "req-1")

	delivered := ls.Deliver("srv-1", "req-1", []byte("hello"))
	if !delivered {
		t.Fatal("expected delivery to succeed")
	}

	select {
	case data := <-ch:
		if string(data) != "hello" {
			t.Fatalf("expected 'hello', got '%s'", string(data))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}

func TestLogSessions_DeliverNotFound(t *testing.T) {
	ls := NewLogSessions()
	delivered := ls.Deliver("srv-1", "nonexistent", []byte("hello"))
	if delivered {
		t.Fatal("expected delivery to fail")
	}
}

func TestLogSessions_Remove(t *testing.T) {
	ls := NewLogSessions()
	ls.Register("srv-1", "req-1")
	ls.Remove("srv-1", "req-1")
	delivered := ls.Deliver("srv-1", "req-1", []byte("hello"))
	if delivered {
		t.Fatal("expected delivery to fail after remove")
	}
}

func TestLogSessions_MultipleDeliveries(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("srv-1", "req-1")
	defer ls.Remove("srv-1", "req-1")

	ls.Deliver("srv-1", "req-1", []byte("chunk1"))
	ls.Deliver("srv-1", "req-1", []byte("chunk2"))

	d1 := <-ch
	d2 := <-ch
	if string(d1) != "chunk1" || string(d2) != "chunk2" {
		t.Fatalf("expected chunk1/chunk2, got %s/%s", d1, d2)
	}
}

func TestLogSessions_OverflowTracking(t *testing.T) {
	ls := NewLogSessions()
	_ = ls.Register("srv-1", "req-1")
	defer ls.Remove("srv-1", "req-1")

	// Fill the buffer (capacity 64)
	for i := 0; i < 64; i++ {
		ok := ls.Deliver("srv-1", "req-1", []byte("x"))
		if !ok {
			t.Fatalf("delivery %d should succeed", i)
		}
	}

	// No overflow yet
	if n := ls.DrainOverflow("srv-1", "req-1"); n != 0 {
		t.Fatalf("expected 0 overflow, got %d", n)
	}

	// Next deliveries should be dropped and counted
	for i := 0; i < 10; i++ {
		ok := ls.Deliver("srv-1", "req-1", []byte("dropped"))
		if ok {
			t.Fatalf("delivery %d should fail (buffer full)", i)
		}
	}

	n := ls.DrainOverflow("srv-1", "req-1")
	if n != 10 {
		t.Fatalf("expected 10 overflow, got %d", n)
	}

	// DrainOverflow resets the counter
	n = ls.DrainOverflow("srv-1", "req-1")
	if n != 0 {
		t.Fatalf("expected 0 after drain, got %d", n)
	}
}

func TestLogSessions_DrainOverflowUnknownSession(t *testing.T) {
	ls := NewLogSessions()
	// DrainOverflow on a non-existent session returns 0 (no panic)
	n := ls.DrainOverflow("srv-1", "nonexistent")
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestLogSessions_OverflowClearedOnRemove(t *testing.T) {
	ls := NewLogSessions()
	_ = ls.Register("srv-1", "req-1")

	// Fill buffer and cause overflow
	for i := 0; i < 70; i++ {
		ls.Deliver("srv-1", "req-1", []byte("x"))
	}

	ls.Remove("srv-1", "req-1")

	// After removal, drain returns 0
	n := ls.DrainOverflow("srv-1", "req-1")
	if n != 0 {
		t.Fatalf("expected 0 after remove, got %d", n)
	}
}

func TestLogSessions_NormalDeliveryNoOverflow(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("srv-1", "req-1")
	defer ls.Remove("srv-1", "req-1")

	// Deliver and consume — no overflow should occur
	for i := 0; i < 100; i++ {
		ok := ls.Deliver("srv-1", "req-1", []byte("data"))
		if !ok {
			t.Fatalf("delivery %d should succeed", i)
		}
		<-ch
	}

	if n := ls.DrainOverflow("srv-1", "req-1"); n != 0 {
		t.Fatalf("expected 0 overflow, got %d", n)
	}
}

func TestLogSessions_CrossServerIsolation(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("srv-1", "req-1")
	defer ls.Remove("srv-1", "req-1")

	// A different server trying the same requestID should fail
	delivered := ls.Deliver("srv-2", "req-1", []byte("injected"))
	if delivered {
		t.Fatal("expected cross-server delivery to fail")
	}

	// The legitimate server should succeed
	delivered = ls.Deliver("srv-1", "req-1", []byte("legit"))
	if !delivered {
		t.Fatal("expected same-server delivery to succeed")
	}

	select {
	case data := <-ch:
		if string(data) != "legit" {
			t.Fatalf("expected 'legit', got '%s'", string(data))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout")
	}
}
