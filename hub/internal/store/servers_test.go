package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestCreateAndGetServer(t *testing.T) {
	s := testStore(t)

	tokenHash := "sha256hashvalue"
	expiresAt := "2026-03-23 13:00:00"
	srv := &model.Server{
		ID:                "srv-1",
		Name:              "web-prod-1",
		Status:            "pending",
		RegistrationToken: &tokenHash,
		TokenExpiresAt:    &expiresAt,
		Labels:            "{}",
	}

	if err := s.CreateServer(srv); err != nil {
		t.Fatalf("create server: %v", err)
	}

	got, err := s.GetServerByID("srv-1")
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.Name != "web-prod-1" {
		t.Fatalf("expected name 'web-prod-1', got '%s'", got.Name)
	}
	if got.Status != "pending" {
		t.Fatalf("expected status 'pending', got '%s'", got.Status)
	}
}

func TestCreateServer_DuplicateID(t *testing.T) {
	s := testStore(t)

	srv := &model.Server{
		ID: "srv-1", Name: "server-a", Status: "pending", Labels: "{}",
	}
	if err := s.CreateServer(srv); err != nil {
		t.Fatalf("first create: %v", err)
	}

	srv2 := &model.Server{
		ID: "srv-1", Name: "server-b", Status: "pending", Labels: "{}",
	}
	if err := s.CreateServer(srv2); err == nil {
		t.Fatal("expected error for duplicate ID")
	}
}

func TestGetServerByID_NotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetServerByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

func TestListServers(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s3", Name: "gamma", Status: "offline", Labels: "{}"})

	// List all
	servers, total, err := s.ListServers(model.ServerFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(servers) != 3 {
		t.Fatalf("expected 3 servers, got %d", len(servers))
	}
}

func TestListServers_FilterByStatus(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})

	status := "online"
	servers, total, err := s.ListServers(model.ServerFilter{Status: &status, Limit: 50})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if servers[0].Name != "alpha" {
		t.Fatalf("expected 'alpha', got '%s'", servers[0].Name)
	}
}

func TestListServers_Search(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "web-prod-1", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "db-staging", Status: "online", Labels: "{}"})

	search := "web"
	servers, total, err := s.ListServers(model.ServerFilter{Search: &search, Limit: 50})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if servers[0].Name != "web-prod-1" {
		t.Fatalf("expected 'web-prod-1', got '%s'", servers[0].Name)
	}
}

func TestListServers_Pagination(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "a", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "b", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s3", Name: "c", Status: "online", Labels: "{}"})

	servers, total, err := s.ListServers(model.ServerFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}
}

func TestListServersForUser(t *testing.T) {
	s := testStore(t)

	// Create a user
	s.CreateUser(&model.User{
		ID: "viewer-1", Username: "viewer", Email: "v@v.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	// Create servers
	s.CreateServer(&model.Server{ID: "s1", Name: "allowed", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "forbidden", Status: "online", Labels: "{}"})

	// Grant permission to s1 only
	_, err := s.DB().Exec(
		"INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)",
		"p1", "viewer-1", "s1", "/",
	)
	if err != nil {
		t.Fatalf("insert permission: %v", err)
	}

	servers, total, err := s.ListServersForUser("viewer-1", model.ServerFilter{Limit: 50})
	if err != nil {
		t.Fatalf("list servers for user: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Name != "allowed" {
		t.Fatalf("expected 'allowed', got '%s'", servers[0].Name)
	}
}

func TestUpdateServer(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "old-name", Status: "online", Labels: "{}"})

	if err := s.UpdateServer("s1", "new-name", `{"env":"prod"}`); err != nil {
		t.Fatalf("update server: %v", err)
	}

	got, _ := s.GetServerByID("s1")
	if got.Name != "new-name" {
		t.Fatalf("expected name 'new-name', got '%s'", got.Name)
	}
	if got.Labels != `{"env":"prod"}` {
		t.Fatalf("expected labels updated, got '%s'", got.Labels)
	}
}

func TestDeleteServer(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "doomed", Status: "online", Labels: "{}"})

	if err := s.DeleteServer("s1"); err != nil {
		t.Fatalf("delete server: %v", err)
	}

	_, err := s.GetServerByID("s1")
	if err == nil {
		t.Fatal("expected error after deletion")
	}
}

func TestDeleteServer_NotFound(t *testing.T) {
	s := testStore(t)

	err := s.DeleteServer("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

func TestDeleteServers_Batch(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "a", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "b", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s3", Name: "c", Status: "online", Labels: "{}"})

	if err := s.DeleteServers([]string{"s1", "s3"}); err != nil {
		t.Fatalf("batch delete: %v", err)
	}

	servers, total, _ := s.ListServers(model.ServerFilter{Limit: 50})
	if total != 1 {
		t.Fatalf("expected 1 remaining, got %d", total)
	}
	if servers[0].ID != "s2" {
		t.Fatalf("expected s2 to remain, got %s", servers[0].ID)
	}
}

func TestGetServerByToken(t *testing.T) {
	s := testStore(t)

	tokenHash := "abc123hash"
	expiresAt := "2099-12-31 23:59:59"
	s.CreateServer(&model.Server{
		ID: "s1", Name: "tokentest", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	got, err := s.GetServerByToken("abc123hash")
	if err != nil {
		t.Fatalf("get by token: %v", err)
	}
	if got.ID != "s1" {
		t.Fatalf("expected 's1', got '%s'", got.ID)
	}
}

func TestGetServerByToken_NotFound(t *testing.T) {
	s := testStore(t)

	_, err := s.GetServerByToken("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown token")
	}
}

func TestActivateServer(t *testing.T) {
	s := testStore(t)

	tokenHash := "abc123hash"
	expiresAt := "2099-12-31 23:59:59"
	s.CreateServer(&model.Server{
		ID: "s1", Name: "activate-me", Status: "pending", Labels: "{}",
		RegistrationToken: &tokenHash, TokenExpiresAt: &expiresAt,
	})

	err := s.ActivateServer("s1", "web-prod-1", "10.10.1.50", "Ubuntu 24.04", "0.1.0")
	if err != nil {
		t.Fatalf("activate server: %v", err)
	}

	got, _ := s.GetServerByID("s1")
	if got.Status != "online" {
		t.Fatalf("expected status 'online', got '%s'", got.Status)
	}
	if got.Hostname == nil || *got.Hostname != "web-prod-1" {
		t.Fatal("expected hostname 'web-prod-1'")
	}
	if got.RegistrationToken != nil {
		t.Fatal("expected registration_token to be cleared")
	}
	if got.TokenExpiresAt != nil {
		t.Fatal("expected token_expires_at to be cleared")
	}
	if got.AgentVersion == nil || *got.AgentVersion != "0.1.0" {
		t.Fatal("expected agent_version '0.1.0'")
	}
}
