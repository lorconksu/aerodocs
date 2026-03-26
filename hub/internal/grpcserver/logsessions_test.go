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
