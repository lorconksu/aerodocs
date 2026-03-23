# Fleet Dashboard & Server Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add server management CRUD, agent registration endpoint, and fleet dashboard UI with mass actions to AeroDocs.

**Architecture:** New SQLite migrations for servers/permissions tables, new store methods, new HTTP handlers, React dashboard page with TanStack Query data fetching.

**Tech Stack:** Same as sub-project 1 -- Go, SQLite, React, TypeScript, Tailwind CSS, TanStack Query

**Spec:** `docs/superpowers/specs/2026-03-23-fleet-dashboard-design.md`

---

## File Map

### Go Backend (`hub/`)

| File | Responsibility |
|------|---------------|
| `hub/internal/migrate/migrations/004_create_servers.sql` | Servers table DDL |
| `hub/internal/migrate/migrations/005_create_permissions.sql` | Permissions table DDL with indexes |
| `hub/internal/model/server.go` | Server, ServerFilter, CreateServerRequest, CreateServerResponse, RegisterAgentRequest, BatchDeleteRequest types |
| `hub/internal/model/audit.go` | Add new audit action constants for server events |
| `hub/internal/store/servers.go` | Server CRUD: Create, GetByID, List, ListForUser, Update, Delete, BatchDelete, GetByToken, Activate |
| `hub/internal/store/servers_test.go` | Tests for all server store methods |
| `hub/internal/server/handlers_servers.go` | Server endpoints: list, create, get, update, delete, batch-delete, register |
| `hub/internal/server/handlers_servers_test.go` | Tests for all server handler endpoints |
| `hub/internal/server/router.go` | Add server route registrations |

### React Frontend (`web/`)

| File | Responsibility |
|------|---------------|
| `web/src/types/api.ts` | Add Server, ServerFilter, CreateServerResponse, BatchDeleteRequest types |
| `web/src/pages/dashboard.tsx` | Rewrite: FleetDashboard with table, filters, mass actions |
| `web/src/pages/add-server-modal.tsx` | Add Server modal component (two-step: name input, then curl command) |
| `web/src/layouts/app-shell.tsx` | Update telemetry bar from hardcoded to live server counts |

---

## Task 1: SQLite Migrations

**Files:**
- Create: `hub/internal/migrate/migrations/004_create_servers.sql`
- Create: `hub/internal/migrate/migrations/005_create_permissions.sql`

- [ ] **Step 1: Create servers table migration**

Create `hub/internal/migrate/migrations/004_create_servers.sql`:

```sql
CREATE TABLE servers (
    id                 TEXT PRIMARY KEY,
    name               TEXT NOT NULL,
    hostname           TEXT,
    ip_address         TEXT,
    os                 TEXT,
    status             TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','online','offline')),
    registration_token TEXT UNIQUE,
    token_expires_at   TEXT,
    agent_version      TEXT,
    labels             TEXT DEFAULT '{}',
    last_seen_at       TEXT,
    created_at         TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at         TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- [ ] **Step 2: Create permissions table migration**

Create `hub/internal/migrate/migrations/005_create_permissions.sql`:

```sql
CREATE TABLE permissions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL,
    server_id  TEXT NOT NULL,
    path       TEXT NOT NULL DEFAULT '/',
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (server_id) REFERENCES servers(id) ON DELETE CASCADE,
    UNIQUE(user_id, server_id, path)
);

CREATE INDEX idx_permissions_user_id ON permissions(user_id);
CREATE INDEX idx_permissions_server_id ON permissions(server_id);
```

- [ ] **Step 3: Verify migrations run**

```bash
cd hub && go test ./internal/store/ -run TestCreateAndGetUser -v
```

The existing `testStore(t)` helper calls `store.New(":memory:")` which runs all migrations. If the new SQL files have syntax errors, this test will fail with a migration error. Expected: passes with no migration errors.

- [ ] **Step 4: Commit**

```bash
git add hub/internal/migrate/migrations/004_create_servers.sql hub/internal/migrate/migrations/005_create_permissions.sql
git commit -m "feat: add servers and permissions table migrations (004, 005)"
```

---

## Task 2: Domain Models

**Files:**
- Create: `hub/internal/model/server.go`
- Modify: `hub/internal/model/audit.go`

- [ ] **Step 1: Create server domain models**

Create `hub/internal/model/server.go`:

```go
package model

import "time"

type Server struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Hostname          *string   `json:"hostname"`
	IPAddress         *string   `json:"ip_address"`
	OS                *string   `json:"os"`
	Status            string    `json:"status"`
	RegistrationToken *string   `json:"-"`
	TokenExpiresAt    *string   `json:"-"`
	AgentVersion      *string   `json:"agent_version"`
	Labels            string    `json:"labels"`
	LastSeenAt        *string   `json:"last_seen_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ServerFilter struct {
	Status *string
	Search *string
	Limit  int
	Offset int
}

type CreateServerRequest struct {
	Name   string `json:"name"`
	Labels string `json:"labels,omitempty"`
}

type CreateServerResponse struct {
	Server            Server `json:"server"`
	RegistrationToken string `json:"registration_token"`
	InstallCommand    string `json:"install_command"`
}

type RegisterAgentRequest struct {
	Token        string `json:"token"`
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ip_address"`
	OS           string `json:"os"`
	AgentVersion string `json:"agent_version"`
}

