package grpcserver

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/connmgr"
	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testHandler(t *testing.T) (*Handler, *store.Store) {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	cm := connmgr.New()
	h := &Handler{
		store:   st,
		connMgr: cm,
	}
	return h, st
}

func TestHandleRegister_ValidToken(t *testing.T) {
	h, st := testHandler(t)
	tokenHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	expiresAt := "2099-12-31 23:59:59"
	st.CreateServer(&model.Server{
		ID: "s1", Name: "test", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})
	serverID, err := h.handleRegister("", "host1", "10.0.0.1", "Linux", "0.1.0")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if serverID != "s1" {
		t.Fatalf("expected 's1', got '%s'", serverID)
	}
	srv, _ := st.GetServerByID("s1")
	if srv.Status != "online" {
		t.Fatalf("expected 'online', got '%s'", srv.Status)
	}
}

func TestHandleRegister_InvalidToken(t *testing.T) {
	h, _ := testHandler(t)
	_, err := h.handleRegister("totally-fake", "host1", "10.0.0.1", "Linux", "0.1.0")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestHandleHeartbeat_ValidServer(t *testing.T) {
	h, st := testHandler(t)
	st.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "offline", Labels: "{}"})
	err := h.handleHeartbeat("s1")
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	srv, _ := st.GetServerByID("s1")
	if srv.Status != "online" {
		t.Fatalf("expected 'online', got '%s'", srv.Status)
	}
}

func TestHandleHeartbeat_UnknownServer(t *testing.T) {
	h, _ := testHandler(t)
	err := h.handleHeartbeat("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown server")
	}
}
