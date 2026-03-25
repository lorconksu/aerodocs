# Foundation & Auth Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the AeroDocs monorepo with Go backend, React frontend, SQLite database, and a complete mandatory-2FA authentication system.

**Architecture:** Layered Go backend (handler → store → SQLite) serving a Vite-built React SPA via `go:embed`. JWT-based auth with four token types (setup, totp, access, refresh). All users must complete TOTP setup before accessing the app.

**Tech Stack:** Go 1.26+, SQLite (modernc.org/sqlite), React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query, React Router v7, golang-jwt/jwt/v5, pquerna/otp, bcrypt

**Spec:** `docs/superpowers/specs/2026-03-23-foundation-and-auth-design.md`

---

## File Map

### Go Backend (`hub/`)

| File | Responsibility |
|------|---------------|
| `hub/cmd/aerodocs/main.go` | Entry point: parse flags, open DB, run migrations, start server |
| `hub/cmd/aerodocs/admin.go` | CLI break-glass: `admin reset-totp` subcommand |
| `hub/internal/model/user.go` | User, CreateUserRequest, LoginRequest, TokenPair types |
| `hub/internal/model/audit.go` | AuditEntry, AuditFilter types |
| `hub/internal/migrate/migrate.go` | Migration runner (read SQL files, track in `_migrations`) |
| `hub/internal/migrate/migrations/001_create_users.sql` | Users table DDL |
| `hub/internal/migrate/migrations/002_create_audit_logs.sql` | Audit logs table DDL |
| `hub/internal/migrate/migrations/003_create_config.sql` | Config table DDL |
| `hub/internal/store/store.go` | Store struct, constructor, transaction helper |
| `hub/internal/store/users.go` | User CRUD: Create, GetByID, GetByUsername, UpdateTOTP, List |
| `hub/internal/store/audit.go` | Audit insert + paginated query |
| `hub/internal/store/config.go` | Config get/set (JWT signing key) |
| `hub/internal/auth/password.go` | HashPassword, ComparePassword, ValidatePolicy |
| `hub/internal/auth/jwt.go` | GenerateTokenPair, GenerateSetupToken, GenerateTOTPToken, ValidateToken |
| `hub/internal/auth/totp.go` | GenerateSecret, GenerateQRURL, ValidateCode |
| `hub/internal/server/server.go` | Server struct, New(), Start(), Shutdown() |
| `hub/internal/server/router.go` | Route registration with token type requirements |
| `hub/internal/server/middleware.go` | Auth, CORS, logging, rate limiting middleware |
| `hub/internal/server/handlers_auth.go` | Auth endpoints: status, register, login, login/totp, refresh, me, totp/* |
| `hub/internal/server/handlers_users.go` | User management: list, create |
| `hub/internal/server/handlers_audit.go` | Audit log listing |
| `hub/internal/server/respond.go` | JSON response helpers |
| `hub/embed.go` | `go:embed` directive for `web/dist` |

### React Frontend (`web/`)

| File | Responsibility |
|------|---------------|
| `web/src/styles/tokens.css` | CSS custom properties (colors, spacing) |
| `web/src/types/api.ts` | TypeScript interfaces: User, TokenPair, LoginRequest, etc. |
| `web/src/lib/api.ts` | Fetch wrapper with JWT injection + 401 refresh interceptor |
| `web/src/lib/auth.ts` | Token storage (localStorage), refresh logic |
| `web/src/lib/query-client.ts` | TanStack Query client config |
| `web/src/hooks/use-auth.ts` | AuthProvider context + useAuth hook |
| `web/src/components/ui/*` | shadcn/ui primitives (Button, Input, Card, Label) |
| `web/src/layouts/auth-layout.tsx` | Centered card layout for login/setup pages |
| `web/src/layouts/app-shell.tsx` | Authenticated shell: telemetry bar + sidebar + outlet |
| `web/src/pages/login.tsx` | Username + password form |
| `web/src/pages/login-totp.tsx` | 6-digit TOTP code input |
| `web/src/pages/setup.tsx` | Initial admin registration form |
| `web/src/pages/setup-totp.tsx` | QR code display + TOTP verification |
| `web/src/pages/dashboard.tsx` | Stub placeholder for sub-project 2 |
| `web/src/App.tsx` | Route definitions + AuthProvider wrapper |
| `web/src/main.tsx` | React DOM render entry point |

### Root

| File | Responsibility |
|------|---------------|
| `Makefile` | Build orchestration: `dev-hub`, `dev-web`, `build`, `test` |
| `.gitignore` | Ignore patterns for Go, Node, build artifacts |

---

## Task 1: Project Scaffolding

**Files:**
- Create: `hub/go.mod`, `hub/cmd/aerodocs/main.go`
- Create: `agent/go.mod`, `agent/cmd/aerodocs-agent/main.go`
- Create: `web/package.json`, `web/vite.config.ts`, `web/tsconfig.json`, `web/index.html`, `web/src/main.tsx`, `web/src/App.tsx`
- Create: `Makefile`, `.gitignore`
- Create: `proto/aerodocs/v1/.gitkeep`

- [ ] **Step 1: Initialize Go module for hub**

```bash
cd hub && go mod init github.com/wyiu/aerodocs/hub
```

- [ ] **Step 2: Create hub entry point**

Create `hub/cmd/aerodocs/main.go`:

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("AeroDocs Hub starting...")
	return nil
}
```

- [ ] **Step 3: Verify hub compiles**

```bash
cd hub && go build ./cmd/aerodocs/
```

Expected: builds successfully, produces `aerodocs` binary.

- [ ] **Step 4: Initialize Go module for agent**

```bash
cd agent && go mod init github.com/wyiu/aerodocs/agent
```

Create `agent/cmd/aerodocs-agent/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("AeroDocs Agent — placeholder for sub-project 3")
}
```

- [ ] **Step 5: Scaffold Vite React project**

```bash
cd web && npm create vite@latest . -- --template react-ts
```

- [ ] **Step 6: Install frontend dependencies**

```bash
cd web && npm install react-router-dom @tanstack/react-query lucide-react && \
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 7: Configure Tailwind CSS v4**

Update `web/src/index.css`:

```css
@import "tailwindcss";
```

Update `web/vite.config.ts`:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [ ] **Step 8: Create root Makefile**

Create `Makefile`:

```makefile
.PHONY: dev-hub dev-web build test clean

# Development
dev-hub:
	cd hub && go run ./cmd/aerodocs/ --dev

dev-web:
	cd web && npm run dev

# Build production binary (frontend embedded in Go)
build: build-web build-hub

build-web:
	cd web && npm run build

build-hub: build-web
	cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/

# Test
test: test-hub test-web

test-hub:
	cd hub && go test ./...

test-web:
	cd web && npm test

clean:
	rm -rf bin/ web/dist/ hub/aerodocs
```

- [ ] **Step 9: Update .gitignore**

Append to `.gitignore`:

```
# Go
hub/aerodocs
agent/aerodocs-agent
bin/

# Node
web/node_modules/
web/dist/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store
```

- [ ] **Step 10: Create proto placeholder**

```bash
mkdir -p proto/aerodocs/v1 && touch proto/aerodocs/v1/.gitkeep
```

- [ ] **Step 11: Verify everything builds**

```bash
make build-hub 2>&1 | tail -5  # Hub should compile (no web dist yet, that's ok)
cd web && npm run build         # Vite should build
```

- [ ] **Step 12: Commit**

```bash
git add hub/ agent/ web/ proto/ Makefile .gitignore
git commit -m "feat: scaffold monorepo with Go hub, agent, and Vite React frontend"
```

---

## Task 2: SQLite Schema & Migration Runner

**Files:**
- Create: `hub/internal/migrate/migrations/001_create_users.sql`
- Create: `hub/internal/migrate/migrations/002_create_audit_logs.sql`
- Create: `hub/internal/migrate/migrations/003_create_config.sql`
- Create: `hub/internal/migrate/migrate.go`
- Test: `hub/internal/migrate/migrate_test.go`

- [ ] **Step 1: Create migration SQL files**

Create `hub/internal/migrate/migrations/001_create_users.sql`:

```sql
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('admin', 'viewer')),
    totp_secret   TEXT,
    totp_enabled  INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Create `hub/internal/migrate/migrations/002_create_audit_logs.sql`:

```sql
CREATE TABLE audit_logs (
    id         TEXT PRIMARY KEY,
    user_id    TEXT,
    action     TEXT NOT NULL,
    target     TEXT,
    detail     TEXT,
    ip_address TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

Create `hub/internal/migrate/migrations/003_create_config.sql`:

```sql
CREATE TABLE _config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
```

- [ ] **Step 2: Write failing test for migration runner**

Create `hub/internal/migrate/migrate_test.go`:

```go
package migrate_test

import (
	"database/sql"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/migrate"
	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRunMigrations(t *testing.T) {
	db := testDB(t)

	if err := migrate.Run(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	// Verify users table exists
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&name)
	if err != nil {
		t.Fatalf("users table not found: %v", err)
	}

	// Verify audit_logs table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='audit_logs'").Scan(&name)
	if err != nil {
		t.Fatalf("audit_logs table not found: %v", err)
	}

	// Verify _config table exists
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='_config'").Scan(&name)
	if err != nil {
		t.Fatalf("_config table not found: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := testDB(t)

	if err := migrate.Run(db); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := migrate.Run(db); err != nil {
		t.Fatalf("second run should be idempotent: %v", err)
	}

	// Verify only 3 migrations recorded
	var count int
	db.QueryRow("SELECT COUNT(*) FROM _migrations").Scan(&count)
	if count != 3 {
		t.Fatalf("expected 3 migrations, got %d", count)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd hub && go test ./internal/migrate/ -v
```

Expected: FAIL — `migrate` package does not exist yet.

- [ ] **Step 4: Install SQLite dependency**

```bash
cd hub && go get modernc.org/sqlite
```

- [ ] **Step 5: Implement migration runner**

Create `hub/internal/migrate/migrate.go`:

```go
package migrate

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

func Run(db *sql.DB) error {
	// Bootstrap _migrations table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS _migrations (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		filename   TEXT NOT NULL UNIQUE,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create _migrations table: %w", err)
	}

	// Read applied migrations
	applied := make(map[string]bool)
	rows, err := db.Query("SELECT filename FROM _migrations")
	if err != nil {
		return fmt.Errorf("query _migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return fmt.Errorf("scan migration: %w", err)
		}
		applied[f] = true
	}

	// Read migration files
	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort by filename (001_, 002_, etc.)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	// Apply new migrations in a transaction
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") || applied[entry.Name()] {
			continue
		}

		content, err := fs.ReadFile(migrationFS, "migrations/"+entry.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", entry.Name(), err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute %s: %w", entry.Name(), err)
		}

		if _, err := tx.Exec("INSERT INTO _migrations (filename) VALUES (?)", entry.Name()); err != nil {
			tx.Rollback()
			return fmt.Errorf("record %s: %w", entry.Name(), err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", entry.Name(), err)
		}
	}

	return nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd hub && go test ./internal/migrate/ -v
```

Expected: PASS — both `TestRunMigrations` and `TestRunMigrations_Idempotent`.

- [ ] **Step 7: Commit**

```bash
git add hub/internal/migrate/
git commit -m "feat: add SQLite migration runner with users, audit_logs, and config schemas"
```

---

## Task 3: Domain Models

**Files:**
- Create: `hub/internal/model/user.go`
- Create: `hub/internal/model/audit.go`

- [ ] **Step 1: Create user model types**

Create `hub/internal/model/user.go`:

```go
package model

import "time"

type Role string

const (
	RoleAdmin  Role = "admin"
	RoleViewer Role = "viewer"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	TOTPSecret   *string   `json:"-"`
	TOTPEnabled  bool      `json:"totp_enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     Role   `json:"role"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginTOTPRequest struct {
	TOTPToken string `json:"totp_token"`
	Code      string `json:"code"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type TOTPEnableRequest struct {
	Code string `json:"code"`
}

type TOTPDisableRequest struct {
	UserID        string `json:"user_id"`
	AdminTOTPCode string `json:"admin_totp_code"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthStatusResponse struct {
	Initialized bool `json:"initialized"`
}

type LoginResponse struct {
	TOTPToken         string `json:"totp_token,omitempty"`
	SetupToken        string `json:"setup_token,omitempty"`
	RequiresTOTPSetup bool   `json:"requires_totp_setup,omitempty"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	User         User   `json:"user"`
}

type TOTPSetupResponse struct {
	Secret string `json:"secret"`
	QRURL  string `json:"qr_url"`
}

type CreateUserResponse struct {
	User              User   `json:"user"`
	TemporaryPassword string `json:"temporary_password"`
}
```

- [ ] **Step 2: Create audit model types**

Create `hub/internal/model/audit.go`:

```go
package model

import "time"

type AuditEntry struct {
	ID        string    `json:"id"`
	UserID    *string   `json:"user_id"`
	Action    string    `json:"action"`
	Target    *string   `json:"target"`
	Detail    *string   `json:"detail"`
	IPAddress *string   `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditFilter struct {
	UserID *string
	Action *string
	Limit  int
	Offset int
}

// Audit action constants
const (
	AuditUserLogin          = "user.login"
	AuditUserLoginFailed    = "user.login_failed"
	AuditUserLoginTOTPFailed = "user.login_totp_failed"
	AuditUserRegistered     = "user.registered"
	AuditUserTOTPSetup      = "user.totp_setup"
	AuditUserTOTPEnabled    = "user.totp_enabled"
	AuditUserTOTPDisabled   = "user.totp_disabled"
	AuditUserCreated        = "user.created"
	AuditUserTOTPReset      = "user.totp_reset"
)
```

- [ ] **Step 3: Verify compilation**

```bash
cd hub && go build ./internal/model/
```

Expected: compiles without errors.

- [ ] **Step 4: Commit**

```bash
git add hub/internal/model/
git commit -m "feat: add domain model types for users, auth requests, and audit entries"
```

---

## Task 4: Store — Config, Users, Audit

**Files:**
- Create: `hub/internal/store/store.go`
- Create: `hub/internal/store/config.go`
- Create: `hub/internal/store/users.go`
- Create: `hub/internal/store/audit.go`
- Test: `hub/internal/store/store_test.go`
- Test: `hub/internal/store/users_test.go`
- Test: `hub/internal/store/audit_test.go`

- [ ] **Step 1: Create store constructor**

Create `hub/internal/store/store.go`:

```go
package store

import (
	"database/sql"
	"fmt"

	"github.com/wyiu/aerodocs/hub/internal/migrate"
	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode and foreign keys
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	// Run migrations
	if err := migrate.Run(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}
```

- [ ] **Step 2: Write failing test for config store**

Create `hub/internal/store/store_test.go`:

```go
package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
```

Create `hub/internal/store/config_test.go`:

```go
package store_test

import "testing"

func TestConfigGetSet(t *testing.T) {
	s := testStore(t)

	// Set a value
	if err := s.SetConfig("jwt_key", "test-secret"); err != nil {
		t.Fatalf("set config: %v", err)
	}

	// Get the value
	val, err := s.GetConfig("jwt_key")
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if val != "test-secret" {
		t.Fatalf("expected 'test-secret', got '%s'", val)
	}
}

func TestConfigGetMissing(t *testing.T) {
	s := testStore(t)

	_, err := s.GetConfig("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd hub && go test ./internal/store/ -v -run TestConfig
```

Expected: FAIL — `SetConfig` and `GetConfig` methods don't exist.

- [ ] **Step 4: Implement config store**

Create `hub/internal/store/config.go`:

```go
package store

import (
	"database/sql"
	"fmt"
)

func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM _config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("config key %q not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("get config %q: %w", key, err)
	}
	return value, nil
}

func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(
		"INSERT INTO _config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value",
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set config %q: %w", key, err)
	}
	return nil
}
```

- [ ] **Step 5: Run config tests**

```bash
cd hub && go test ./internal/store/ -v -run TestConfig
```

Expected: PASS.

- [ ] **Step 6: Write failing tests for user store**

Create `hub/internal/store/users_test.go`:

```go
package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestCreateAndGetUser(t *testing.T) {
	s := testStore(t)

	user := &model.User{
		ID:           "test-uuid-1",
		Username:     "admin",
		Email:        "admin@test.com",
		PasswordHash: "$2a$12$fakehash",
		Role:         model.RoleAdmin,
	}

	if err := s.CreateUser(user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	got, err := s.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	if got.ID != "test-uuid-1" {
		t.Fatalf("expected id 'test-uuid-1', got '%s'", got.ID)
	}
	if got.Role != model.RoleAdmin {
		t.Fatalf("expected role 'admin', got '%s'", got.Role)
	}
}

func TestCreateUser_DuplicateUsername(t *testing.T) {
	s := testStore(t)

	user := &model.User{
		ID: "u1", Username: "dup", Email: "a@a.com",
		PasswordHash: "hash", Role: model.RoleViewer,
	}
	if err := s.CreateUser(user); err != nil {
		t.Fatalf("first create: %v", err)
	}

	user2 := &model.User{
		ID: "u2", Username: "dup", Email: "b@b.com",
		PasswordHash: "hash", Role: model.RoleViewer,
	}
	if err := s.CreateUser(user2); err == nil {
		t.Fatal("expected error for duplicate username")
	}
}

func TestUserCount(t *testing.T) {
	s := testStore(t)

	count, err := s.UserCount()
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 users, got %d", count)
	}

	s.CreateUser(&model.User{
		ID: "u1", Username: "a", Email: "a@a.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	count, _ = s.UserCount()
	if count != 1 {
		t.Fatalf("expected 1 user, got %d", count)
	}
}

func TestUpdateUserTOTP(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "a", Email: "a@a.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})

	secret := "JBSWY3DPEHPK3PXP"
	if err := s.UpdateUserTOTP("u1", &secret, true); err != nil {
		t.Fatalf("update totp: %v", err)
	}

	user, _ := s.GetUserByID("u1")
	if !user.TOTPEnabled {
		t.Fatal("expected totp_enabled = true")
	}
	if user.TOTPSecret == nil || *user.TOTPSecret != secret {
		t.Fatal("totp_secret not set correctly")
	}
}

func TestListUsers(t *testing.T) {
	s := testStore(t)

	s.CreateUser(&model.User{
		ID: "u1", Username: "alice", Email: "alice@test.com",
		PasswordHash: "h", Role: model.RoleAdmin,
	})
	s.CreateUser(&model.User{
		ID: "u2", Username: "bob", Email: "bob@test.com",
		PasswordHash: "h", Role: model.RoleViewer,
	})

	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}
```

- [ ] **Step 7: Run user tests to verify they fail**

```bash
cd hub && go test ./internal/store/ -v -run TestCreateAndGetUser
```

Expected: FAIL — `CreateUser`, `GetUserByUsername` methods don't exist.

- [ ] **Step 8: Implement user store**

Create `hub/internal/store/users.go`:

```go
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) CreateUser(u *model.User) error {
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, email, password_hash, role, totp_secret, totp_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Email, u.PasswordHash, u.Role, u.TOTPSecret, u.TOTPEnabled,
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *Store) GetUserByID(id string) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	))
}

func (s *Store) GetUserByUsername(username string) (*model.User, error) {
	return s.scanUser(s.db.QueryRow(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	))
}

func (s *Store) UserCount() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (s *Store) UpdateUserTOTP(userID string, secret *string, enabled bool) error {
	_, err := s.db.Exec(
		"UPDATE users SET totp_secret = ?, totp_enabled = ?, updated_at = datetime('now') WHERE id = ?",
		secret, enabled, userID,
	)
	if err != nil {
		return fmt.Errorf("update totp: %w", err)
	}
	return nil
}

func (s *Store) UpdateUserPassword(userID, passwordHash string) error {
	_, err := s.db.Exec(
		"UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ?",
		passwordHash, userID,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	return nil
}

func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(
		`SELECT id, username, email, password_hash, role, totp_secret, totp_enabled, created_at, updated_at
		 FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := s.scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

func (s *Store) scanUser(row *sql.Row) (*model.User, error) {
	var u model.User
	var createdAt, updatedAt string
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.TOTPSecret, &u.TOTPEnabled, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &u, nil
}

func (s *Store) scanUserRow(rows *sql.Rows) (*model.User, error) {
	var u model.User
	var createdAt, updatedAt string
	err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Role,
		&u.TOTPSecret, &u.TOTPEnabled, &createdAt, &updatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan user row: %w", err)
	}
	u.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	u.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &u, nil
}
```

- [ ] **Step 9: Run all user tests**

```bash
cd hub && go test ./internal/store/ -v -run TestUser -run TestCreate -run TestList
```

Expected: all PASS.

- [ ] **Step 10: Write failing test for audit store**

Create `hub/internal/store/audit_test.go`:

```go
package store_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func TestLogAndListAudit(t *testing.T) {
	s := testStore(t)

	userID := "user-1"
	ip := "127.0.0.1"
	if err := s.LogAudit(model.AuditEntry{
		ID:        "a1",
		UserID:    &userID,
		Action:    model.AuditUserLogin,
		IPAddress: &ip,
	}); err != nil {
		t.Fatalf("log audit: %v", err)
	}

	entries, total, err := s.ListAuditLogs(model.AuditFilter{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Action != model.AuditUserLogin {
		t.Fatalf("expected action %q, got %q", model.AuditUserLogin, entries[0].Action)
	}
}

func TestListAuditWithFilter(t *testing.T) {
	s := testStore(t)

	s.LogAudit(model.AuditEntry{ID: "a1", Action: model.AuditUserLogin})
	s.LogAudit(model.AuditEntry{ID: "a2", Action: model.AuditUserLoginFailed})
	s.LogAudit(model.AuditEntry{ID: "a3", Action: model.AuditUserLogin})

	action := model.AuditUserLogin
	entries, total, err := s.ListAuditLogs(model.AuditFilter{
		Action: &action,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total 2, got %d", total)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}
```

- [ ] **Step 11: Run audit tests to verify they fail**

```bash
cd hub && go test ./internal/store/ -v -run TestLog
```

Expected: FAIL — `LogAudit`, `ListAuditLogs` don't exist.

- [ ] **Step 12: Implement audit store**

Create `hub/internal/store/audit.go`:

```go
package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Store) LogAudit(entry model.AuditEntry) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs (id, user_id, action, target, detail, ip_address)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.UserID, entry.Action, entry.Target, entry.Detail, entry.IPAddress,
	)
	if err != nil {
		return fmt.Errorf("log audit: %w", err)
	}
	return nil
}

func (s *Store) ListAuditLogs(filter model.AuditFilter) ([]model.AuditEntry, int, error) {
	var where []string
	var args []interface{}

	if filter.UserID != nil {
		where = append(where, "user_id = ?")
		args = append(args, *filter.UserID)
	}
	if filter.Action != nil {
		where = append(where, "action = ?")
		args = append(args, *filter.Action)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = " WHERE " + strings.Join(where, " AND ")
	}

	// Get total count
	var total int
	countQuery := "SELECT COUNT(*) FROM audit_logs" + whereClause
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Get paginated results
	query := "SELECT id, user_id, action, target, detail, ip_address, created_at FROM audit_logs" +
		whereClause + " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []model.AuditEntry
	for rows.Next() {
		var e model.AuditEntry
		var createdAt string
		if err := rows.Scan(&e.ID, &e.UserID, &e.Action, &e.Target, &e.Detail, &e.IPAddress, &createdAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit entry: %w", err)
		}
		e.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		entries = append(entries, e)
	}

	return entries, total, rows.Err()
}
```

- [ ] **Step 13: Run all store tests**

```bash
cd hub && go test ./internal/store/ -v
```

Expected: all PASS.

- [ ] **Step 14: Commit**

```bash
git add hub/internal/store/
git commit -m "feat: add SQLite store layer with user CRUD, audit logging, and config storage"
```

---

## Task 5: Auth Package — Password

**Files:**
- Create: `hub/internal/auth/password.go`
- Test: `hub/internal/auth/password_test.go`

- [ ] **Step 1: Write failing tests**

Create `hub/internal/auth/password_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestHashAndCompare(t *testing.T) {
	hash, err := auth.HashPassword("MyP@ssw0rd!23")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if !auth.ComparePassword(hash, "MyP@ssw0rd!23") {
		t.Fatal("expected password to match")
	}

	if auth.ComparePassword(hash, "wrong-password") {
		t.Fatal("expected password to NOT match")
	}
}

func TestValidatePasswordPolicy(t *testing.T) {
	tests := []struct {
		name    string
		pw      string
		wantErr bool
	}{
		{"valid", "MyP@ssw0rd!23", false},
		{"too short", "Short!1a", true},
		{"no uppercase", "myp@ssw0rd!23", true},
		{"no lowercase", "MYP@SSW0RD!23", true},
		{"no digit", "MyP@ssword!AB", true},
		{"no special", "MyPassw0rd123", true},
		{"exactly 12 chars", "MyP@ssw0rd!2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidatePasswordPolicy(tt.pw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePasswordPolicy(%q) = %v, wantErr = %v", tt.pw, err, tt.wantErr)
			}
		})
	}
}

func TestGenerateTemporaryPassword(t *testing.T) {
	pw := auth.GenerateTemporaryPassword()

	if len(pw) != 20 {
		t.Fatalf("expected 20 chars, got %d", len(pw))
	}

	if err := auth.ValidatePasswordPolicy(pw); err != nil {
		t.Fatalf("generated password should pass policy: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd hub && go get golang.org/x/crypto/bcrypt && go test ./internal/auth/ -v
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement password module**

Create `hub/internal/auth/password.go`:

```go
package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func ComparePassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func ValidatePasswordPolicy(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("password must include at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must include at least one lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("password must include at least one digit")
	}
	if !hasSpecial {
		return fmt.Errorf("password must include at least one special character")
	}

	return nil
}

func GenerateTemporaryPassword() string {
	const (
		length  = 20
		upper   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lower   = "abcdefghijklmnopqrstuvwxyz"
		digits  = "0123456789"
		special = "!@#$%^&*"
		all     = upper + lower + digits + special
	)

	// Guarantee at least one of each required type
	pw := make([]byte, length)
	pw[0] = randChar(upper)
	pw[1] = randChar(lower)
	pw[2] = randChar(digits)
	pw[3] = randChar(special)

	for i := 4; i < length; i++ {
		pw[i] = randChar(all)
	}

	// Shuffle
	for i := length - 1; i > 0; i-- {
		j := randInt(i + 1)
		pw[i], pw[j] = pw[j], pw[i]
	}

	return string(pw)
}

func randChar(charset string) byte {
	n := randInt(len(charset))
	return charset[n]
}

func randInt(max int) int {
	n, _ := rand.Int(rand.Reader, big.NewInt(int64(max)))
	return int(n.Int64())
}
```

- [ ] **Step 4: Run tests**

```bash
cd hub && go test ./internal/auth/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add hub/internal/auth/password.go hub/internal/auth/password_test.go
git commit -m "feat: add password hashing, validation, and temporary password generation"
```

---

## Task 6: Auth Package — JWT

**Files:**
- Create: `hub/internal/auth/jwt.go`
- Test: `hub/internal/auth/jwt_test.go`

- [ ] **Step 1: Write failing tests**

Create `hub/internal/auth/jwt_test.go`:

```go
package auth_test

import (
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestGenerateAndValidateAccessToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	access, _, err := auth.GenerateTokenPair(secret, "user-1", "admin")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	claims, err := auth.ValidateToken(secret, access)
	if err != nil {
		t.Fatalf("validate access: %v", err)
	}

	if claims.Subject != "user-1" {
		t.Fatalf("expected sub 'user-1', got '%s'", claims.Subject)
	}
	if claims.Role != "admin" {
		t.Fatalf("expected role 'admin', got '%s'", claims.Role)
	}
	if claims.TokenType != auth.TokenTypeAccess {
		t.Fatalf("expected type 'access', got '%s'", claims.TokenType)
	}
}

func TestValidateRefreshToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	_, refresh, _ := auth.GenerateTokenPair(secret, "user-1", "admin")

	claims, err := auth.ValidateToken(secret, refresh)
	if err != nil {
		t.Fatalf("validate refresh: %v", err)
	}
	if claims.TokenType != auth.TokenTypeRefresh {
		t.Fatalf("expected type 'refresh', got '%s'", claims.TokenType)
	}
}

func TestGenerateSetupToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	token, err := auth.GenerateSetupToken(secret, "user-1", "admin")
	if err != nil {
		t.Fatalf("generate setup: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate setup: %v", err)
	}
	if claims.TokenType != auth.TokenTypeSetup {
		t.Fatalf("expected type 'setup', got '%s'", claims.TokenType)
	}
}

func TestGenerateTOTPToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	token, err := auth.GenerateTOTPToken(secret, "user-1", "admin")
	if err != nil {
		t.Fatalf("generate totp: %v", err)
	}

	claims, err := auth.ValidateToken(secret, token)
	if err != nil {
		t.Fatalf("validate totp: %v", err)
	}
	if claims.TokenType != auth.TokenTypeTOTP {
		t.Fatalf("expected type 'totp', got '%s'", claims.TokenType)
	}
}

func TestExpiredToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"

	// Create a token that's already expired
	token, _ := auth.GenerateTokenWithExpiry(secret, "user-1", "admin", auth.TokenTypeAccess, -1*time.Minute)

	_, err := auth.ValidateToken(secret, token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestWrongSecret(t *testing.T) {
	access, _, _ := auth.GenerateTokenPair("correct-secret-key-256-bits!!!!", "user-1", "admin")

	_, err := auth.ValidateToken("wrong-secret-key-256-bits-long!", access)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd hub && go get github.com/golang-jwt/jwt/v5 && go test ./internal/auth/ -v -run TestGenerate -run TestExpired -run TestWrong
```

Expected: FAIL — JWT functions don't exist.

- [ ] **Step 3: Implement JWT module**

Create `hub/internal/auth/jwt.go`:

```go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	TokenTypeSetup   = "setup"
	TokenTypeTOTP    = "totp"

	AccessTokenExpiry  = 15 * time.Minute
	RefreshTokenExpiry = 7 * 24 * time.Hour
	SetupTokenExpiry   = 10 * time.Minute
	TOTPTokenExpiry    = 60 * time.Second
)

type Claims struct {
	jwt.RegisteredClaims
	Role      string `json:"role"`
	TokenType string `json:"type"`
}

// No local TokenPair type — use model.TokenPair for JSON serialization.
// GenerateTokenPair returns raw strings to avoid a duplicate type.

func GenerateTokenPair(secret, userID, role string) (accessToken, refreshToken string, err error) {
	accessToken, err = GenerateTokenWithExpiry(secret, userID, role, TokenTypeAccess, AccessTokenExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err = GenerateTokenWithExpiry(secret, userID, role, TokenTypeRefresh, RefreshTokenExpiry)
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

func GenerateSetupToken(secret, userID, role string) (string, error) {
	return GenerateTokenWithExpiry(secret, userID, role, TokenTypeSetup, SetupTokenExpiry)
}

func GenerateTOTPToken(secret, userID, role string) (string, error) {
	return GenerateTokenWithExpiry(secret, userID, role, TokenTypeTOTP, TOTPTokenExpiry)
}

func GenerateTokenWithExpiry(secret, userID, role, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
		},
		Role:      role,
		TokenType: tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

func ValidateToken(secret, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd hub && go test ./internal/auth/ -v -run "TestGenerate|TestExpired|TestWrong|TestValidate"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add hub/internal/auth/jwt.go hub/internal/auth/jwt_test.go
git commit -m "feat: add JWT token generation and validation with four token types"
```

---

## Task 7: Auth Package — TOTP

**Files:**
- Create: `hub/internal/auth/totp.go`
- Test: `hub/internal/auth/totp_test.go`

- [ ] **Step 1: Write failing tests**

Create `hub/internal/auth/totp_test.go`:

```go
package auth_test

import (
	"testing"

	"github.com/pquerna/otp/totp"
	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestGenerateTOTPSecret(t *testing.T) {
	key, err := auth.GenerateTOTPSecret("admin", "AeroDocs")
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}

	if key.Secret() == "" {
		t.Fatal("secret should not be empty")
	}
	if key.URL() == "" {
		t.Fatal("URL should not be empty")
	}
}

func TestValidateTOTPCode(t *testing.T) {
	key, _ := auth.GenerateTOTPSecret("admin", "AeroDocs")

	// Generate a valid code
	code, err := totp.GenerateCode(key.Secret(), auth.TOTPTime())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	if !auth.ValidateTOTPCode(key.Secret(), code) {
		t.Fatal("valid code should pass validation")
	}

	if auth.ValidateTOTPCode(key.Secret(), "000000") {
		t.Fatal("invalid code should fail validation")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd hub && go get github.com/pquerna/otp && go test ./internal/auth/ -v -run TestGenerateTOTP -run TestValidateTOTP
```

Expected: FAIL.

- [ ] **Step 3: Implement TOTP module**

Create `hub/internal/auth/totp.go`:

```go
package auth

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func GenerateTOTPSecret(username, issuer string) (*otp.Key, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: username,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP secret: %w", err)
	}
	return key, nil
}

func ValidateTOTPCode(secret, code string) bool {
	return totp.Validate(code, secret)
}

// GenerateValidCode produces a valid TOTP code for the given secret.
// Used in tests to simulate authenticator app input.
func GenerateValidCode(secret string) (string, error) {
	return totp.GenerateCode(secret, time.Now())
}

func TOTPTime() time.Time {
	return time.Now()
}
```

- [ ] **Step 4: Run tests**

```bash
cd hub && go test ./internal/auth/ -v -run "TOTP"
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add hub/internal/auth/totp.go hub/internal/auth/totp_test.go
git commit -m "feat: add TOTP secret generation and code validation"
```

---

## Task 8: HTTP Server, Middleware & Response Helpers

**Files:**
- Create: `hub/internal/server/server.go`
- Create: `hub/internal/server/router.go`
- Create: `hub/internal/server/middleware.go`
- Create: `hub/internal/server/respond.go`
- Test: `hub/internal/server/middleware_test.go`

- [ ] **Step 1: Create JSON response helpers**

Create `hub/internal/server/respond.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
)

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}
```

- [ ] **Step 2: Create server struct**

Create `hub/internal/server/server.go`:

```go
package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

type Server struct {
	httpServer *http.Server
	store      *store.Store
	jwtSecret  string
	isDev      bool
}

type Config struct {
	Addr      string
	Store     *store.Store
	JWTSecret string
	IsDev     bool
}

func New(cfg Config) *Server {
	s := &Server{
		store:     cfg.Store,
		jwtSecret: cfg.JWTSecret,
		isDev:     cfg.IsDev,
	}

	mux := s.routes()

	s.httpServer = &http.Server{
		Addr:         cfg.Addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	fmt.Printf("AeroDocs Hub listening on %s\n", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// initJWTSecret generates a random JWT signing key on first run,
// or retrieves the existing one from the database.
func InitJWTSecret(st *store.Store) (string, error) {
	secret, err := st.GetConfig("jwt_signing_key")
	if err == nil {
		return secret, nil
	}

	// Generate new 256-bit key
	secret = auth.GenerateTemporaryPassword() + auth.GenerateTemporaryPassword()
	if err := st.SetConfig("jwt_signing_key", secret); err != nil {
		return "", fmt.Errorf("store jwt key: %w", err)
	}

	return secret, nil
}
```

- [ ] **Step 3: Create middleware**

Create `hub/internal/server/middleware.go`:

```go
package server

import (
	"context"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

type contextKey string

const (
	ctxUserID    contextKey = "user_id"
	ctxUserRole  contextKey = "user_role"
	ctxTokenType contextKey = "token_type"
)

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserID).(string)
	return v
}

func UserRoleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserRole).(string)
	return v
}

func TokenTypeFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxTokenType).(string)
	return v
}

// authMiddleware validates JWT from Authorization header and enforces token type.
func (s *Server) authMiddleware(requiredType string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}

		if tokenStr == "" {
			respondError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		claims, err := auth.ValidateToken(s.jwtSecret, tokenStr)
		if err != nil {
			respondError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		if claims.TokenType != requiredType {
			respondError(w, http.StatusForbidden, "invalid token type for this endpoint")
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserID, claims.Subject)
		ctx = context.WithValue(ctx, ctxUserRole, claims.Role)
		ctx = context.WithValue(ctx, ctxTokenType, claims.TokenType)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminOnly wraps a handler to require admin role.
func (s *Server) adminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserRoleFromContext(r.Context()) != "admin" {
			respondError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs each request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// corsMiddleware adds CORS headers for development.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.isDev {
			w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// rateLimiter tracks login attempts per IP.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Filter expired entries
	valid := rl.attempts[ip][:0]
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	if len(valid) >= rl.limit {
		return false
	}

	rl.attempts[ip] = append(rl.attempts[ip], now)
	return true
}

func (rl *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}

		if !rl.allow(strings.TrimSpace(ip)) {
			w.Header().Set("Retry-After", "60")
			respondError(w, http.StatusTooManyRequests, "too many login attempts")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the client IP from the request.
func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	return r.RemoteAddr
}
```

- [ ] **Step 4: Write failing middleware tests**

Create `hub/internal/server/middleware_test.go`:

```go
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	access, _, _ := auth.GenerateTokenPair(secret, "user-1", "admin")

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid := UserIDFromContext(r.Context())
		if uid != "user-1" {
			t.Fatalf("expected user-1, got %s", uid)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+access)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingToken(t *testing.T) {
	s := &Server{jwtSecret: "secret"}

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_WrongTokenType(t *testing.T) {
	secret := "test-secret-key-256-bits-long!!!"
	s := &Server{jwtSecret: secret}

	// Generate a setup token, try to use it on an access-required endpoint
	setupToken, _ := auth.GenerateSetupToken(secret, "user-1", "admin")

	handler := s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+setupToken)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}

	if rl.allow("1.2.3.4") {
		t.Fatal("4th attempt should be blocked")
	}

	// Different IP should still be allowed
	if !rl.allow("5.6.7.8") {
		t.Fatal("different IP should be allowed")
	}
}
```

- [ ] **Step 5: Run middleware tests**

```bash
cd hub && go test ./internal/server/ -v -run "TestAuth|TestRate"
```

Expected: all PASS (we wrote both test and implementation together since middleware is tightly coupled).

- [ ] **Step 6: Commit**

```bash
git add hub/internal/server/server.go hub/internal/server/middleware.go hub/internal/server/middleware_test.go hub/internal/server/respond.go
git commit -m "feat: add HTTP server, auth/CORS/rate-limit middleware, and response helpers"
```

---

## Task 9: Route Registration & Auth Handlers

**Files:**
- Create: `hub/internal/server/router.go`
- Create: `hub/internal/server/handlers_auth.go`
- Test: `hub/internal/server/handlers_auth_test.go`

- [ ] **Step 1: Create route registration**

Create `hub/internal/server/router.go`:

```go
package server

import (
	"net/http"
	"time"

	"github.com/wyiu/aerodocs/hub/internal/auth"
)

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	loginLimiter := newRateLimiter(5, 60*time.Second)

	// Public auth endpoints (rate-limited)
	mux.Handle("GET /api/auth/status", loggingMiddleware(http.HandlerFunc(s.handleAuthStatus)))
	mux.Handle("POST /api/auth/register", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleRegister))))
	mux.Handle("POST /api/auth/login", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleLogin))))
	mux.Handle("POST /api/auth/login/totp", loggingMiddleware(loginLimiter.middleware(http.HandlerFunc(s.handleLoginTOTP))))

	// Refresh endpoint (token in body)
	mux.Handle("POST /api/auth/refresh", loggingMiddleware(http.HandlerFunc(s.handleRefresh)))

	// Setup-token-protected endpoints
	mux.Handle("POST /api/auth/totp/setup", loggingMiddleware(s.authMiddleware(auth.TokenTypeSetup, http.HandlerFunc(s.handleTOTPSetup))))
	mux.Handle("POST /api/auth/totp/enable", loggingMiddleware(s.authMiddleware(auth.TokenTypeSetup, http.HandlerFunc(s.handleTOTPEnable))))

	// Access-token-protected endpoints
	mux.Handle("GET /api/auth/me", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleMe))))
	mux.Handle("POST /api/auth/totp/disable", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleTOTPDisable)))))

	// Admin endpoints
	mux.Handle("GET /api/users", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListUsers)))))
	mux.Handle("POST /api/users", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleCreateUser)))))
	mux.Handle("GET /api/audit-logs", loggingMiddleware(s.authMiddleware(auth.TokenTypeAccess, s.adminOnly(http.HandlerFunc(s.handleListAuditLogs)))))

	// Apply CORS globally
	return s.corsMiddleware(mux)
}
```

- [ ] **Step 2: Create auth handlers**

Create `hub/internal/server/handlers_auth.go`:

```go
package server

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.UserCount()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check user count")
		return
	}
	respondJSON(w, http.StatusOK, model.AuthStatusResponse{Initialized: count > 0})
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	// Only allowed when no users exist
	count, err := s.store.UserCount()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to check user count")
		return
	}
	if count > 0 {
		respondError(w, http.StatusForbidden, "registration disabled — use admin to create users")
		return
	}

	var req model.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.ValidatePasswordPolicy(req.Password); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         model.RoleAdmin,
	}

	if err := s.store.CreateUser(user); err != nil {
		respondError(w, http.StatusConflict, "user already exists")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &user.ID,
		Action: model.AuditUserRegistered, IPAddress: &ip,
	})

	setupToken, err := auth.GenerateSetupToken(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"setup_token": setupToken,
		"user":        user,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req model.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.store.GetUserByUsername(req.Username)
	if err != nil {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), Action: model.AuditUserLoginFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.ComparePassword(user.PasswordHash, req.Password) {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), UserID: &user.ID,
			Action: model.AuditUserLoginFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// If TOTP not set up yet, return setup token
	if !user.TOTPEnabled {
		setupToken, err := auth.GenerateSetupToken(s.jwtSecret, user.ID, string(user.Role))
		if err != nil {
			respondError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		respondJSON(w, http.StatusOK, model.LoginResponse{
			SetupToken:        setupToken,
			RequiresTOTPSetup: true,
		})
		return
	}

	// TOTP is enabled — require TOTP code
	totpToken, err := auth.GenerateTOTPToken(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondJSON(w, http.StatusAccepted, model.LoginResponse{
		TOTPToken: totpToken,
	})
}

func (s *Server) handleLoginTOTP(w http.ResponseWriter, r *http.Request) {
	var req model.LoginTOTPRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate the TOTP token (proves password was already verified)
	claims, err := auth.ValidateToken(s.jwtSecret, req.TOTPToken)
	if err != nil || claims.TokenType != auth.TokenTypeTOTP {
		respondError(w, http.StatusUnauthorized, "invalid or expired TOTP token")
		return
	}

	user, err := s.store.GetUserByID(claims.Subject)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "user not found")
		return
	}

	if user.TOTPSecret == nil || !auth.ValidateTOTPCode(*user.TOTPSecret, req.Code) {
		ip := clientIP(r)
		s.store.LogAudit(model.AuditEntry{
			ID: uuid.NewString(), UserID: &user.ID,
			Action: model.AuditUserLoginTOTPFailed, IPAddress: &ip,
		})
		respondError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &user.ID,
		Action: model.AuditUserLogin, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req model.RefreshRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	claims, err := auth.ValidateToken(s.jwtSecret, req.RefreshToken)
	if err != nil || claims.TokenType != auth.TokenTypeRefresh {
		respondError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, claims.Subject, claims.Role)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	respondJSON(w, http.StatusOK, model.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}
	respondJSON(w, http.StatusOK, user)
}

func (s *Server) handleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	key, err := auth.GenerateTOTPSecret(user.Username, "AeroDocs")
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate TOTP secret")
		return
	}

	// Store secret (not yet enabled)
	secret := key.Secret()
	if err := s.store.UpdateUserTOTP(userID, &secret, false); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to store TOTP secret")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &userID,
		Action: model.AuditUserTOTPSetup, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, model.TOTPSetupResponse{
		Secret: key.Secret(),
		QRURL:  key.URL(),
	})
}

func (s *Server) handleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	var req model.TOTPEnableRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	if user.TOTPSecret == nil {
		respondError(w, http.StatusBadRequest, "TOTP not set up — call /api/auth/totp/setup first")
		return
	}

	if !auth.ValidateTOTPCode(*user.TOTPSecret, req.Code) {
		respondError(w, http.StatusUnauthorized, "invalid TOTP code")
		return
	}

	if err := s.store.UpdateUserTOTP(userID, user.TOTPSecret, true); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to enable TOTP")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &userID,
		Action: model.AuditUserTOTPEnabled, IPAddress: &ip,
	})

	// Generate full access tokens now that 2FA is enabled
	accessToken, refreshToken, err := auth.GenerateTokenPair(s.jwtSecret, user.ID, string(user.Role))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to generate tokens")
		return
	}

	// Refresh user to get updated totp_enabled
	user, _ = s.store.GetUserByID(userID)

	respondJSON(w, http.StatusOK, model.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         *user,
	})
}

func (s *Server) handleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	var req model.TOTPDisableRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify admin's own TOTP code
	adminID := UserIDFromContext(r.Context())
	admin, err := s.store.GetUserByID(adminID)
	if err != nil {
		respondError(w, http.StatusNotFound, "admin user not found")
		return
	}

	if admin.TOTPSecret == nil || !auth.ValidateTOTPCode(*admin.TOTPSecret, req.AdminTOTPCode) {
		respondError(w, http.StatusUnauthorized, "invalid admin TOTP code")
		return
	}

	// Disable target user's TOTP
	if err := s.store.UpdateUserTOTP(req.UserID, nil, false); err != nil {
		respondError(w, http.StatusInternalServerError, "failed to disable TOTP")
		return
	}

	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserTOTPDisabled, Target: &req.UserID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusOK, map[string]string{"status": "totp disabled"})
}

func validateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username must be 3-32 characters")
	}
	for _, r := range username {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return fmt.Errorf("username may only contain alphanumeric characters and underscores")
		}
	}
	return nil
}
```

- [ ] **Step 3: Write handler integration tests**

Create `hub/internal/server/handlers_auth_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/model"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	jwtSecret, err := InitJWTSecret(st)
	if err != nil {
		t.Fatalf("init jwt secret: %v", err)
	}

	return New(Config{
		Addr:      ":0",
		Store:     st,
		JWTSecret: jwtSecret,
		IsDev:     true,
	})
}

func TestAuthStatus_NotInitialized(t *testing.T) {
	s := testServer(t)

	req := httptest.NewRequest("GET", "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.AuthStatusResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Initialized {
		t.Fatal("expected initialized=false")
	}
}

func TestRegisterFirstUser(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin",
		Email:    "admin@test.com",
		Password: "MyP@ssw0rd!234",
	})

	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp["setup_token"] == nil {
		t.Fatal("expected setup_token in response")
	}
}

func TestRegisterBlocked_AfterFirstUser(t *testing.T) {
	s := testServer(t)

	// Register first user
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	// Try to register again
	body2, _ := json.Marshal(model.RegisterRequest{
		Username: "hacker", Email: "hacker@test.com", Password: "MyP@ssw0rd!234",
	})
	req2 := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body2))
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec2.Code)
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	s := testServer(t)

	body, _ := json.Marshal(model.LoginRequest{
		Username: "nobody", Password: "wrong",
	})
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
```

- [ ] **Step 4: Run handler tests**

```bash
cd hub && go get github.com/google/uuid && go test ./internal/server/ -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/router.go hub/internal/server/handlers_auth.go hub/internal/server/handlers_auth_test.go
git commit -m "feat: add auth route handlers — register, login, TOTP setup/verify, refresh, me"
```

---

## Task 10: User Management & Audit Log Handlers

**Files:**
- Create: `hub/internal/server/handlers_users.go`
- Create: `hub/internal/server/handlers_audit.go`
- Test: `hub/internal/server/handlers_users_test.go`

- [ ] **Step 1: Implement user management handlers**

Create `hub/internal/server/handlers_users.go`:

```go
package server

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list users")
		return
	}
	respondJSON(w, http.StatusOK, users)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req model.CreateUserRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateUsername(req.Username); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Role != model.RoleAdmin && req.Role != model.RoleViewer {
		respondError(w, http.StatusBadRequest, "role must be 'admin' or 'viewer'")
		return
	}

	tempPassword := auth.GenerateTemporaryPassword()
	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &model.User{
		ID:           uuid.NewString(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         req.Role,
	}

	if err := s.store.CreateUser(user); err != nil {
		respondError(w, http.StatusConflict, "user already exists")
		return
	}

	adminID := UserIDFromContext(r.Context())
	ip := clientIP(r)
	s.store.LogAudit(model.AuditEntry{
		ID: uuid.NewString(), UserID: &adminID,
		Action: model.AuditUserCreated, Target: &user.ID, IPAddress: &ip,
	})

	respondJSON(w, http.StatusCreated, model.CreateUserResponse{
		User:              *user,
		TemporaryPassword: tempPassword,
	})
}
```

- [ ] **Step 2: Implement audit log handler**

Create `hub/internal/server/handlers_audit.go`:

```go
package server

import (
	"net/http"
	"strconv"

	"github.com/wyiu/aerodocs/hub/internal/model"
)

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filter := model.AuditFilter{
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
	if v := q.Get("action"); v != "" {
		filter.Action = &v
	}
	if v := q.Get("user_id"); v != "" {
		filter.UserID = &v
	}

	entries, total, err := s.store.ListAuditLogs(filter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "failed to list audit logs")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"entries": entries,
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
	})
}
```

- [ ] **Step 3: Write tests for create user endpoint**

Add to `hub/internal/server/handlers_users_test.go`:

```go
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/model"
)

func registerAndGetAdminToken(t *testing.T, s *Server) string {
	t.Helper()

	// Register first admin
	body, _ := json.Marshal(model.RegisterRequest{
		Username: "admin", Email: "admin@test.com", Password: "MyP@ssw0rd!234",
	})
	req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	var regResp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&regResp)
	setupToken := regResp["setup_token"].(string)

	// Setup TOTP
	req2 := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	req2.Header.Set("Authorization", "Bearer "+setupToken)
	rec2 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec2, req2)

	var totpResp model.TOTPSetupResponse
	json.NewDecoder(rec2.Body).Decode(&totpResp)

	// Generate valid TOTP code and enable
	code, _ := auth.GenerateValidCode(totpResp.Secret)

	enableBody, _ := json.Marshal(model.TOTPEnableRequest{Code: code})
	req3 := httptest.NewRequest("POST", "/api/auth/totp/enable", bytes.NewReader(enableBody))
	req3.Header.Set("Authorization", "Bearer "+setupToken)
	rec3 := httptest.NewRecorder()
	s.routes().ServeHTTP(rec3, req3)

	var authResp model.AuthResponse
	json.NewDecoder(rec3.Body).Decode(&authResp)

	return authResp.AccessToken
}

func TestCreateUser(t *testing.T) {
	s := testServer(t)
	token := registerAndGetAdminToken(t, s)

	body, _ := json.Marshal(model.CreateUserRequest{
		Username: "viewer1",
		Email:    "viewer@test.com",
		Role:     model.RoleViewer,
	})

	req := httptest.NewRequest("POST", "/api/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.CreateUserResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.TemporaryPassword == "" {
		t.Fatal("expected temporary_password in response")
	}
	if resp.User.Username != "viewer1" {
		t.Fatalf("expected username 'viewer1', got '%s'", resp.User.Username)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
cd hub && go test ./internal/server/ -v -run TestCreateUser
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add hub/internal/server/handlers_users.go hub/internal/server/handlers_audit.go hub/internal/server/handlers_users_test.go hub/internal/auth/totp.go
git commit -m "feat: add user creation, user listing, and audit log endpoints"
```

---

## Task 11: Main Entry Point & CLI Break-Glass

**Files:**
- Modify: `hub/cmd/aerodocs/main.go`
- Create: `hub/cmd/aerodocs/admin.go`
- Create: `hub/embed.go`

- [ ] **Step 1: Update main.go with full server startup**

Update `hub/cmd/aerodocs/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/wyiu/aerodocs/hub/internal/server"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "admin" {
		if err := runAdmin(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runServer(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServer() error {
	addr := flag.String("addr", ":8080", "listen address")
	dbPath := flag.String("db", "aerodocs.db", "SQLite database path")
	dev := flag.Bool("dev", false, "enable development mode (CORS)")
	flag.Parse()

	st, err := store.New(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	jwtSecret, err := server.InitJWTSecret(st)
	if err != nil {
		return fmt.Errorf("init JWT secret: %w", err)
	}

	srv := server.New(server.Config{
		Addr:      *addr,
		Store:     st,
		JWTSecret: jwtSecret,
		IsDev:     *dev,
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		fmt.Println("\nShutting down...")
		srv.Shutdown(context.Background())
	}()

	return srv.Start()
}
```

- [ ] **Step 2: Create CLI break-glass command**

Create `hub/cmd/aerodocs/admin.go`:

```go
package main

import (
	"flag"
	"fmt"

	"github.com/wyiu/aerodocs/hub/internal/auth"
	"github.com/wyiu/aerodocs/hub/internal/store"
)

func runAdmin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aerodocs admin <command>\ncommands: reset-totp")
	}

	switch args[0] {
	case "reset-totp":
		return runResetTOTP(args[1:])
	default:
		return fmt.Errorf("unknown admin command: %s", args[0])
	}
}

func runResetTOTP(args []string) error {
	fs := flag.NewFlagSet("reset-totp", flag.ExitOnError)
	username := fs.String("username", "", "username to reset TOTP for")
	dbPath := fs.String("db", "aerodocs.db", "SQLite database path")
	fs.Parse(args)

	if *username == "" {
		return fmt.Errorf("--username is required")
	}

	st, err := store.New(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer st.Close()

	user, err := st.GetUserByUsername(*username)
	if err != nil {
		return fmt.Errorf("user %q not found", *username)
	}

	// Generate new temporary password
	tempPassword := auth.GenerateTemporaryPassword()
	hash, err := auth.HashPassword(tempPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Reset TOTP and password
	if err := st.UpdateUserTOTP(user.ID, nil, false); err != nil {
		return fmt.Errorf("reset TOTP: %w", err)
	}
	if err := st.UpdateUserPassword(user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	fmt.Printf("TOTP reset for user %q\n", *username)
	fmt.Printf("Temporary password: %s\n", tempPassword)
	fmt.Println("User must set up TOTP again on next login.")

	return nil
}
```

- [ ] **Step 3: Create embed.go placeholder**

Create `hub/embed.go`:

```go
package hub

// This file will embed the frontend dist/ directory once the frontend is built.
// For now, it serves as a placeholder.
//
// In production build:
//   //go:embed web/dist
//   var frontendFS embed.FS
```

- [ ] **Step 4: Verify hub builds and starts**

```bash
cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/ && ../bin/aerodocs --help
```

Expected: binary compiles. Flag usage printed.

- [ ] **Step 5: Verify break-glass command**

```bash
cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/ && ../bin/aerodocs admin reset-totp --help
```

Expected: prints usage for `--username` flag.

- [ ] **Step 6: Commit**

```bash
git add hub/cmd/ hub/embed.go
git commit -m "feat: add server entry point with graceful shutdown and CLI break-glass command"
```

---

## Task 12: Frontend Scaffolding & Design Tokens

**Files:**
- Create: `web/src/styles/tokens.css`
- Modify: `web/src/index.css`
- Modify: `web/vite.config.ts`
- Modify: `web/tsconfig.json`
- Create: `web/src/App.tsx`
- Create: `web/src/main.tsx`

- [ ] **Step 1: Create design tokens CSS**

Create `web/src/styles/tokens.css`:

```css
@theme {
  --color-base: #0a0a0b;
  --color-surface: #111113;
  --color-elevated: #18181b;
  --color-border: #27272a;
  --color-border-subtle: #1e1e21;
  --color-text-primary: #f4f4f5;
  --color-text-secondary: #a1a1aa;
  --color-text-muted: #71717a;
  --color-text-faint: #52525b;
  --color-accent: #3b82f6;
  --color-accent-hover: #2563eb;
  --color-status-online: #22c55e;
  --color-status-warning: #f59e0b;
  --color-status-offline: #ef4444;
  --color-status-error: #ef4444;

  --font-mono: 'JetBrains Mono', 'Fira Code', 'SF Mono', ui-monospace, monospace;
  --font-sans: 'Inter', system-ui, -apple-system, sans-serif;
}
```

- [ ] **Step 2: Update index.css**

Update `web/src/index.css`:

```css
@import "tailwindcss";
@import "./styles/tokens.css";

body {
  margin: 0;
  background-color: var(--color-base);
  color: var(--color-text-primary);
  font-family: var(--font-sans);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

* {
  border-color: var(--color-border);
}
```

- [ ] **Step 3: Configure tsconfig for path aliases**

Update `web/tsconfig.json` to add:

```json
{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"]
    }
  }
}
```

Update `web/vite.config.ts` to add path alias resolution:

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
```

- [ ] **Step 4: Create minimal App.tsx**

Create `web/src/App.tsx`:

```tsx
function App() {
  return (
    <div className="min-h-screen bg-base text-text-primary flex items-center justify-center">
      <h1 className="text-2xl font-bold tracking-wider">AERODOCS</h1>
    </div>
  )
}

export default App
```

- [ ] **Step 5: Update main.tsx**

Update `web/src/main.tsx`:

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 6: Verify frontend builds**

```bash
cd web && npm run build
```

Expected: Vite builds without errors.

- [ ] **Step 7: Commit**

```bash
git add web/
git commit -m "feat: scaffold React frontend with Tailwind CSS v4 and design token system"
```

---

## Task 13: Frontend Types & API Client

**Files:**
- Create: `web/src/types/api.ts`
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/auth.ts`
- Create: `web/src/lib/query-client.ts`

- [ ] **Step 1: Create TypeScript API types**

Create `web/src/types/api.ts`:

```ts
export type Role = 'admin' | 'viewer'

export interface User {
  id: string
  username: string
  email: string
  role: Role
  totp_enabled: boolean
  created_at: string
  updated_at: string
}

export interface AuthStatusResponse {
  initialized: boolean
}

export interface RegisterRequest {
  username: string
  email: string
  password: string
}

export interface LoginRequest {
  username: string
  password: string
}

export interface LoginResponse {
  totp_token?: string
  setup_token?: string
  requires_totp_setup?: boolean
}

export interface LoginTOTPRequest {
  totp_token: string
  code: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: User
}

export interface TokenPair {
  access_token: string
  refresh_token: string
}

export interface TOTPSetupResponse {
  secret: string
  qr_url: string
}

export interface TOTPEnableRequest {
  code: string
}

export interface CreateUserRequest {
  username: string
  email: string
  role: Role
}

export interface CreateUserResponse {
  user: User
  temporary_password: string
}

export interface AuditEntry {
  id: string
  user_id: string | null
  action: string
  target: string | null
  detail: string | null
  ip_address: string | null
  created_at: string
}

export interface AuditLogResponse {
  entries: AuditEntry[]
  total: number
  limit: number
  offset: number
}

export interface ApiError {
  error: string
}
```

- [ ] **Step 2: Create token storage helpers**

Create `web/src/lib/auth.ts`:

```ts
const ACCESS_TOKEN_KEY = 'aerodocs_access_token'
const REFRESH_TOKEN_KEY = 'aerodocs_refresh_token'

export function getAccessToken(): string | null {
  return localStorage.getItem(ACCESS_TOKEN_KEY)
}

export function getRefreshToken(): string | null {
  return localStorage.getItem(REFRESH_TOKEN_KEY)
}

export function setTokens(accessToken: string, refreshToken: string): void {
  localStorage.setItem(ACCESS_TOKEN_KEY, accessToken)
  localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken)
}

export function clearTokens(): void {
  localStorage.removeItem(ACCESS_TOKEN_KEY)
  localStorage.removeItem(REFRESH_TOKEN_KEY)
}
```

- [ ] **Step 3: Create API client with refresh interceptor**

Create `web/src/lib/api.ts`:

```ts
import { getAccessToken, getRefreshToken, setTokens, clearTokens } from './auth'
import type { TokenPair } from '@/types/api'

const BASE_URL = '/api'

let isRefreshing = false
let refreshPromise: Promise<boolean> | null = null

async function refreshTokens(): Promise<boolean> {
  const refreshToken = getRefreshToken()
  if (!refreshToken) return false

  try {
    const res = await fetch(`${BASE_URL}/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ refresh_token: refreshToken }),
    })

    if (!res.ok) return false

    const data: TokenPair = await res.json()
    setTokens(data.access_token, data.refresh_token)
    return true
  } catch {
    return false
  }
}

export async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')

  const accessToken = getAccessToken()
  if (accessToken) {
    headers.set('Authorization', `Bearer ${accessToken}`)
  }

  let res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  // If 401 and we have a refresh token, try refreshing
  if (res.status === 401 && getRefreshToken()) {
    if (!isRefreshing) {
      isRefreshing = true
      refreshPromise = refreshTokens().finally(() => {
        isRefreshing = false
        refreshPromise = null
      })
    }

    const refreshed = await refreshPromise
    if (refreshed) {
      // Retry with new token
      const retryHeaders = new Headers(options.headers)
      retryHeaders.set('Content-Type', 'application/json')
      retryHeaders.set('Authorization', `Bearer ${getAccessToken()}`)
      res = await fetch(`${BASE_URL}${path}`, { ...options, headers: retryHeaders })
    } else {
      clearTokens()
      window.location.href = '/login'
      throw new Error('Session expired')
    }
  }

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(error.error || `HTTP ${res.status}`)
  }

  return res.json()
}

// Convenience for requests that use a specific token (setup/totp)
export async function apiFetchWithToken<T>(
  path: string,
  token: string,
  options: RequestInit = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  headers.set('Authorization', `Bearer ${token}`)

  const res = await fetch(`${BASE_URL}${path}`, { ...options, headers })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: 'Unknown error' }))
    throw new Error(error.error || `HTTP ${res.status}`)
  }

  return res.json()
}
```

- [ ] **Step 4: Create TanStack Query client**

Create `web/src/lib/query-client.ts`:

```ts
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
})
```

- [ ] **Step 5: Verify types compile**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors.

- [ ] **Step 6: Commit**

```bash
git add web/src/types/ web/src/lib/
git commit -m "feat: add TypeScript API types, fetch client with JWT refresh, and query client"
```

---

## Task 14: Frontend Auth Context & Routing

**Files:**
- Create: `web/src/hooks/use-auth.ts`
- Create: `web/src/layouts/auth-layout.tsx`
- Create: `web/src/layouts/app-shell.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Create auth context and hook**

Create `web/src/hooks/use-auth.ts`:

```tsx
import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { apiFetch } from '@/lib/api'
import { getAccessToken, setTokens, clearTokens } from '@/lib/auth'
import type { User } from '@/types/api'

interface AuthContextType {
  user: User | null
  isLoading: boolean
  isAuthenticated: boolean
  login: (accessToken: string, refreshToken: string, user: User) => void
  logout: () => void
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const token = getAccessToken()
    if (!token) {
      setIsLoading(false)
      return
    }

    apiFetch<User>('/auth/me')
      .then(setUser)
      .catch(() => clearTokens())
      .finally(() => setIsLoading(false))
  }, [])

  const login = useCallback((accessToken: string, refreshToken: string, user: User) => {
    setTokens(accessToken, refreshToken)
    setUser(user)
  }, [])

  const logout = useCallback(() => {
    clearTokens()
    setUser(null)
  }, [])

  return (
    <AuthContext.Provider value={{
      user,
      isLoading,
      isAuthenticated: !!user,
      login,
      logout,
    }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
```

- [ ] **Step 2: Create auth layout**

Create `web/src/layouts/auth-layout.tsx`:

```tsx
import { Outlet } from 'react-router-dom'

export function AuthLayout() {
  return (
    <div className="min-h-screen bg-base flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <h1 className="text-xl font-bold tracking-[0.2em] text-text-primary">AERODOCS</h1>
        </div>
        <div className="bg-surface border border-border rounded-lg p-6">
          <Outlet />
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 3: Create app shell layout**

Create `web/src/layouts/app-shell.tsx`:

```tsx
import { Outlet, NavLink, useNavigate } from 'react-router-dom'
import { LayoutDashboard, ScrollText, Settings, LogOut } from 'lucide-react'
import { useAuth } from '@/hooks/use-auth'

export function AppShell() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  const navItems = [
    { to: '/', icon: LayoutDashboard, label: 'Fleet Dashboard' },
    { to: '/audit-logs', icon: ScrollText, label: 'Audit Logs' },
    { to: '/settings', icon: Settings, label: 'Settings' },
  ]

  return (
    <div className="min-h-screen bg-base flex flex-col">
      {/* Top Telemetry Bar */}
      <header className="bg-surface border-b border-border px-4 py-2 flex items-center justify-between text-xs">
        <div className="flex items-center gap-4">
          <span className="font-bold text-sm tracking-[0.1em] text-text-primary">AERODOCS</span>
          <span className="text-text-faint">|</span>
          <span className="text-text-muted uppercase tracking-widest text-[10px]">Fleet Health</span>
          <span className="text-status-online">● 0 Online</span>
          <span className="text-status-offline">● 0 Offline</span>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-text-secondary">{user?.username}</span>
          <button
            onClick={handleLogout}
            className="text-text-muted hover:text-text-primary transition-colors"
            title="Sign out"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </header>

      <div className="flex flex-1">
        {/* Left Sidebar */}
        <nav className="w-52 bg-surface/50 border-r border-border flex flex-col py-3">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-4 py-2 text-sm transition-colors ${
                  isActive
                    ? 'text-text-primary bg-elevated border-l-2 border-accent'
                    : 'text-text-muted hover:text-text-secondary border-l-2 border-transparent'
                }`
              }
            >
              <Icon className="w-4 h-4" />
              {label}
            </NavLink>
          ))}
          <div className="flex-1" />
          <div className="px-4 text-[10px] text-text-faint uppercase tracking-widest">v0.1.0</div>
        </nav>

        {/* Main Content */}
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: Verify frontend builds**

```bash
cd web && npx tsc --noEmit
```

Expected: no type errors (we haven't wired up routes yet, just verifying types compile).

- [ ] **Step 5: Commit**

```bash
git add web/src/hooks/ web/src/layouts/
git commit -m "feat: add AuthProvider context, auth layout, and app shell layout"
```

---

## Task 15: Frontend Auth Pages

**Files:**
- Create: `web/src/pages/login.tsx`
- Create: `web/src/pages/login-totp.tsx`
- Create: `web/src/pages/setup.tsx`
- Create: `web/src/pages/setup-totp.tsx`
- Create: `web/src/pages/dashboard.tsx`

- [ ] **Step 1: Create login page**

Create `web/src/pages/login.tsx`:

```tsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiFetch } from '@/lib/api'
import type { LoginRequest, LoginResponse, AuthStatusResponse } from '@/types/api'

