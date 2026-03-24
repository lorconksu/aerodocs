package grpcserver

import (
	"testing"
	"time"
)

func TestLogSessions_RegisterAndDeliver(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("req-1")
	defer ls.Remove("req-1")

	delivered := ls.Deliver("req-1", []byte("hello"))
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
	delivered := ls.Deliver("nonexistent", []byte("hello"))
	if delivered {
		t.Fatal("expected delivery to fail")
	}
}

func TestLogSessions_Remove(t *testing.T) {
	ls := NewLogSessions()
	ls.Register("req-1")
	ls.Remove("req-1")
	delivered := ls.Deliver("req-1", []byte("hello"))
	if delivered {
		t.Fatal("expected delivery to fail after remove")
	}
}

func TestLogSessions_MultipleDeliveries(t *testing.T) {
	ls := NewLogSessions()
	ch := ls.Register("req-1")
	defer ls.Remove("req-1")

	ls.Deliver("req-1", []byte("chunk1"))
	ls.Deliver("req-1", []byte("chunk2"))

	d1 := <-ch
	d2 := <-ch
	if string(d1) != "chunk1" || string(d2) != "chunk2" {
		t.Fatalf("expected chunk1/chunk2, got %s/%s", d1, d2)
	}
}
