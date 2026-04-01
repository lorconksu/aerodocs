package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

// TestUpdateServer_NotFound verifies updating a non-existent server returns an error.
func TestUpdateServer_NotFound(t *testing.T) {
	s := testStore(t)

	err := s.UpdateServer("nonexistent", "new-name", "{}")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

// TestDeleteServers_Empty verifies batch delete with empty list is a no-op.
func TestDeleteServers_Empty(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "keep", Status: "online", Labels: "{}"})

	if err := s.DeleteServers([]string{}); err != nil {
		t.Fatalf("delete empty list: %v", err)
	}

	// Server should still exist
	_, err := s.GetServerByID("s1")
	if err != nil {
		t.Fatal("expected server to still exist after empty batch delete")
	}
}

// TestActivateServer_NotFound verifies activating a non-existent server returns an error.
func TestActivateServer_NotFound(t *testing.T) {
	s := testStore(t)

	err := s.ActivateServer("nonexistent", "host", "1.2.3.4", "linux", "0.1.0")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

// TestUpdateServerLastSeen_NotFound verifies updating last_seen for non-existent server.
func TestUpdateServerLastSeen_NotFound(t *testing.T) {
	s := testStore(t)

	err := s.UpdateServerLastSeen("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

// TestListServersForUser_WithStatusFilter verifies the viewer user filter respects status.
func TestListServersForUser_WithStatusFilter(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "viewer-2", Username: "viewer2", Email: "v2@v.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	s.CreateServer(&model.Server{ID: "s1", Name: "online-srv", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "offline-srv", Status: "offline", Labels: "{}"})

	s.DB().Exec("INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)", "p1", "viewer-2", "s1", "/")
	s.DB().Exec("INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)", "p2", "viewer-2", "s2", "/")

	status := "online"
	servers, total, err := s.ListServersForUser("viewer-2", model.ServerFilter{Status: &status, Limit: 50})
	if err != nil {
		t.Fatalf("list servers for user: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 online server for viewer, got %d", total)
	}
	if servers[0].ID != "s1" {
		t.Fatalf("expected s1, got %s", servers[0].ID)
	}
}

// TestListServersForUser_WithSearchFilter verifies search filter for user servers.
func TestListServersForUser_WithSearchFilter(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "viewer-3", Username: "viewer3", Email: "v3@v.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	s.CreateServer(&model.Server{ID: "s1", Name: "web-prod", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "db-prod", Status: "online", Labels: "{}"})

	s.DB().Exec("INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)", "p1", "viewer-3", "s1", "/")
	s.DB().Exec("INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)", "p2", "viewer-3", "s2", "/")

	search := "web"
	servers, total, err := s.ListServersForUser("viewer-3", model.ServerFilter{Search: &search, Limit: 50})
	if err != nil {
		t.Fatalf("list servers for user: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 server matching 'web', got %d", total)
	}
	if servers[0].Name != "web-prod" {
		t.Fatalf("expected 'web-prod', got '%s'", servers[0].Name)
	}
}

// TestListServersForUser_Pagination verifies pagination for user server list.
func TestListServersForUser_Pagination(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "viewer-4", Username: "viewer4", Email: "v4@v.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	for i := 0; i < 5; i++ {
		id := "s" + string(rune('1'+i))
		s.CreateServer(&model.Server{ID: id, Name: "srv-" + id, Status: "online", Labels: "{}"})
		s.DB().Exec("INSERT INTO permissions (id, user_id, server_id, path) VALUES (?, ?, ?, ?)",
			"p"+id, "viewer-4", id, "/")
	}

	servers, total, err := s.ListServersForUser("viewer-4", model.ServerFilter{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("list servers: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers with limit=2, got %d", len(servers))
	}
}

// TestSetServerIP verifies setting the IP address of a server.
func TestSetServerIP(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	if err := s.SetServerIP("s1", "10.0.0.5"); err != nil {
		t.Fatalf("set server IP: %v", err)
	}

	got, _ := s.GetServerByID("s1")
	if got.IPAddress == nil || *got.IPAddress != "10.0.0.5" {
		t.Fatalf("expected IP '10.0.0.5', got '%v'", got.IPAddress)
	}
}

// TestUpdateServerIP verifies updating the IP address of a server.
func TestUpdateServerIP(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "test", Status: "online", Labels: "{}"})

	if err := s.UpdateServerIP("s1", "192.168.1.1"); err != nil {
		t.Fatalf("update server IP: %v", err)
	}

	got, _ := s.GetServerByID("s1")
	if got.IPAddress == nil || *got.IPAddress != "192.168.1.1" {
		t.Fatalf("expected IP '192.168.1.1', got '%v'", got.IPAddress)
	}
}

// TestGetOnlineServersNotIn_AllActive verifies no stale servers when all online are in active list.
func TestGetOnlineServersNotIn_AllActive(t *testing.T) {
	s := testStore(t)

	s.CreateServer(&model.Server{ID: "s1", Name: "a", Status: "online", Labels: "{}"})
	s.CreateServer(&model.Server{ID: "s2", Name: "b", Status: "online", Labels: "{}"})

	stale, err := s.GetOnlineServersNotIn([]string{"s1", "s2"})
	if err != nil {
		t.Fatalf("get stale: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("expected 0 stale servers, got %d", len(stale))
	}
}
