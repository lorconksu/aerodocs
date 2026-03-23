# AeroDocs Sub-project 1: Foundation & Auth — Design Spec

**Date:** 2026-03-23
**Status:** Approved
**Scope:** Project scaffolding, SQLite schema, Go server skeleton, user authentication (login/register/mandatory 2FA), React app shell with routing

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Backend language | Go | Fast development, excellent concurrency (goroutines for WebSocket/gRPC), trivial cross-compilation, single-binary deployment via `go:embed`. Industry standard for infrastructure tools. |
| Frontend build | Vite SPA embedded via `go:embed` | Single binary serves both API and frontend. No separate web server needed — ideal for self-hosted deployment. |
| Frontend↔Hub API | REST (JSON over HTTP) | Simple, well-tooled, TanStack Query works natively. WebSockets added for streaming in later sub-projects. |
| Hub↔Agent API | gRPC | Efficient binary protocol with streaming support and strongly typed `.proto` contracts. |
| Auth strategy | JWT (access + refresh tokens) | Stateless. No server-side session storage. Access token (15 min) + refresh token (7 day, single-use). |
| 2FA | Mandatory TOTP for all users | Every user must set up TOTP before accessing the app. |
| User model | Multi-user with roles (admin/viewer) | Admin can manage servers, users, permissions. Viewers can only browse accessible servers. |
| Backend architecture | Layered (handler → store → SQLite) | Right balance of structure and simplicity. Standard pattern for Go infrastructure tools. |
| Repo structure | Monorepo | Single repo with `/hub`, `/agent`, `/web`, `/proto`. Codebase is not large enough to benefit from multi-repo. |

## 2. Repository Structure

```
aerodocs/
├── hub/                        # Go backend (Hub server)
│   ├── cmd/
│   │   └── aerodocs/
│   │       └── main.go         # Entry point — starts HTTP server, runs migrations
│   ├── internal/
│   │   ├── server/             # HTTP router, middleware, route registration
│   │   ├── auth/               # JWT issue/validate, TOTP generate/verify, password hashing
│   │   ├── store/              # SQLite repository (UserStore, ServerStore, AuditStore, etc.)
│   │   ├── model/              # Shared domain types (User, Server, Permission, AuditEntry)
│   │   └── migrate/            # Ordered SQL migration files + migration runner
│   ├── embed.go                # go:embed directive for frontend dist/
│   ├── go.mod
│   └── go.sum
│
├── agent/                      # Go agent (runs on remote servers)
│   ├── cmd/
│   │   └── aerodocs-agent/
│   │       └── main.go
│   ├── internal/               # (populated in sub-project 3)
│   ├── go.mod
│   └── go.sum
│
├── web/                        # React frontend (Vite SPA)
│   ├── src/
│   │   ├── components/ui/      # shadcn/ui primitives (Button, Input, Card, etc.)
│   │   ├── layouts/            # AppShell, AuthLayout
│   │   ├── pages/              # Route-level page components
│   │   ├── hooks/              # Custom React hooks
│   │   ├── lib/                # Utilities, API client, auth helpers
│   │   ├── types/              # Shared TypeScript interfaces
│   │   └── styles/             # Tailwind config, design tokens
│   ├── index.html
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── tsconfig.json
│   └── package.json
│
├── proto/                      # Shared .proto definitions (Hub↔Agent gRPC)
│   └── aerodocs/v1/            # Versioned proto package (populated in sub-project 3)
│
├── docs/                       # Design specs, architecture docs
├── Makefile                    # Build commands: make build, make dev, make migrate
├── claude.md
├── aerodocs-prd.md
└── areodocs-sdd.md
```

Key points:
- `hub/` and `agent/` are separate Go modules (separate `go.mod`) for independent builds
- `web/dist/` output is embedded into the Hub binary at build time
- `proto/` generates code into both `hub/` and `agent/`
- Makefile orchestrates: `make build` compiles frontend then embeds into Go binary

## 3. SQLite Schema

### 3.1 Users Table

