# AeroDocs Sub-project 2: Fleet Dashboard & Server Management — Design Spec

**Date:** 2026-03-23
**Status:** Approved
**Scope:** Servers/permissions SQLite schema, server CRUD API, agent registration endpoint, fleet dashboard UI with mass actions, "Add Server" modal

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Server status model | Registration only | Servers are "pending" until agent connects (sub-project 3). No simulated heartbeat. |
| Registration flow | One-time token per server | Admin creates server → Hub generates unique token + curl command. Token expires after 1 hour or first use. |
| Viewer access | Permission-based | Viewers only see servers they have permissions for. Admins see all. |

## 2. SQLite Schema Additions

### 2.1 Servers Table

```sql
-- 004_create_servers.sql
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

- `registration_token` stores a SHA-256 hash of the raw token. Cleared after agent registers.
- `token_expires_at` is 1 hour from creation. Agent registration endpoint rejects expired tokens.
- `labels` is a JSON string for arbitrary key-value metadata.
- `status` starts as `pending`. Agent registration changes it to `online`. Sub-project 3 implements heartbeat transitions to `offline`.

### 2.2 Permissions Table

```sql
-- 005_create_permissions.sql
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

- Created now but UI for managing permissions is deferred until after agents exist (sub-project 3+).
- Admins implicitly have access to all servers — permissions table only restricts viewers.
- `path` defaults to `/` (full server access). More granular paths are used in sub-project 4 (file tree).

## 3. API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/servers` | Access | List servers. Admin: all servers. Viewer: only permitted servers. Supports `?status=pending&search=name` query params. |
| POST | `/api/servers` | Access (Admin) | Create server + generate registration token. Returns server, raw token, and curl install command. |
| GET | `/api/servers/:id` | Access | Get server detail. Viewer must have permission. |
| PUT | `/api/servers/:id` | Access (Admin) | Update server name and labels. |
| DELETE | `/api/servers/:id` | Access (Admin) | Delete a server and its permissions (CASCADE). |
| POST | `/api/servers/batch-delete` | Access (Admin) | Mass delete. Body: `{ "ids": ["id1", "id2"] }`. |
| POST | `/api/servers/register` | None (token in body) | Agent registration. Body: `{ "token": "raw", "hostname": "...", "ip_address": "...", "os": "..." }`. Validates token hash, checks expiry, populates server details, sets status to `online`. |

### 3.1 Create Server Response

```json
{
  "server": {
    "id": "uuid",
    "name": "web-prod-1",
    "status": "pending",
    "created_at": "2026-03-23T12:00:00Z"
  },
  "registration_token": "raw-token-value",
  "install_command": "curl -sSL https://aerodocs.yiucloud.com/install.sh | sudo bash -s -- --token raw-token-value --hub https://aerodocs.yiucloud.com"
}
```

The raw token is returned **once** in this response. The Hub stores only the SHA-256 hash.

### 3.2 Agent Registration Request

```json
{
  "token": "raw-token-value",
  "hostname": "web-prod-1",
  "ip_address": "10.10.1.50",
  "os": "Ubuntu 24.04",
  "agent_version": "0.1.0"
}
```

On success: `200 { "server_id": "uuid", "status": "online" }`

On failure:
- Invalid/expired token: `401 { "error": "invalid or expired registration token" }`
- Already used: `409 { "error": "token already used" }`

### 3.3 List Servers Query Parameters

| Param | Type | Description |
|-------|------|-------------|
| `status` | string | Filter by status: `pending`, `online`, `offline` |
| `search` | string | Search by name (case-insensitive LIKE) |
| `limit` | int | Results per page (default 50, max 100) |
| `offset` | int | Pagination offset |

Response includes `total` count for pagination UI.

## 4. Server Store Methods

| Method | Description |
|--------|-------------|
| `CreateServer(server *model.Server) error` | Insert server with hashed registration token |
| `GetServerByID(id string) (*model.Server, error)` | Get single server |
| `ListServers(filter model.ServerFilter) ([]model.Server, int, error)` | Paginated, filterable list |
| `ListServersForUser(userID string, filter model.ServerFilter) ([]model.Server, int, error)` | Viewer-scoped: only permitted servers |
| `UpdateServer(id string, name string, labels string) error` | Update name and labels |
| `DeleteServer(id string) error` | Delete server (permissions cascade) |
| `DeleteServers(ids []string) error` | Batch delete |
| `GetServerByToken(tokenHash string) (*model.Server, error)` | Find server by hashed registration token |
| `ActivateServer(id, hostname, ip, os, agentVersion string) error` | Set details + status=online, clear token |

## 5. Domain Model Additions

```go
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

## 6. Fleet Dashboard UI

### 6.1 Route

| Path | Layout | Component | Auth |
|------|--------|-----------|------|
| `/` | AppShell | FleetDashboard | Yes |

Replaces the current dashboard stub.

### 6.2 Layout

Page header with title ("Fleet Dashboard"), server count subtitle, and "+ Add Server" button (admin only).

Mass action bar appears above the table when servers are selected: shows count, "Delete Selected" button, "Clear" button.

### 6.3 Server Table Columns

| Column | Content |
|--------|---------|
| Checkbox | Mass-select checkbox (admin only) |
| Status | Green dot (online), red dot (offline), amber dot (pending) |
| Name | Server name, clickable (links to `/servers/:id` — stub for now) |
| Hostname / IP | Monospaced. "—" if pending |
| OS | Operating system. "—" if pending |
| Last Seen | Relative time ("2 min ago"). "Pending agent" in amber for pending servers |
| Actions | Edit / Delete for online/offline. Show command / Delete for pending. |

### 6.4 "Add Server" Modal

Two-step flow within a single modal:

**Step 1:** Input field for server name. "Generate" button.

**Step 2:** Displays the curl install command in a monospace code block with a "Copy" button. Shows token expiry ("Expires in 1 hour"). "Close" button.

### 6.5 Frontend Files

| File | Responsibility |
|------|---------------|
| `web/src/pages/dashboard.tsx` | Rewrite: FleetDashboard with table, filters, mass actions |
| `web/src/pages/add-server-modal.tsx` | Add Server modal component |
| `web/src/types/api.ts` | Add Server, ServerFilter, CreateServerResponse types |
| `web/src/lib/api.ts` | No changes needed (generic apiFetch handles it) |

### 6.6 Data Fetching

Use TanStack Query:
- `useQuery(['servers', filters])` for the server list
- `useMutation` for create, delete, batch-delete with query invalidation

## 7. Telemetry Bar Update

The top bar currently shows hardcoded "0 Online / 0 Offline". Update it to show live counts from the server list query:
- `● {onlineCount} Online` (green)
- `● {offlineCount} Offline` (red)
- If any pending: `● {pendingCount} Pending` (amber)

The "+ Add Server" button moves from the telemetry bar to the dashboard page header (it's context-specific, not global).

## 8. Audit Events

New audit actions logged:
- `server.created` — admin created a server
- `server.updated` — admin updated server name/labels
- `server.deleted` — admin deleted a server
- `server.batch_deleted` — admin mass-deleted servers
- `server.registered` — agent registered with valid token

## 9. Out of Scope

- Agent binary and gRPC communication (sub-project 3)
- Heartbeat and online/offline transitions (sub-project 3)
- Server detail page content (sub-project 4+)
- Permissions management UI (deferred)
- `install.sh` script (built with agent in sub-project 3 — the URL is generated but the script doesn't exist yet)