type BatchDeleteRequest struct {
	IDs []string `json:"ids"`
}
```

- [ ] **Step 2: Add server audit action constants**

Add to `hub/internal/model/audit.go` after the existing constants:

```go
const (
	AuditServerCreated      = "server.created"
	AuditServerUpdated      = "server.updated"
	AuditServerDeleted      = "server.deleted"
	AuditServerBatchDeleted = "server.batch_deleted"
	AuditServerRegistered   = "server.registered"
)
```

- [ ] **Step 3: Verify compilation**

```bash
cd hub && go build ./...
```

Expected: compiles with no errors.

- [ ] **Step 4: Commit**

```bash
git add hub/internal/model/server.go hub/internal/model/audit.go
git commit -m "feat: add Server domain model and audit action constants"
```

---

## Task 3: Server Store Methods

**Files:**
- Create: `hub/internal/store/servers.go`
- Create: `hub/internal/store/servers_test.go`

### 3a: CreateServer and GetServerByID

- [ ] **Step 1: Write test for CreateServer and GetServerByID**

Create `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 2: Run test, verify it fails (method not found)**

```bash
cd hub && go test ./internal/store/ -run TestCreateAndGetServer -v
```

Expected: compilation error -- `CreateServer` and `GetServerByID` methods do not exist.

- [ ] **Step 3: Implement CreateServer, GetServerByID, and scan helpers**

Create `hub/internal/store/servers.go`:

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) CreateServer(srv *model.Server) error {
	_, err := s.db.Exec(
		`INSERT INTO servers (id, name, hostname, ip_address, os, status, registration_token, token_expires_at, agent_version, labels)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		srv.ID, srv.Name, srv.Hostname, srv.IPAddress, srv.OS, srv.Status,
		srv.RegistrationToken, srv.TokenExpiresAt, srv.AgentVersion, srv.Labels,
	)
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}
	return nil
}

func (s *Store) GetServerByID(id string) (*model.Server, error) {
	return s.scanServer(s.db.QueryRow(
		`SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
		        agent_version, labels, last_seen_at, created_at, updated_at
		 FROM servers WHERE id = ?`, id,
	))
}

func (s *Store) scanServer(row *sql.Row) (*model.Server, error) {
	var srv model.Server
	var createdAt, updatedAt string
	err := row.Scan(
		&srv.ID, &srv.Name, &srv.Hostname, &srv.IPAddress, &srv.OS, &srv.Status,
		&srv.RegistrationToken, &srv.TokenExpiresAt,
		&srv.AgentVersion, &srv.Labels, &srv.LastSeenAt, &createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan server: %w", err)
	}
	srv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	srv.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &srv, nil
}

func (s *Store) scanServerRow(rows *sql.Rows) (*model.Server, error) {
	var srv model.Server
	var createdAt, updatedAt string
	err := rows.Scan(
		&srv.ID, &srv.Name, &srv.Hostname, &srv.IPAddress, &srv.OS, &srv.Status,
		&srv.RegistrationToken, &srv.TokenExpiresAt,
		&srv.AgentVersion, &srv.Labels, &srv.LastSeenAt, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan server row: %w", err)
	}
	srv.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	srv.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &srv, nil
}
```

- [ ] **Step 4: Run test, verify it passes**

```bash
cd hub && go test ./internal/store/ -run TestCreateAndGetServer -v
cd hub && go test ./internal/store/ -run TestCreateServer_DuplicateID -v
cd hub && go test ./internal/store/ -run TestGetServerByID_NotFound -v
```

Expected: all three tests pass.

### 3b: ListServers (admin -- all servers)

- [ ] **Step 5: Write test for ListServers**

Add to `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 6: Run test, verify it fails**

```bash
cd hub && go test ./internal/store/ -run TestListServers -v
```

Expected: compilation error -- `ListServers` method does not exist.

- [ ] **Step 7: Implement ListServers**

Add to `hub/internal/store/servers.go`:

```go
func (s *Store) ListServers(filter model.ServerFilter) ([]model.Server, int, error) {
	var where []string
	var args []interface{}

	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Search != nil {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+*filter.Search+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM servers" + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count servers: %w", err)
	}

	// Get paginated results
	query := `SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
	                 agent_version, labels, last_seen_at, created_at, updated_at
	          FROM servers` + whereClause + " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query servers: %w", err)
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		srv, err := s.scanServerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		servers = append(servers, *srv)
	}

	return servers, total, rows.Err()
}
```

Note: add `"strings"` to the import list in `servers.go`.

- [ ] **Step 8: Run test, verify it passes**

```bash
cd hub && go test ./internal/store/ -run TestListServers -v
```

Expected: all `TestListServers*` tests pass.

### 3c: ListServersForUser (viewer-scoped)

- [ ] **Step 9: Write test for ListServersForUser**

Add to `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 10: Run test, verify it fails**

```bash
cd hub && go test ./internal/store/ -run TestListServersForUser -v
```

Expected: compilation error -- `ListServersForUser` does not exist.

- [ ] **Step 11: Implement ListServersForUser**

Add to `hub/internal/store/servers.go`:

```go
func (s *Store) ListServersForUser(userID string, filter model.ServerFilter) ([]model.Server, int, error) {
	var where []string
	var args []interface{}

	// JOIN with permissions table to restrict to user's servers
	joinClause := " INNER JOIN permissions p ON servers.id = p.server_id AND p.user_id = ?"
	args = append(args, userID)

	if filter.Status != nil {
		where = append(where, "servers.status = ?")
		args = append(args, *filter.Status)
	}
	if filter.Search != nil {
		where = append(where, "servers.name LIKE ?")
		args = append(args, "%"+*filter.Search+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(DISTINCT servers.id) FROM servers" + joinClause + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count servers for user: %w", err)
	}

	// Get paginated results
	query := `SELECT DISTINCT servers.id, servers.name, servers.hostname, servers.ip_address,
	                 servers.os, servers.status, servers.registration_token, servers.token_expires_at,
	                 servers.agent_version, servers.labels, servers.last_seen_at,
	                 servers.created_at, servers.updated_at
	          FROM servers` + joinClause + whereClause + " ORDER BY servers.created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query servers for user: %w", err)
	}
	defer rows.Close()

	var servers []model.Server
	for rows.Next() {
		srv, err := s.scanServerRow(rows)
		if err != nil {
			return nil, 0, err
		}
		servers = append(servers, *srv)
	}

	return servers, total, rows.Err()
}
```