export function LoginPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const resp = await apiFetch<LoginResponse>('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password } satisfies LoginRequest),
      })

      if (resp.requires_totp_setup && resp.setup_token) {
        navigate('/setup/totp', { state: { setupToken: resp.setup_token } })
      } else if (resp.totp_token) {
        navigate('/login/totp', { state: { totpToken: resp.totp_token } })
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-4">Sign In</div>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      <div className="space-y-3">
        <input
          type="text"
          placeholder="username"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          autoFocus
          required
        />
        <input
          type="password"
          placeholder="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          required
        />
        <button
          type="submit"
          disabled={loading}
          className="w-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
        >
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </div>
    </form>
  )
}
```

- [ ] **Step 2: Create TOTP verification page**

Create `web/src/pages/login-totp.tsx`:

```tsx
import { useState, useRef, useEffect } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { apiFetch } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import type { LoginTOTPRequest, AuthResponse } from '@/types/api'

export function LoginTOTPPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const totpToken = (location.state as { totpToken?: string })?.totpToken

  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])

  useEffect(() => {
    if (!totpToken) navigate('/login')
  }, [totpToken, navigate])

  const handleDigitChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return

    const newDigits = [...digits]
    newDigits[index] = value.slice(-1)
    setDigits(newDigits)

    if (value && index < 5) {
      inputRefs.current[index + 1]?.focus()
    }

    // Auto-submit when all 6 digits entered
    if (newDigits.every(d => d) && index === 5) {
      submitCode(newDigits.join(''))
    }
  }

  const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !digits[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const submitCode = async (code: string) => {
    if (!totpToken) return
    setError('')
    setLoading(true)

    try {
      const resp = await apiFetch<AuthResponse>('/auth/login/totp', {
        method: 'POST',
        body: JSON.stringify({ totp_token: totpToken, code } satisfies LoginTOTPRequest),
      })

      login(resp.access_token, resp.refresh_token, resp.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed')
      setDigits(['', '', '', '', '', ''])
      inputRefs.current[0]?.focus()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-2">Two-Factor Authentication</div>
      <p className="text-text-faint text-xs mb-4">Enter the 6-digit code from your authenticator app</p>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      <div className="flex gap-2 justify-center mb-4">
        {digits.map((digit, i) => (
          <input
            key={i}
            ref={el => { inputRefs.current[i] = el }}
            type="text"
            inputMode="numeric"
            maxLength={1}
            value={digit}
            onChange={(e) => handleDigitChange(i, e.target.value)}
            onKeyDown={(e) => handleKeyDown(i, e)}
            className="w-10 h-12 bg-elevated border border-border rounded text-center text-lg font-mono text-text-primary focus:outline-none focus:border-accent"
            autoFocus={i === 0}
            disabled={loading}
          />
        ))}
      </div>

      <button
        onClick={() => submitCode(digits.join(''))}
        disabled={loading || digits.some(d => !d)}
        className="w-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
      >
        {loading ? 'Verifying...' : 'Verify'}
      </button>
    </div>
  )
}
```

- [ ] **Step 3: Create setup page (initial admin registration)**

Create `web/src/pages/setup.tsx`:

```tsx
import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiFetch } from '@/lib/api'
import type { RegisterRequest, AuthStatusResponse } from '@/types/api'

export function SetupPage() {
  const navigate = useNavigate()
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [passwordErrors, setPasswordErrors] = useState<string[]>([])

  useEffect(() => {
    apiFetch<AuthStatusResponse>('/auth/status')
      .then(resp => {
        if (resp.initialized) navigate('/login')
      })
      .catch(() => {})
  }, [navigate])

  const validatePassword = (pw: string) => {
    const errors: string[] = []
    if (pw.length < 12) errors.push('At least 12 characters')
    if (!/[A-Z]/.test(pw)) errors.push('One uppercase letter')
    if (!/[a-z]/.test(pw)) errors.push('One lowercase letter')
    if (!/\d/.test(pw)) errors.push('One digit')
    if (!/[^a-zA-Z0-9]/.test(pw)) errors.push('One special character')
    setPasswordErrors(errors)
    return errors.length === 0
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!validatePassword(password)) return

    setError('')
    setLoading(true)

    try {
      const resp = await apiFetch<{ setup_token: string }>('/auth/register', {
        method: 'POST',
        body: JSON.stringify({ username, email, password } satisfies RegisterRequest),
      })

      navigate('/setup/totp', { state: { setupToken: resp.setup_token } })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  return (
    <form onSubmit={handleSubmit}>
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-4">Create Admin Account</div>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      <div className="space-y-3">
        <input
          type="text" placeholder="username" value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          autoFocus required
        />
        <input
          type="email" placeholder="email" value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
          required
        />
        <div>
          <input
            type="password" placeholder="password (min 12 chars)" value={password}
            onChange={(e) => { setPassword(e.target.value); validatePassword(e.target.value) }}
            className="w-full bg-elevated border border-border rounded px-3 py-2 text-sm text-text-primary placeholder:text-text-faint focus:outline-none focus:border-accent"
            required
          />
          {password && passwordErrors.length > 0 && (
            <div className="mt-2 space-y-1">
              {passwordErrors.map(err => (
                <div key={err} className="text-status-warning text-[10px]">• {err}</div>
              ))}
            </div>
          )}
        </div>
        <button
          type="submit" disabled={loading}
          className="w-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
        >
          {loading ? 'Creating...' : 'Create Account'}
        </button>
      </div>
    </form>
  )
}
```

- [ ] **Step 4: Create TOTP setup page**

Create `web/src/pages/setup-totp.tsx`:

```tsx
import { useState, useEffect, useRef } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { apiFetchWithToken } from '@/lib/api'
import { useAuth } from '@/hooks/use-auth'
import type { TOTPSetupResponse, TOTPEnableRequest, AuthResponse } from '@/types/api'

export function SetupTOTPPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { login } = useAuth()
  const setupToken = (location.state as { setupToken?: string })?.setupToken

  const [totpData, setTotpData] = useState<TOTPSetupResponse | null>(null)
  const [digits, setDigits] = useState(['', '', '', '', '', ''])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const inputRefs = useRef<(HTMLInputElement | null)[]>([])
  const hasFetched = useRef(false)

  useEffect(() => {
    if (!setupToken) { navigate('/login'); return }
    if (hasFetched.current) return
    hasFetched.current = true

    apiFetchWithToken<TOTPSetupResponse>('/auth/totp/setup', setupToken, { method: 'POST' })
      .then(setTotpData)
      .catch(() => setError('Failed to generate TOTP secret'))
  }, [setupToken, navigate])

  const handleDigitChange = (index: number, value: string) => {
    if (!/^\d*$/.test(value)) return
    const newDigits = [...digits]
    newDigits[index] = value.slice(-1)
    setDigits(newDigits)
    if (value && index < 5) inputRefs.current[index + 1]?.focus()
    if (newDigits.every(d => d) && index === 5) submitCode(newDigits.join(''))
  }

  const handleKeyDown = (index: number, e: React.KeyboardEvent) => {
    if (e.key === 'Backspace' && !digits[index] && index > 0) {
      inputRefs.current[index - 1]?.focus()
    }
  }

  const submitCode = async (code: string) => {
    if (!setupToken) return
    setError('')
    setLoading(true)

    try {
      const resp = await apiFetchWithToken<AuthResponse>('/auth/totp/enable', setupToken, {
        method: 'POST',
        body: JSON.stringify({ code } satisfies TOTPEnableRequest),
      })

      login(resp.access_token, resp.refresh_token, resp.user)
      navigate('/')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Verification failed')
      setDigits(['', '', '', '', '', ''])
      inputRefs.current[0]?.focus()
    } finally {
      setLoading(false)
    }
  }

  return (
    <div>
      <div className="text-text-muted text-[10px] uppercase tracking-widest mb-2">Set Up Two-Factor Authentication</div>
      <p className="text-text-faint text-xs mb-4">Scan this QR code with your authenticator app, then enter the 6-digit code</p>

      {error && (
        <div className="bg-status-error/10 border border-status-error/20 text-status-error text-xs rounded px-3 py-2 mb-4">
          {error}
        </div>
      )}

      {totpData && (
        <>
          <div className="bg-white rounded-lg p-4 mb-4 flex items-center justify-center">
            <img src={`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(totpData.qr_url)}`} alt="TOTP QR Code" className="w-48 h-48" />
          </div>

          <div className="bg-elevated border border-border rounded px-3 py-2 mb-4">
            <div className="text-text-muted text-[10px] uppercase tracking-widest mb-1">Manual Entry Key</div>
            <code className="text-text-primary text-xs font-mono break-all select-all">{totpData.secret}</code>
          </div>

          <div className="flex gap-2 justify-center mb-4">
            {digits.map((digit, i) => (
              <input
                key={i}
                ref={el => { inputRefs.current[i] = el }}
                type="text" inputMode="numeric" maxLength={1}
                value={digit}
                onChange={(e) => handleDigitChange(i, e.target.value)}
                onKeyDown={(e) => handleKeyDown(i, e)}
                className="w-10 h-12 bg-elevated border border-border rounded text-center text-lg font-mono text-text-primary focus:outline-none focus:border-accent"
                autoFocus={i === 0}
                disabled={loading}
              />
            ))}
          </div>

          <button
            onClick={() => submitCode(digits.join(''))}
            disabled={loading || digits.some(d => !d)}
            className="w-full bg-accent hover:bg-accent-hover text-white text-sm font-semibold rounded py-2 transition-colors disabled:opacity-50"
          >
            {loading ? 'Verifying...' : 'Verify & Enable 2FA'}
          </button>
        </>
      )}
    </div>
  )
}
```

- [ ] **Step 5: Create dashboard stub**

Create `web/src/pages/dashboard.tsx`:

```tsx
export function DashboardPage() {
  return (
    <div>
      <h2 className="text-lg font-semibold text-text-primary mb-2">Fleet Dashboard</h2>
      <p className="text-text-muted text-sm">Server fleet will be displayed here in sub-project 2.</p>
    </div>
  )
}
```

- [ ] **Step 6: Wire up routing in App.tsx**

Update `web/src/App.tsx`:

```tsx
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/lib/query-client'
import { AuthProvider, useAuth } from '@/hooks/use-auth'
import { AuthLayout } from '@/layouts/auth-layout'
import { AppShell } from '@/layouts/app-shell'
import { LoginPage } from '@/pages/login'
import { LoginTOTPPage } from '@/pages/login-totp'
import { SetupPage } from '@/pages/setup'
import { SetupTOTPPage } from '@/pages/setup-totp'
import { DashboardPage } from '@/pages/dashboard'

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, isLoading } = useAuth()

  if (isLoading) {
    return (
      <div className="min-h-screen bg-base flex items-center justify-center">
        <div className="text-text-muted text-sm">Loading...</div>
      </div>
    )
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}

