package client

import (
	"testing"
	"time"
)

func TestNextBackoff(t *testing.T) {
	c := &Client{
		backoff:    1 * time.Second,
		maxBackoff: 60 * time.Second,
	}
	b1 := c.nextBackoff()
	if b1 != 1*time.Second {
		t.Fatalf("expected 1s, got %v", b1)
	}
	b2 := c.nextBackoff()
	if b2 != 2*time.Second {
		t.Fatalf("expected 2s, got %v", b2)
	}
	b3 := c.nextBackoff()
	if b3 != 4*time.Second {
		t.Fatalf("expected 4s, got %v", b3)
	}
}

func TestNextBackoff_CapsAtMax(t *testing.T) {
	c := &Client{
		backoff:    32 * time.Second,
		maxBackoff: 60 * time.Second,
	}
	b1 := c.nextBackoff()
	if b1 != 32*time.Second {
		t.Fatalf("expected 32s, got %v", b1)
	}
	b2 := c.nextBackoff()
	if b2 != 60*time.Second {
		t.Fatalf("expected 60s (capped), got %v", b2)
	}
}

func TestResetBackoff(t *testing.T) {
	c := &Client{
		backoff:    16 * time.Second,
		maxBackoff: 60 * time.Second,
	}
	c.resetBackoff()
	b := c.nextBackoff()
	if b != 1*time.Second {
		t.Fatalf("expected 1s after reset, got %v", b)
	}
}

func TestNewClient(t *testing.T) {
	c := New(Config{
		HubAddr:  "localhost:9090",
		ServerID: "srv-1",
	})
	if c.hubAddr != "localhost:9090" {
		t.Fatalf("expected hubAddr 'localhost:9090', got '%s'", c.hubAddr)
	}
	if c.serverID != "srv-1" {
		t.Fatalf("expected serverID 'srv-1', got '%s'", c.serverID)
	}
}
