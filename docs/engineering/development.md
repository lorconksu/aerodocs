# AeroDocs Development Guide

## Prerequisites

- Go 1.22+
- Node.js 20+
- Make
- Git

---

## Clone and Setup

```bash
git clone https://github.com/wyiu/aerodocs.git
cd aerodocs

# Install frontend dependencies
cd web && npm install && cd ..
```

---

## Running in Development

The development environment runs two processes — open two terminal windows.

**Terminal 1 — Hub (API server on :8080):**

```bash
make dev-hub
```

This runs `go run ./cmd/aerodocs/ --dev --addr :8080` from the `hub/` directory. The `--dev` flag enables permissive CORS so the Vite dev server can make cross-origin API requests, and disables serving the embedded frontend (Vite handles that instead).

**Terminal 2 — Vite dev server (UI on :5173):**

```bash
make dev-web
```

This runs `npm run dev` from the `web/` directory. Navigate to `http://localhost:5173` in your browser.

On first run, you'll be directed to the setup page to create your admin account and configure TOTP.

### How the Vite proxy works

The Vite config in `web/vite.config.ts` includes a proxy rule:

```typescript
server: {
  proxy: {
    '/api': 'http://localhost:8080',
  },
},
```

Any request the browser makes to `/api/*` is forwarded by Vite to the Go Hub on port 8080. This means the browser always talks to `localhost:5173` (no CORS issues from the browser's perspective), and Vite silently proxies the API calls. In production there is no proxy — the Hub serves both the SPA and the API on the same port.

---

## Building for Production

```bash
make build
```

Output: `bin/aerodocs` — a single self-contained binary.

---

## Running Tests

```bash
make test
```

This runs `go test ./...` from the `hub/` directory. The test suite covers:

- `auth/` — JWT generation, validation, token type enforcement, bcrypt, TOTP
- `store/` — All store methods against a real in-memory SQLite database
- `server/` — HTTP handler tests using `httptest`

There are no frontend tests at this time.

---

## Project Structure Walkthrough

```
aerodocs/
├── Makefile                    # Build orchestration
├── hub/
│   ├── embed.go                # go:embed directive pointing at web/dist
│   ├── go.mod / go.sum
│   ├── cmd/
│   │   └── aerodocs/
│   │       ├── main.go         # Flag parsing, wiring, graceful shutdown
│   │       └── admin.go        # CLI admin subcommands
│   └── internal/
│       ├── auth/
│       │   ├── jwt.go          # Token generation and validation
│       │   ├── password.go     # bcrypt helpers + password policy
│       │   └── totp.go         # TOTP secret generation + code verification
│       ├── migrate/
│       │   ├── migrate.go      # Migration runner
│       │   └── migrations/     # Numbered .sql files (001_, 002_, ...)
│       ├── model/
│       │   ├── user.go         # User, request/response types
│       │   ├── server.go       # Server, request/response types
│       │   └── audit.go        # AuditEntry, action constants
│       ├── server/
│       │   ├── server.go       # Server struct, Init, SPA handler
│       │   ├── router.go       # Route registration
│       │   ├── middleware.go   # Auth middleware, rate limiter, CORS, logging
│       │   ├── respond.go      # respondJSON / respondError helpers
│       │   ├── handlers_auth.go
│       │   ├── handlers_servers.go
│       │   ├── handlers_users.go
│       │   └── handlers_audit.go
│       └── store/
│           ├── store.go        # Store struct, New(), DB pragma setup
│           ├── users.go        # User CRUD
│           ├── servers.go      # Server CRUD
│           ├── audit.go        # Audit log writes and queries
│           └── config.go       # Key-value config store
└── web/
    ├── vite.config.ts
    ├── package.json
    └── src/
        ├── main.tsx            # React root
        ├── App.tsx             # Router setup
        ├── pages/              # One file per route
        ├── components/         # Shared UI components
        ├── hooks/              # Custom React hooks (API calls via TanStack Query)
        ├── layouts/            # Layout wrappers
        ├── lib/                # Utilities
        └── types/              # TypeScript type definitions
```

---

## Adding a New Feature

A typical full-stack feature (e.g. adding a new resource type) involves these steps:

### 1. Write a migration

Create a new numbered SQL file in `hub/internal/migrate/migrations/`:

```
007_create_my_resource.sql
```

```sql
CREATE TABLE my_resources (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Migrations run automatically on the next startup. They are applied in filename order and never re-run.

### 2. Add a model

Add request/response structs to `hub/internal/model/`. Keep this package free of business logic — plain structs only.

```go
// hub/internal/model/my_resource.go
type MyResource struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

type CreateMyResourceRequest struct {
    Name string `json:"name"`
}
```

### 3. Add store methods

Add a new file to `hub/internal/store/` with the SQL queries:

```go
// hub/internal/store/my_resources.go
func (s *Store) CreateMyResource(name string) (*model.MyResource, error) { ... }
func (s *Store) GetMyResource(id string) (*model.MyResource, error) { ... }
func (s *Store) ListMyResources() ([]model.MyResource, error) { ... }
func (s *Store) DeleteMyResource(id string) error { ... }
```

Write tests in `my_resources_test.go` using the `testStore()` helper pattern from existing test files.

### 4. Add HTTP handlers

Create `hub/internal/server/handlers_my_resource.go`:

```go
func (s *Server) handleListMyResources(w http.ResponseWriter, r *http.Request) {
    resources, err := s.store.ListMyResources()
    if err != nil {
        respondError(w, http.StatusInternalServerError, "failed to list resources")
        return
    }
    respondJSON(w, http.StatusOK, resources)
}
```

### 5. Register routes

Add the routes to `hub/internal/server/router.go`:

```go
mux.Handle("GET /api/my-resources", loggingMiddleware(
    s.authMiddleware(auth.TokenTypeAccess, http.HandlerFunc(s.handleListMyResources)),
))
```

### 6. Add a frontend page

Create a new page in `web/src/pages/` and add the route in `web/src/App.tsx`. Use TanStack Query for data fetching (see existing pages for the pattern).

---

## Code Style and Conventions

### Go

- Standard `gofmt` formatting. Run `go fmt ./...` before committing.
- Errors are wrapped with `fmt.Errorf("context: %w", err)` and returned to callers. HTTP handlers convert errors to HTTP responses via `respondError`.
- No global state. Dependencies are injected via the `Server` struct and `Config`.
- All SQL queries are in the `store` package. Handlers must not construct SQL.
- Audit log entries should be written for all user-facing state changes. See `model.Audit*` constants for the naming pattern (`resource.action`).

### TypeScript / React

- Strict TypeScript throughout (`strict: true` in tsconfig).
- TanStack Query manages all server state. Do not use `useEffect` + `useState` for data fetching.
- Components live in `src/components/`, pages in `src/pages/`. Pages are thin — extract reusable UI to components.
- Use the `@/` path alias for all imports within `src/`.

### Commits

Use conventional commit prefixes: `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`.