- [ ] **Step 12: Run test, verify it passes**

```bash
cd hub && go test ./internal/store/ -run TestListServersForUser -v
```

Expected: passes.

### 3d: UpdateServer

- [ ] **Step 13: Write test for UpdateServer**

Add to `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 14: Run test, verify it fails**

```bash
cd hub && go test ./internal/store/ -run TestUpdateServer -v
```

- [ ] **Step 15: Implement UpdateServer**

Add to `hub/internal/store/servers.go`:

```go
func (s *Store) UpdateServer(id, name, labels string) error {
	result, err := s.db.Exec(
		"UPDATE servers SET name = ?, labels = ?, updated_at = datetime('now') WHERE id = ?",
		name, labels, id,
	)
	if err != nil {
		return fmt.Errorf("update server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}
```

- [ ] **Step 16: Run test, verify it passes**

```bash
cd hub && go test ./internal/store/ -run TestUpdateServer -v
```

### 3e: DeleteServer and DeleteServers (batch)

- [ ] **Step 17: Write tests for DeleteServer and DeleteServers**

Add to `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 18: Run tests, verify they fail**

```bash
cd hub && go test ./internal/store/ -run "TestDeleteServer|TestDeleteServers" -v
```

- [ ] **Step 19: Implement DeleteServer and DeleteServers**

Add to `hub/internal/store/servers.go`:

```go
func (s *Store) DeleteServer(id string) error {
	result, err := s.db.Exec("DELETE FROM servers WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}

func (s *Store) DeleteServers(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "DELETE FROM servers WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("batch delete servers: %w", err)
	}
	return nil
}
```

- [ ] **Step 20: Run tests, verify they pass**

```bash
cd hub && go test ./internal/store/ -run "TestDeleteServer|TestDeleteServers" -v
```

### 3f: GetServerByToken and ActivateServer

- [ ] **Step 21: Write tests for GetServerByToken and ActivateServer**

Add to `hub/internal/store/servers_test.go`:

```go
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
```

- [ ] **Step 22: Run tests, verify they fail**

```bash
cd hub && go test ./internal/store/ -run "TestGetServerByToken|TestActivateServer" -v
```

- [ ] **Step 23: Implement GetServerByToken and ActivateServer**

Add to `hub/internal/store/servers.go`:

```go
func (s *Store) GetServerByToken(tokenHash string) (*model.Server, error) {
	return s.scanServer(s.db.QueryRow(
		`SELECT id, name, hostname, ip_address, os, status, registration_token, token_expires_at,
		        agent_version, labels, last_seen_at, created_at, updated_at
		 FROM servers WHERE registration_token = ?`, tokenHash,
	))
}

func (s *Store) ActivateServer(id, hostname, ip, os, agentVersion string) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	result, err := s.db.Exec(
		`UPDATE servers
		 SET hostname = ?, ip_address = ?, os = ?, agent_version = ?,
		     status = 'online', registration_token = NULL, token_expires_at = NULL,
		     last_seen_at = ?, updated_at = datetime('now')
		 WHERE id = ?`,
		hostname, ip, os, agentVersion, now, id,
	)
	if err != nil {
		return fmt.Errorf("activate server: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("server not found")
	}
	return nil
}
```

- [ ] **Step 24: Run tests, verify they pass**

```bash
cd hub && go test ./internal/store/ -run "TestGetServerByToken|TestActivateServer" -v
```

- [ ] **Step 25: Run all store tests to confirm nothing is broken**

```bash
cd hub && go test ./internal/store/ -v
```

Expected: all tests pass.

- [ ] **Step 26: Commit**

```bash
git add hub/internal/store/servers.go hub/internal/store/servers_test.go
git commit -m "feat: add server store methods with CRUD, batch delete, token lookup, and activation"
```

---

## Task 4: HTTP Handlers

**Files:**
- Create: `hub/internal/server/handlers_servers.go`
- Create: `hub/internal/server/handlers_servers_test.go`
- Modify: `hub/internal/server/router.go`

### 4a: List Servers endpoint

- [ ] **Step 1: Write test for GET /api/servers (admin sees all)**

Create `hub/internal/server/handlers_servers_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestListServers_Admin(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create servers directly in the store
	s.store.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
}

func TestListServers_FilterByStatus(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "alpha", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "beta", Status: "pending", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers?status=online", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	total := int(resp["total"].(float64))
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
}
```

- [ ] **Step 2: Run test, verify it fails**

```bash
cd hub && go test ./internal/server/ -run TestListServers -v
```

Expected: fails because route and handler do not exist yet.

- [ ] **Step 3: Implement handleListServers**

Create `hub/internal/server/handlers_servers.go`:

```go
package server

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListServers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := model.ServerFilter{
		Limit:  50,
		Offset: 0,
	}

	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			filter.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}
	if v := q.Get("status"); v != "" {
		filter.Status = &v
	}
	if v := q.Get("search"); v != "" {
		filter.Search = &v
	}

	role := UserRoleFromContext(r.Context())
	userID := UserIDFromContext(r.Context())

	var servers []model.Server
	var total int
	var err error

	if role == "admin" {
		servers, total, err = s.store.ListServers(filter)
	} else {
		servers, total, err = s.store.ListServersForUser(userID, filter)
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"servers": servers,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}
```

- [ ] **Step 4: Register route in router.go**

Add to `hub/internal/server/router.go` in the `routes()` method, before the SPA catch-all:

```go
	// Server endpoints (any authenticated user, role-filtered in handler)
	mux.Handle("GET /api/servers", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleListServers))))
```

- [ ] **Step 5: Run test, verify it passes**

```bash
cd hub && go test ./internal/server/ -run TestListServers -v
```

### 4b: Create Server endpoint

- [ ] **Step 6: Write test for POST /api/servers**

Add to `hub/internal/server/handlers_servers_test.go`:

```go
func TestCreateServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateServerRequest{
		Name: "web-prod-1",
	})

	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.CreateServerResponse
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Server.Name != "web-prod-1" {
		t.Fatalf("expected name 'web-prod-1', got '%s'", resp.Server.Name)
	}
	if resp.Server.Status != "pending" {
		t.Fatalf("expected status 'pending', got '%s'", resp.Server.Status)
	}
	if resp.RegistrationToken == "" {
		t.Fatal("expected registration_token in response")
	}
	if resp.InstallCommand == "" {
		t.Fatal("expected install_command in response")
	}
}

func TestCreateServer_EmptyName(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateServerRequest{Name: ""})

	req := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
```

- [ ] **Step 7: Run test, verify it fails**

```bash
cd hub && go test ./internal/server/ -run TestCreateServer -v
```

- [ ] **Step 8: Implement handleCreateServer**

Add to `hub/internal/server/handlers_servers.go`:

```go
func (s *Server) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var req model.CreateServerRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "server name is required")
		return
	}

	if req.Labels == "" {
		req.Labels = "{}"
	}

	// Generate raw registration token
	rawToken := uuid.NewString()

	// Hash it for storage
	hash := sha256.Sum256([]byte(rawToken))
	tokenHash := fmt.Sprintf("%x", hash)

	expiresAt := time.Now().Add(1 * time.Hour).UTC().Format("2006-01-02 15:04:05")

	srv := &model.Server{
		ID:                uuid.NewString(),
		Name:              req.Name,
		Status:            "pending",
		RegistrationToken: &tokenHash,
		TokenExpiresAt:    &expiresAt,
		Labels:            req.Labels,
	}

	if err := s.store.CreateServer(srv); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to create server")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerCreated, Target: &srv.ID, IPAddress: &ip,
	})

	installCmd := fmt.Sprintf(
		"curl -sSL https://aerodocs.yiucloud.com/install.sh | sudo bash -s -- --token %s --hub https://aerodocs.yiucloud.com",
		rawToken,
	)

	respondJSON(w, http.StatusCreated, model.CreateServerResponse{
		Server:            *srv,
		RegistrationToken: rawToken,
		InstallCommand:    installCmd,
	})
}
```

- [ ] **Step 9: Register route in router.go**

Add to `hub/internal/server/router.go`:

```go
	mux.Handle("POST /api/servers", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleCreateServer)))))
```

- [ ] **Step 10: Run test, verify it passes**

```bash
cd hub && go test ./internal/server/ -run TestCreateServer -v
```

### 4c: Get, Update, Delete Server endpoints

- [ ] **Step 11: Write tests for GET/PUT/DELETE /api/servers/:id**

Add to `hub/internal/server/handlers_servers_test.go`:

```go
func TestGetServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "test-srv", Status: "online", Labels: "{}"})

	req := httptest.NewRequest("GET", "/api/servers/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var srv model.Server
	json.NewDecoder(rec.Body).Decode(&srv)
	if srv.Name != "test-srv" {
		t.Fatalf("expected 'test-srv', got '%s'", srv.Name)
	}
}

func TestGetServer_NotFound(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	req := httptest.NewRequest("GET", "/api/servers/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdateServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "old-name", Status: "online", Labels: "{}"})

	body, _ := json.Marshal(map[string]string{"name": "new-name", "labels": `{"env":"staging"}`})

	req := httptest.NewRequest("PUT", "/api/servers/s1", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteServer(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "doomed", Status: "online", Labels: "{}"})

	req := httptest.NewRequest("DELETE", "/api/servers/s1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone
	_, err := s.store.GetServerByID("s1")
	if err == nil {
		t.Fatal("expected server to be deleted")
	}
}
```

- [ ] **Step 12: Run tests, verify they fail**

```bash
cd hub && go test ./internal/server/ -run "TestGetServer|TestUpdateServer|TestDeleteServer" -v
```

- [ ] **Step 13: Implement handleGetServer, handleUpdateServer, handleDeleteServer**

Add to `hub/internal/server/handlers_servers.go`:

```go
func (s *Server) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	srv, err := s.store.GetServerByID(id)
	if err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	// Viewers must have permission
	role := UserRoleFromContext(r.Context())
	if role != "admin" {
		userID := UserIDFromContext(r.Context())
		servers, _, err := s.store.ListServersForUser(userID, model.ServerFilter{Limit: 1})
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to check permissions")
			return
		}
		found := false
		for _, permitted := range servers {
			if permitted.ID == id {
				found = true
				break
			}
		}
		if !found {
			respondError(w, http.StatusForbidden, "access denied")
			return
		}
	}

	respondJSON(w, http.StatusOK, srv)
}

func (s *Server) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name   string `json:"name"`
		Labels string `json:"labels"`
	}
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "server name is required")
		return
	}

	if err := s.store.UpdateServer(id, req.Name, req.Labels); err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerUpdated, Target: &id, IPAddress: &ip,
	})

	srv, _ := s.store.GetServerByID(id)
	respondJSON(w, http.StatusOK, srv)
}

func (s *Server) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	if err := s.store.DeleteServer(id); err != nil {
		respondError(w, http.StatusNotFound, "server not found")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerDeleted, Target: &id, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
```

- [ ] **Step 14: Register routes in router.go**

Add to `hub/internal/server/router.go`:

```go
	mux.Handle("GET /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleGetServer))))
	mux.Handle("PUT /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleUpdateServer)))))
	mux.Handle("DELETE /api/servers/{id}", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleDeleteServer)))))
```

- [ ] **Step 15: Run tests, verify they pass**

```bash
cd hub && go test ./internal/server/ -run "TestGetServer|TestUpdateServer|TestDeleteServer" -v
```

### 4d: Batch Delete endpoint

- [ ] **Step 16: Write test for POST /api/servers/batch-delete**

Add to `hub/internal/server/handlers_servers_test.go`:

```go
func TestBatchDeleteServers(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	s.store.CreateServer(&model.Server{ID: "s1", Name: "a", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s2", Name: "b", Status: "online", Labels: "{}"})
	s.store.CreateServer(&model.Server{ID: "s3", Name: "c", Status: "online", Labels: "{}"})

	body, _ := json.Marshal(model.BatchDeleteRequest{IDs: []string{"s1", "s3"}})

	req := httptest.NewRequest("POST", "/api/servers/batch-delete", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify only s2 remains
	servers, total, _ := s.store.ListServers(model.ServerFilter{Limit: 50})
	if total != 1 {
		t.Fatalf("expected 1 remaining, got %d", total)
	}
	if servers[0].ID != "s2" {
		t.Fatalf("expected s2 to survive, got %s", servers[0].ID)
	}
}

func TestBatchDeleteServers_EmptyList(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.BatchDeleteRequest{IDs: []string{}})

	req := httptest.NewRequest("POST", "/api/servers/batch-delete", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
```

- [ ] **Step 17: Run test, verify it fails**

```bash
cd hub && go test ./internal/server/ -run TestBatchDeleteServers -v
```

- [ ] **Step 18: Implement handleBatchDeleteServers**

Add to `hub/internal/server/handlers_servers.go`:

```go
func (s *Server) handleBatchDeleteServers(w http.ResponseWriter, r *http.Request) {
	var req model.BatchDeleteRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.IDs) == 0 {
		respondError(w, http.StatusBadRequest, "ids list cannot be empty")
		return
	}

	if err := s.store.DeleteServers(req.IDs); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to delete servers")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	detail := fmt.Sprintf("deleted %d servers", len(req.IDs))
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditServerBatchDeleted, Detail: &detail, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "deleted",
		"deleted": len(req.IDs),
	})
}
```

- [ ] **Step 19: Register route in router.go**

Add to `hub/internal/server/router.go` -- this must be registered **before** the `/api/servers/{id}` routes so Go's ServeMux matches the literal path first:

```go
	mux.Handle("POST /api/servers/batch-delete", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleBatchDeleteServers)))))
```

- [ ] **Step 20: Run test, verify it passes**

```bash
cd hub && go test ./internal/server/ -run TestBatchDeleteServers -v
```

### 4e: Agent Registration endpoint

- [ ] **Step 21: Write test for POST /api/servers/register**

Add to `hub/internal/server/handlers_servers_test.go`:

```go
func TestRegisterAgent_Success(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create a server via the API to get a raw token
	createBody, _ := json.Marshal(model.CreateServerRequest{Name: "agent-test"})
	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateServerResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	rawToken := createResp.RegistrationToken

	// Register agent (no auth required)
	regBody, _ := json.Marshal(model.RegisterAgentRequest{
		Token:        rawToken,
		Hostname:     "agent-host",
		IPAddress:    "10.10.1.50",
		OS:           "Ubuntu 24.04",
		AgentVersion: "0.1.0",
	})
	regReq := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", regRec.Code, regRec.Body.String())
	}

	var regResp map[string]interface{}
	json.NewDecoder(regRec.Body).Decode(&regResp)
	if regResp["status"] != "online" {
		t.Fatalf("expected status 'online', got '%v'", regResp["status"])
	}
}

func TestRegisterAgent_InvalidToken(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterAgentRequest{
		Token: "totally-fake-token", Hostname: "h", IPAddress: "1.1.1.1", OS: "Linux", AgentVersion: "0.1",
	})
	req := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRegisterAgent_AlreadyUsed(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	// Create and register
	createBody, _ := json.Marshal(model.CreateServerRequest{Name: "double-reg"})
	createReq := httptest.NewRequest("POST", "/api/servers", bytes.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createRec := httptest.NewRecorder()
	s.routes().ServeHTTP(createRec, createReq)

	var createResp model.CreateServerResponse
	json.NewDecoder(createRec.Body).Decode(&createResp)
	rawToken := createResp.RegistrationToken

	// First registration
	regBody, _ := json.Marshal(model.RegisterAgentRequest{
		Token: rawToken, Hostname: "h", IPAddress: "1.1.1.1", OS: "Linux", AgentVersion: "0.1",
	})
	regReq := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec, regReq)

	if regRec.Code != http.StatusOK {
		t.Fatalf("first register: expected 200, got %d", regRec.Code)
	}

	// Second registration with same token
	regReq2 := httptest.NewRequest("POST", "/api/servers/register", bytes.NewReader(regBody))
	regRec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(regRec2, regReq2)

	if regRec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for reused token, got %d: %s", regRec2.Code, regRec2.Body.String())
	}
}
```

- [ ] **Step 22: Run tests, verify they fail**

```bash
cd hub && go test ./internal/server/ -run "TestRegisterAgent" -v
```

- [ ] **Step 23: Implement handleRegisterAgent**

Add to `hub/internal/server/handlers_servers.go`:

```go
func (s *Server) handleRegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Token == "" {
		respondError(w, http.StatusBadRequest, "token is required")
		return
	}

	// Hash the raw token to look up the server
	hash := sha256.Sum256([]byte(req.Token))
	tokenHash := fmt.Sprintf("%x", hash)

	srv, err := s.store.GetServerByToken(tokenHash)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "invalid or expired registration token")
		return
	}

	// Check if token is expired
	if srv.TokenExpiresAt != nil {
		expiresAt, err := time.Parse("2006-01-02 15:04:05", *srv.TokenExpiresAt)
		if err == nil && time.Now().UTC().After(expiresAt) {
			respondError(w, http.StatusUnauthorized, "invalid or expired registration token")
			return
		}
	}

	// Activate the server
	if err := s.store.ActivateServer(srv.ID, req.Hostname, req.IPAddress, req.OS, req.AgentVersion); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to activate server")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID:     uuid.NewString(),
		Action: model.AuditServerRegistered, Target: &srv.ID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{
		"server_id": srv.ID,
		"status":    "online",
	})
}
```

- [ ] **Step 24: Register route in router.go**

Add to `hub/internal/server/router.go` -- this is a public endpoint (no auth required):

```go
	mux.Handle("POST /api/servers/register", loggingMiddleware(http.HandlerFunc(s.handleRegisterAgent)))
```

- [ ] **Step 25: Run tests, verify they pass**

```bash
cd hub && go test ./internal/server/ -run "TestRegisterAgent" -v
```

- [ ] **Step 26: Run full test suite**

```bash
cd hub && go test ./... -v
```

Expected: all tests pass -- both old and new.

- [ ] **Step 27: Commit**

```bash
git add hub/internal/server/handlers_servers.go hub/internal/server/handlers_servers_test.go hub/internal/server/router.go
git commit -m "feat: add server CRUD handlers, batch delete, and agent registration endpoint"
```

---

## Task 5: Frontend Types and API Additions

**Files:**
- Modify: `web/src/types/api.ts`

- [ ] **Step 1: Add server-related TypeScript types**

Add to `web/src/types/api.ts`:

```ts
export type ServerStatus = 'pending' | 'online' | 'offline'

export interface Server {
  id: string
  name: string
  hostname: string | null
  ip_address: string | null
  os: string | null
  status: ServerStatus
  agent_version: string | null
  labels: string
  last_seen_at: string | null
  created_at: string
  updated_at: string
}

export interface ServerListResponse {
  servers: Server[]
  total: number
  limit: number
  offset: number
}

export interface CreateServerRequest {
  name: string
  labels?: string
}

export interface CreateServerResponse {
  server: Server
  registration_token: string
  install_command: string
}

export interface BatchDeleteRequest {
  ids: string[]
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/types/api.ts
git commit -m "feat: add server TypeScript types for fleet dashboard"
```

---

## Task 6: Fleet Dashboard Page

**Files:**
- Rewrite: `web/src/pages/dashboard.tsx`

- [ ] **Step 1: Rewrite dashboard with server table and filters**

Rewrite `web/src/pages/dashboard.tsx` with the full FleetDashboard component. Key elements:

- TanStack Query `useQuery(['servers', filters])` to fetch `GET /api/servers` with status/search params
- State for `selectedIds: Set<string>`, `statusFilter`, and `searchTerm`
- Status filter buttons (All / Online / Offline / Pending)
- Search input with debounce
- Server table with columns: Checkbox (admin only), Status dot, Name (link to `/servers/:id`), Hostname/IP, OS, Last Seen, Actions
- Status dots: green for `online`, red for `offline`, amber for `pending`
- "Pending agent" in amber text for pending servers in the Last Seen column
- Actions column: Edit/Delete for online/offline, Show command/Delete for pending
- Select-all checkbox in table header
- Mass action bar when `selectedIds.size > 0`: shows count, "Delete Selected" button, "Clear" button
- `useMutation` for `DELETE /api/servers/:id` with `queryClient.invalidateQueries(['servers'])`
- `useMutation` for `POST /api/servers/batch-delete` with query invalidation
- "+ Add Server" button in page header (admin only) that opens AddServerModal
- Empty state when no servers exist
- Relative time display for `last_seen_at` (e.g., "2 min ago")

```tsx
import { useState, useMemo } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { Plus, Trash2, X, Search } from 'lucide-react'
import { apiFetch } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import { AddServerModal } from '@/pages/add-server-modal'
import type { ServerListResponse, ServerStatus } from '@/types/api'

function relativeTime(dateStr: string | null): string {
  if (!dateStr) return '—'
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffSec = Math.floor(diffMs / 1000)
  if (diffSec < 60) return `${diffSec}s ago`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin} min ago`
  const diffHr = Math.floor(diffMin / 60)
  if (diffHr < 24) return `${diffHr}h ago`
  const diffDay = Math.floor(diffHr / 24)
  return `${diffDay}d ago`
}

const statusDot: Record<ServerStatus, string> = {
  online: 'text-status-online',
  offline: 'text-status-offline',
  pending: 'text-status-warning',
}

export function DashboardPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const isAdmin = user?.role === 'admin'

  const [statusFilter, setStatusFilter] = useState<string | undefined>()
  const [searchTerm, setSearchTerm] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [showAddModal, setShowAddModal] = useState(false)

  const { data, isLoading } = useQuery({
    queryKey: ['servers', statusFilter, searchTerm],
    queryFn: () => {
      const params = new URLSearchParams()
      if (statusFilter) params.set('status', statusFilter)
      if (searchTerm) params.set('search', searchTerm)
      const qs = params.toString()
      return apiFetch<ServerListResponse>(`/servers${qs ? `?${qs}` : ''}`)
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiFetch(`/servers/${id}`, { method: 'DELETE' }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['servers'] }),
  })

  const batchDeleteMutation = useMutation({
    mutationFn: (ids: string[]) =>
      apiFetch('/servers/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ ids }),
      }),
    onSuccess: () => {
      setSelectedIds(new Set())
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  const servers = data?.servers ?? []
  const total = data?.total ?? 0

  const allSelected = servers.length > 0 && servers.every((s) => selectedIds.has(s.id))

  const toggleSelect = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(servers.map((s) => s.id)))
    }
  }

  const statusFilters: { label: string; value: string | undefined }[] = [
    { label: 'All', value: undefined },
    { label: 'Online', value: 'online' },
    { label: 'Offline', value: 'offline' },
    { label: 'Pending', value: 'pending' },
  ]

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-lg font-semibold text-text-primary">Fleet Dashboard</h2>
          <p className="text-text-muted text-sm">{total} server{total !== 1 ? 's' : ''}</p>
        </div>
        {isAdmin && (
          <button
            onClick={() => setShowAddModal(true)}
            className="flex items-center gap-2 px-3 py-1.5 bg-accent hover:bg-accent-hover text-white text-sm rounded transition-colors"
          >
            <Plus className="w-4 h-4" />
            Add Server
          </button>
        )}
      </div>

      {/* Filters */}
      <div className="flex items-center gap-4 mb-4">
        <div className="flex gap-1">
          {statusFilters.map(({ label, value }) => (
            <button
              key={label}
              onClick={() => setStatusFilter(value)}
              className={`px-3 py-1 text-xs rounded transition-colors ${
                statusFilter === value
                  ? 'bg-accent text-white'
                  : 'bg-elevated text-text-muted hover:text-text-secondary'
              }`}
            >
              {label}
            </button>
          ))}
        </div>
        <div className="relative flex-1 max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-text-faint" />
          <input
            type="text"
            placeholder="Search servers..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-8 pr-3 py-1.5 bg-elevated border border-border rounded text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          />
        </div>
      </div>

      {/* Mass action bar */}
      {selectedIds.size > 0 && (
        <div className="flex items-center gap-3 mb-4 px-3 py-2 bg-elevated border border-border rounded text-sm">
          <span className="text-text-secondary">{selectedIds.size} selected</span>
          <button
            onClick={() => batchDeleteMutation.mutate([...selectedIds])}
            disabled={batchDeleteMutation.isPending}
            className="flex items-center gap-1 px-2 py-1 text-status-offline hover:bg-surface rounded transition-colors text-xs"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Delete Selected
          </button>
          <button
            onClick={() => setSelectedIds(new Set())}
            className="flex items-center gap-1 px-2 py-1 text-text-muted hover:text-text-secondary text-xs"
          >
            <X className="w-3.5 h-3.5" />
            Clear
          </button>
        </div>
      )}

      {/* Table */}
      {isLoading ? (
        <div className="text-text-muted text-sm py-8 text-center">Loading servers...</div>
      ) : servers.length === 0 ? (
        <div className="text-text-muted text-sm py-8 text-center">
          No servers found.{' '}
          {isAdmin && (
            <button onClick={() => setShowAddModal(true)} className="text-accent hover:underline">
              Add your first server
            </button>
          )}
        </div>
      ) : (
        <div className="border border-border rounded overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="bg-elevated text-text-muted text-xs uppercase tracking-wider">
                {isAdmin && (
                  <th className="px-3 py-2 w-8">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      onChange={toggleSelectAll}
                      className="rounded"
                    />
                  </th>
                )}
                <th className="px-3 py-2 w-8">Status</th>
                <th className="px-3 py-2 text-left">Name</th>
                <th className="px-3 py-2 text-left">Hostname / IP</th>
                <th className="px-3 py-2 text-left">OS</th>
                <th className="px-3 py-2 text-left">Last Seen</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {servers.map((srv) => (
                <tr key={srv.id} className="hover:bg-elevated/50 transition-colors">
                  {isAdmin && (
                    <td className="px-3 py-2">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(srv.id)}
                        onChange={() => toggleSelect(srv.id)}
                        className="rounded"
                      />
                    </td>
                  )}
                  <td className="px-3 py-2 text-center">
                    <span className={statusDot[srv.status]}>●</span>
                  </td>
                  <td className="px-3 py-2">
                    <Link
                      to={`/servers/${srv.id}`}
                      className="text-text-primary hover:text-accent transition-colors"
                    >
                      {srv.name}
                    </Link>
                  </td>
                  <td className="px-3 py-2 font-mono text-text-secondary text-xs">
                    {srv.hostname || srv.ip_address ? `${srv.hostname ?? '—'} / ${srv.ip_address ?? '—'}` : '—'}
                  </td>
                  <td className="px-3 py-2 text-text-secondary">{srv.os ?? '—'}</td>
                  <td className="px-3 py-2 text-text-muted">
                    {srv.status === 'pending' ? (
                      <span className="text-status-warning">Pending agent</span>
                    ) : (
                      relativeTime(srv.last_seen_at)
                    )}
                  </td>
                  <td className="px-3 py-2 text-right">
                    {isAdmin && (
                      <button
                        onClick={() => deleteMutation.mutate(srv.id)}
                        disabled={deleteMutation.isPending}
                        className="text-text-muted hover:text-status-offline transition-colors text-xs"
                      >
                        Delete
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Add Server Modal */}
      {showAddModal && <AddServerModal onClose={() => setShowAddModal(false)} />}
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit
```

Note: this will fail until the `AddServerModal` component exists (Task 7). If running this step before Task 7, temporarily comment out the AddServerModal import and usage, or implement Task 7 first.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/dashboard.tsx
git commit -m "feat: rewrite FleetDashboard with server table, filters, and mass actions"
```

---

## Task 7: Add Server Modal

**Files:**
- Create: `web/src/pages/add-server-modal.tsx`

- [ ] **Step 1: Create the AddServerModal component**

Create `web/src/pages/add-server-modal.tsx`:

```tsx
import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { X, Copy, Check } from 'lucide-react'
import { apiFetch } from '@/lib/api'
import type { CreateServerRequest, CreateServerResponse } from '@/types/api'

interface AddServerModalProps {
  onClose: () => void
}

export function AddServerModal({ onClose }: AddServerModalProps) {
  const queryClient = useQueryClient()
  const [name, setName] = useState('')
  const [result, setResult] = useState<CreateServerResponse | null>(null)
  const [copied, setCopied] = useState(false)

  const createMutation = useMutation({
    mutationFn: (req: CreateServerRequest) =>
      apiFetch<CreateServerResponse>('/servers', {
        method: 'POST',
        body: JSON.stringify(req),
      }),
    onSuccess: (data) => {
      setResult(data)
      queryClient.invalidateQueries({ queryKey: ['servers'] })
    },
  })

  const handleGenerate = () => {
    if (!name.trim()) return
    createMutation.mutate({ name: name.trim() })
  }

  const handleCopy = async () => {
    if (!result) return
    await navigator.clipboard.writeText(result.install_command)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-surface border border-border rounded-lg w-full max-w-lg mx-4 p-6">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-text-primary font-semibold">Add Server</h3>
          <button onClick={onClose} className="text-text-muted hover:text-text-primary">
            <X className="w-4 h-4" />
          </button>
        </div>

        {!result ? (
          /* Step 1: Enter name */
          <div>
            <label className="block text-sm text-text-secondary mb-1">Server Name</label>
            <input
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleGenerate()}
              placeholder="e.g., web-prod-1"
              className="w-full px-3 py-2 bg-elevated border border-border rounded text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
              autoFocus
            />
            {createMutation.isError && (
              <p className="text-status-error text-xs mt-2">
                {createMutation.error?.message || 'Failed to create server'}
              </p>
            )}
            <div className="flex justify-end mt-4">
              <button
                onClick={handleGenerate}
                disabled={!name.trim() || createMutation.isPending}
                className="px-4 py-2 bg-accent hover:bg-accent-hover disabled:opacity-50 text-white text-sm rounded transition-colors"
              >
                {createMutation.isPending ? 'Generating...' : 'Generate'}
              </button>
            </div>
          </div>
        ) : (
          /* Step 2: Show install command */
          <div>
            <p className="text-sm text-text-secondary mb-3">
              Run this command on <span className="text-text-primary font-medium">{result.server.name}</span> to install the agent:
            </p>
            <div className="relative">
              <pre className="bg-base border border-border rounded p-3 text-xs font-mono text-text-secondary overflow-x-auto whitespace-pre-wrap break-all">
                {result.install_command}
              </pre>
              <button
                onClick={handleCopy}
                className="absolute top-2 right-2 p-1 bg-elevated border border-border rounded text-text-muted hover:text-text-primary transition-colors"
                title="Copy to clipboard"
              >
                {copied ? <Check className="w-3.5 h-3.5 text-status-online" /> : <Copy className="w-3.5 h-3.5" />}
              </button>
            </div>
            <p className="text-xs text-status-warning mt-2">
              Expires in 1 hour. Token is shown only once.
            </p>
            <div className="flex justify-end mt-4">
              <button
                onClick={onClose}
                className="px-4 py-2 bg-elevated hover:bg-border text-text-primary text-sm rounded transition-colors"
              >
                Close
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/add-server-modal.tsx
git commit -m "feat: add AddServerModal component with two-step token generation flow"
```

---

## Task 8: Telemetry Bar Update

**Files:**
- Modify: `web/src/layouts/app-shell.tsx`

- [ ] **Step 1: Update AppShell to show live server counts**

Modify `web/src/layouts/app-shell.tsx` to:

1. Import `useQuery` from `@tanstack/react-query` and `apiFetch` from `@/lib/api`
2. Import `ServerListResponse` from `@/types/api`
3. Add a query to fetch servers: `useQuery({ queryKey: ['servers'], queryFn: () => apiFetch<ServerListResponse>('/servers?limit=1000') })`
4. Compute counts from the returned `servers` array:
   - `onlineCount = servers.filter(s => s.status === 'online').length`
   - `offlineCount = servers.filter(s => s.status === 'offline').length`
   - `pendingCount = servers.filter(s => s.status === 'pending').length`
5. Replace the hardcoded `0 Online` / `0 Offline` spans with live counts
6. Add a pending count span (only shown when `pendingCount > 0`)

Replace the Fleet Health section of the header:

```tsx
<span className="text-text-muted uppercase tracking-widest text-[10px]">Fleet Health</span>
<span className="text-status-online">● {onlineCount} Online</span>
<span className="text-status-offline">● {offlineCount} Offline</span>
{pendingCount > 0 && (
  <span className="text-status-warning">● {pendingCount} Pending</span>
)}
```

- [ ] **Step 2: Verify TypeScript compiles**

```bash
cd web && npx tsc --noEmit
```

- [ ] **Step 3: Verify the frontend builds**

```bash
cd web && npm run build
```

Expected: builds successfully.

- [ ] **Step 4: Run full Go test suite**

```bash
cd hub && go test ./... -v
```

Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/src/layouts/app-shell.tsx
git commit -m "feat: update telemetry bar to show live server counts from API"
```

---

## Summary Checklist

| Task | Files | Tests |
|------|-------|-------|
| 1. SQLite Migrations | `004_create_servers.sql`, `005_create_permissions.sql` | Verified via existing store tests |
| 2. Domain Models | `model/server.go`, `model/audit.go` | Compilation check |
| 3. Store Methods | `store/servers.go`, `store/servers_test.go` | 13 store tests |
| 4. HTTP Handlers | `server/handlers_servers.go`, `server/handlers_servers_test.go`, `server/router.go` | 10 handler tests |
| 5. Frontend Types | `types/api.ts` | TypeScript compilation |
| 6. Fleet Dashboard | `pages/dashboard.tsx` | TypeScript compilation |
| 7. Add Server Modal | `pages/add-server-modal.tsx` | TypeScript compilation |
| 8. Telemetry Bar | `layouts/app-shell.tsx` | TypeScript compilation + full build |