function AppRoutes() {
  return (
    <Routes>
      {/* Public auth routes */}
      <Route element={<AuthLayout />}>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/login/totp" element={<LoginTOTPPage />} />
        <Route path="/setup" element={<SetupPage />} />
        <Route path="/setup/totp" element={<SetupTOTPPage />} />
      </Route>

      {/* Protected routes */}
      <Route element={<ProtectedRoute><AppShell /></ProtectedRoute>}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/audit-logs" element={<div className="text-text-muted text-sm">Audit logs — sub-project 7</div>} />
        <Route path="/settings" element={<div className="text-text-muted text-sm">Settings — sub-project 7</div>} />
        <Route path="/servers/:id" element={<div className="text-text-muted text-sm">Server detail — sub-project 4</div>} />
      </Route>
    </Routes>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <AppRoutes />
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
```

- [ ] **Step 7: Verify frontend builds**

```bash
cd web && npm run build
```

Expected: Vite builds without errors.

- [ ] **Step 8: Commit**

```bash
git add web/src/
git commit -m "feat: add auth pages (login, TOTP, setup), app shell, routing, and dashboard stub"
```

---

## Task 16: Frontend Embedding & Full Build Integration

**Files:**
- Modify: `hub/embed.go`
- Modify: `hub/internal/server/server.go`
- Modify: `Makefile`

- [ ] **Step 1: Set up go:embed for frontend**

Update `hub/embed.go`:

```go
package hub

import "embed"

//go:embed all:web/dist
var FrontendFS embed.FS
```

Note: this requires `web/dist/` to exist relative to the `hub/` directory. The Makefile will copy `web/dist/` into `hub/web/dist/` before building.

- [ ] **Step 2: Update server to serve embedded frontend**

Add to `hub/internal/server/router.go`, after the API routes:

```go
// Serve embedded frontend (SPA with client-side routing)
mux.Handle("/", s.spaHandler())
```

The server needs a reference to the embedded filesystem. Update `hub/internal/server/server.go` to accept it:

Add `FrontendFS embed.FS` to the `Config` struct and store it on `Server`.

Then add the `spaHandler` method:

```go
func (s *Server) spaHandler() http.Handler {
	if s.frontendFS == nil {
		// Dev mode — frontend served by Vite dev server
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Frontend not embedded — use Vite dev server", http.StatusNotFound)
		})
	}

	// Production — serve embedded frontend with SPA fallback
	subFS, err := fs.Sub(*s.frontendFS, "web/dist")
	if err != nil {
		panic(fmt.Sprintf("embedded frontend not found: %v", err))
	}

	fileServer := http.FileServer(http.FS(subFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the requested file
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists
		f, err := subFS.Open(strings.TrimPrefix(path, "/"))
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fall back to index.html for SPA client-side routing
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}
```

Add `"io/fs"` and `"strings"` to the imports.

Update `hub/embed.go` to pass the FS to the server config, and update `main.go` to wire it through.

- [ ] **Step 3: Update Makefile for full build**

Update `Makefile`:

```makefile
.PHONY: dev-hub dev-web build test clean

# Development (run these in separate terminals)
dev-hub:
	cd hub && go run ./cmd/aerodocs/ --dev --addr :8080

dev-web:
	cd web && npm run dev

# Build production binary (frontend embedded)
build: build-web embed-web build-hub

build-web:
	cd web && npm run build

embed-web:
	rm -rf hub/web/dist
	mkdir -p hub/web
	cp -r web/dist hub/web/dist

build-hub:
	cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/

# Test
test: test-hub

test-hub:
	cd hub && go test ./...

clean:
	rm -rf bin/ web/dist/ hub/web/dist/ hub/aerodocs
```

- [ ] **Step 4: Verify full build pipeline**

```bash
make build
```

Expected: frontend builds, copies to `hub/web/dist/`, Go binary compiles with embedded frontend.

- [ ] **Step 5: Run all backend tests**

```bash
make test
```

Expected: all Go tests pass.

- [ ] **Step 6: Commit**

```bash
git add hub/embed.go hub/internal/server/ Makefile
git commit -m "feat: add frontend embedding and full build pipeline"
```

---

## Task 17: Final Integration Test

**Files:** None — this task verifies the full system works end-to-end.

- [ ] **Step 1: Build the full binary**

```bash
make clean && make build
```

Expected: `bin/aerodocs` binary exists.

- [ ] **Step 2: Start the server**

```bash
./bin/aerodocs --addr :8080 --db test.db &
```

Expected: "AeroDocs Hub listening on :8080"

- [ ] **Step 3: Verify auth status (not initialized)**

```bash
curl -s http://localhost:8080/api/auth/status | jq .
```

Expected: `{ "initialized": false }`

- [ ] **Step 4: Register first admin**

```bash
curl -s -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","email":"admin@test.com","password":"MyP@ssw0rd!234"}' | jq .
```

Expected: `{ "setup_token": "...", "user": { ... } }`

- [ ] **Step 5: Verify registration is now blocked**

```bash
curl -s -X POST http://localhost:8080/api/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"hacker","email":"hacker@test.com","password":"MyP@ssw0rd!234"}' | jq .
```

Expected: `{ "error": "registration disabled..." }` with 403 status.

- [ ] **Step 6: Verify CLI break-glass**

```bash
./bin/aerodocs admin reset-totp --username admin --db test.db
```

Expected: prints temporary password.

- [ ] **Step 7: Clean up test artifacts**

```bash
kill %1 2>/dev/null; rm -f test.db
```

- [ ] **Step 8: Final commit — tag sub-project 1 complete**

```bash
git add -A && git status  # Verify no unexpected files
git tag v0.1.0-foundation
```