```sql
CREATE TABLE users (
    id            TEXT PRIMARY KEY,  -- UUID
    username      TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'viewer' CHECK(role IN ('admin', 'viewer')),
    totp_secret   TEXT,              -- NULL = 2FA not set up yet
    totp_enabled  INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 3.2 Audit Logs Table

```sql
CREATE TABLE audit_logs (
    id         TEXT PRIMARY KEY,  -- UUID
    user_id    TEXT,              -- NULL for system events
    action     TEXT NOT NULL,     -- e.g., 'user.login', 'user.login_failed'
    target     TEXT,              -- What was acted on
    detail     TEXT,              -- JSON blob with extra context
    ip_address TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
```

### 3.3 Config Table

```sql
CREATE TABLE _config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Stores: jwt_signing_key (generated on first run)
```

### 3.4 Migrations Table

```sql
CREATE TABLE _migrations (
    id         INTEGER PRIMARY KEY,
    filename   TEXT NOT NULL UNIQUE,
    applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Design choices:
- UUIDs as TEXT (SQLite has no native UUID type)
- Timestamps as TEXT in ISO 8601 format (works with SQLite `datetime()` functions)
- `audit_logs` is append-only — no UPDATE or DELETE operations exposed
- `totp_secret` nullable — NULL means 2FA not configured yet
- Migration runner uses numbered SQL files, tracks applied state in `_migrations`
- `servers` and `permissions` tables are defined in sub-project 2 — intentionally omitted here

## 4. Go Backend Architecture

### 4.1 Package Layout

```
hub/internal/
├── server/
│   ├── server.go         # Server struct, Start(), Shutdown()
│   ├── router.go         # Route registration (mux setup)
│   ├── middleware.go      # Auth middleware, logging, CORS
│   └── handlers_auth.go  # Auth route handlers
├── auth/
│   ├── jwt.go            # GenerateTokens(), ValidateToken(), RefreshToken()
│   ├── password.go       # HashPassword(), ComparePassword() (bcrypt)
│   └── totp.go           # GenerateSecret(), GenerateQR(), ValidateCode()
├── store/
│   ├── store.go          # Store struct (*sql.DB), constructor, transaction helper
│   ├── users.go          # User CRUD operations
│   └── audit.go          # Audit log insert + query (paginated, filterable)
├── model/
│   ├── user.go           # User, CreateUserRequest, LoginRequest, TokenPair
│   └── audit.go          # AuditEntry, AuditFilter
└── migrate/
    ├── migrate.go        # RunMigrations()
    └── migrations/       # Numbered .sql files
```

### 4.2 Request Flow

```
Browser → HTTP Request
  → CORS middleware
  → Logging middleware
  → Auth middleware (skipped for /login, /register, /auth/status)
    → Extracts JWT from Authorization: Bearer <token> header
    → Validates signature + expiry
    → Reads `type` claim and enforces per-route token type requirements:
        - /api/auth/totp/setup, /api/auth/totp/enable → requires type="setup"
        - /api/auth/refresh → requires type="refresh" (special case: token extracted from request body, not Authorization header)
        - All other protected routes → requires type="access"
    → Attaches user context (user ID, role, token type) to request
  → Route handler
    → Validates request body
    → Calls store methods
    → Returns JSON response
```

The auth middleware is a single middleware that handles all token types. It validates the JWT signature and expiry, then checks the `type` claim against the route's required token type. Routes declare their required token type at registration time in `router.go`.

### 4.3 Go Dependencies

| Library | Purpose |
|---------|---------|
| `net/http` (stdlib, Go 1.22+) | HTTP router with pattern matching |
| `modernc.org/sqlite` | Pure Go SQLite driver (no CGo) |
| `golang-jwt/jwt/v5` | JWT creation and validation |
| `pquerna/otp` | TOTP secret generation and verification |
| `golang.org/x/crypto/bcrypt` | Password hashing |
| `google/uuid` | UUID generation |

### 4.4 API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/auth/status` | No | Returns `{ initialized: bool }` — whether any user exists |
| POST | `/api/auth/register` | No | Create first admin user (disabled after first user exists) |
| POST | `/api/auth/login` | No | Username + password → `202 { totp_token }` or `200 { setup_token }` |
| POST | `/api/auth/login/totp` | No | TOTP code + totp_token → `200 { access_token, refresh_token }` |
| POST | `/api/auth/refresh` | Refresh | Exchange refresh token for new token pair |
| GET | `/api/auth/me` | Access | Get current user profile |
| POST | `/api/auth/totp/setup` | Setup | Generate TOTP secret + QR code URL |
| POST | `/api/auth/totp/enable` | Setup | Verify TOTP code, activate 2FA, return full token pair |
| POST | `/api/auth/totp/disable` | Access (Admin) | Disable another user's 2FA. Request: `{ user_id, admin_totp_code }`. The admin must provide *their own* TOTP code to prove identity. Target user's TOTP is cleared, forcing re-setup on next login. |
| GET | `/api/users` | Access (Admin) | List all users |
| POST | `/api/users` | Access (Admin) | Create a new user (see 4.5 for request/response shape) |
| GET | `/api/audit-logs` | Access (Admin) | List audit logs (paginated, filterable) |

### 4.5 Create User Endpoint Detail

**Request:** `POST /api/users`
```json
{
  "username": "jane",
  "email": "jane@example.com",
  "role": "viewer"
}
```

The admin does **not** supply a password. The Hub auto-generates a secure temporary password (20 chars, mixed case + digits + specials).

**Response:** `201 Created`
```json
{
  "user": {
    "id": "uuid",
    "username": "jane",
    "email": "jane@example.com",
    "role": "viewer",
    "totp_enabled": false,
    "created_at": "2026-03-23T12:00:00Z"
  },
  "temporary_password": "Xk9$mP2wLq..."
}
```

The temporary password is returned **once** in this response. The admin communicates it to the new user out-of-band (e.g., in person, secure message). On first login, the user is forced through the TOTP setup flow (see Section 5.3).

## 5. Authentication Flow

### 5.1 JWT Token Types

| Type | Scope | Lifetime | Purpose |
|------|-------|----------|---------|
| `setup` | Only `/api/auth/totp/*` | 10 min | Proves password verified, allows TOTP setup only |
| `totp` | Only `/api/auth/login/totp` | 60 sec | Proves password verified, awaiting TOTP code |
| `access` | All authenticated endpoints | 15 min | Normal API access |
| `refresh` | Only `/api/auth/refresh` | 7 days | Exchange for new access + refresh pair (single-use) |

JWT payload:
```json
{
  "sub": "user-uuid",
  "role": "admin",
  "exp": 1700000000,
  "iat": 1699999100,
  "type": "access"
}
```

Signing: HMAC-SHA256 with a 256-bit random key stored in `_config` table.

### 5.2 Initial Setup (First Run)

```
User opens app → GET /api/auth/status → { initialized: false }
→ Shows "Create Admin Account" form
→ POST /api/auth/register { username, email, password }
→ 200 { setup_token, user }
→ Shows "Set Up 2FA" screen with QR code
→ POST /api/auth/totp/setup (with setup_token)
→ POST /api/auth/totp/enable { code }
→ 200 { access_token, refresh_token, user }
→ Redirects to dashboard
```

Registration endpoint returns 403 after the first user exists. Subsequent users are created by admins via `POST /api/users`.

### 5.3 New User First Login

```
Admin creates user → user gets temporary password
User logs in → POST /api/auth/login { username, password }
→ 200 { setup_token, requires_totp_setup: true }
→ Same 2FA setup flow as initial setup
→ After TOTP enabled → full JWT pair returned
```

### 5.4 Subsequent Login (Password + TOTP)

```
POST /api/auth/login { username, password }
→ 202 { totp_token }
→ POST /api/auth/login/totp { totp_token, code }
→ 200 { access_token, refresh_token, user }
```

There is no "login without 2FA" path. Every user must have TOTP enabled.

### 5.5 Token Refresh

The refresh token is sent in the **request body** (not the Authorization header):

```
Access token expires (15 min)
→ TanStack Query interceptor catches 401
→ POST /api/auth/refresh
  Body: { "refresh_token": "<token>" }
→ 200 { access_token, refresh_token } (new pair, old refresh token invalidated)
→ Original request retried with new access token

Refresh token expired (7 days):
→ 401 → Frontend clears tokens, redirects to /login
```

### 5.6 CLI Break-Glass

If the admin loses their phone and is locked out of 2FA:

```bash
./aerodocs admin reset-totp --username admin
```

This clears `totp_enabled` and generates a new temporary password, which is **printed to stdout**. The admin must note it down. Forces the setup flow on next login. Only accessible via direct CLI on the Hub server.

## 6. React Frontend Architecture

### 6.1 Project Structure

```
web/src/
├── components/ui/       # shadcn/ui primitives (Button, Input, Card, etc.)
├── layouts/
│   ├── app-shell.tsx    # Authenticated shell: telemetry bar + sidebar + outlet
│   └── auth-layout.tsx  # Centered card layout for login/setup pages
├── pages/
│   ├── login.tsx
│   ├── login-totp.tsx
│   ├── setup.tsx        # First-run admin registration
│   ├── setup-totp.tsx   # Mandatory 2FA setup
│   └── dashboard.tsx    # Placeholder (populated in sub-project 2)
├── hooks/
│   └── use-auth.ts      # Auth context: user, tokens, login(), logout()
├── lib/
│   ├── api.ts           # Fetch wrapper with JWT injection + 401 refresh interceptor
│   ├── auth.ts          # Token storage (localStorage), refresh logic
│   └── query-client.ts  # TanStack Query client config
├── types/
│   └── api.ts           # User, TokenPair, LoginRequest, TOTPSetupResponse, etc.
└── styles/
    └── tokens.css       # CSS custom properties for design tokens
```

### 6.2 Route Structure

| Path | Layout | Component | Auth Required |
|------|--------|-----------|---------------|
| `/login` | AuthLayout | LoginPage | No |
| `/login/totp` | AuthLayout | LoginTOTPPage | No |
| `/setup` | AuthLayout | SetupPage | No |
| `/setup/totp` | AuthLayout | SetupTOTPPage | No |
| `/` | AppShell | DashboardPage | Yes |
| `/audit-logs` | AppShell | AuditLogsPage (stub) | Yes (Admin) |
| `/settings` | AppShell | SettingsPage (stub) | Yes |
| `/servers/:id` | AppShell | ServerDetailPage (stub) | Yes |

Routes marked as "stub" render a placeholder in sub-project 1 and are implemented in later sub-projects.

### 6.3 App Shell Layout

```
┌──────────────────────────────────────────────────────┐
│  AERODOCS  │  Fleet Health: ● 12 Online  ● 1 Offline │  + Add Server  │  admin ▾  │
├────────────┼─────────────────────────────────────────────────────────────────────────┤
│            │                                                                         │
│  Fleet     │  Breadcrumbs                                                            │
│  Dashboard │                                                                         │
│            │  ┌─────────────────────────────────────────────────────────────────────┐ │
│  Audit     │  │                                                                     │ │
│  Logs      │  │                     Main Content Area                               │ │
│            │  │                   (React Router Outlet)                              │ │
│  Settings  │  │                                                                     │ │
│            │  └─────────────────────────────────────────────────────────────────────┘ │
│            │                                                                         │
│  v0.1.0    │                                                                         │
└────────────┴─────────────────────────────────────────────────────────────────────────┘
```

### 6.4 State Management

- **Auth state**: React Context (`AuthProvider`) wrapping the app. Stores current user + tokens. Exposes `login()`, `logout()`, `isAuthenticated`.
- **Server state**: TanStack Query for all API data. No Redux or Zustand.
- **Token storage**: `localStorage` for access + refresh tokens. API client reads automatically.

### 6.5 Design Tokens

```css
:root {
  --bg-base: #0a0a0b;
  --bg-surface: #111113;
  --bg-elevated: #18181b;
  --border: #27272a;
  --text-primary: #f4f4f5;
  --text-secondary: #a1a1aa;
  --text-muted: #71717a;
  --accent-blue: #3b82f6;
  --status-online: #22c55e;
  --status-warning: #f59e0b;
  --status-offline: #ef4444;
}
```

Mapped into Tailwind via `theme.extend.colors` (e.g., `bg-surface`, `text-muted`).

## 7. Security

### 7.1 Password Policy

- Minimum 12 characters
- Must include at least one uppercase letter
- Must include at least one lowercase letter
- Must include at least one digit
- Must include at least one special character
- Validated server-side in Go handlers, mirrored client-side in React forms with real-time feedback
- Stored as bcrypt hash (cost factor 12)
- Constant-time comparison for login failures (no timing oracle)

### 7.2 Rate Limiting

- Auth endpoints only: 5 login attempts per IP per 60-second window
- In-memory counter (resets on Hub restart — acceptable for self-hosted)
- Returns `429 Too Many Requests` with `Retry-After` header

### 7.3 CORS

- Development: allow `http://localhost:5173` (Vite dev server)
- Production: not needed (same-origin, frontend embedded in Go binary)

### 7.4 Input Validation

- Username: 3-32 chars, alphanumeric + underscores
- TOTP code: exactly 6 digits
- All validation in Go handlers before touching the store

### 7.5 Audit Trail

Events logged: `user.login`, `user.login_failed`, `user.login_totp_failed`, `user.registered`, `user.totp_setup`, `user.totp_enabled`, `user.created`

Each entry includes: user ID, IP address, timestamp, optional JSON detail blob. Entries are immutable — no update/delete API.

### 7.6 CLI Break-Glass

`./aerodocs admin reset-totp --username <user>` — resets TOTP and generates temporary password. Only accessible via direct CLI on the Hub server.

## 8. Out of Scope (Deferred to Later Sub-projects)

- Path sanitization (sub-project 4)
- Network breakaway / WebSocket reconnection (sub-project 5)
- Per-server permissions enforcement (sub-project 2)
- Agent binary and gRPC service definitions (sub-project 3)
- File tree browsing, log tailing, file uploads (sub-projects 4-6)
- `react-resizable-panels` — deferred to sub-project 4 when multi-pane layouts are needed for file tree + document viewer
