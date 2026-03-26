# AeroDocs Architecture

> **TL;DR**
> - **What:** Hub-and-Spoke architecture — one central Go server (Hub) + lightweight agents on each managed server
> - **Who:** DevOps teams, sysadmins, home lab operators managing Linux server fleets
> - **Why:** Provides structured, auditable remote access without direct SSH; single binary, no external dependencies
> - **Where:** Hub runs on any Linux server; agents deployed on each managed machine; frontend embedded in Hub binary
> - **When:** Agents connect on startup via persistent gRPC stream; Hub enforces auth/authz on every request
> - **How:** Browser → REST API → Hub → gRPC → Agent; SQLite for persistence; JWT + TOTP for auth

## Hub-and-Spoke Model

AeroDocs is built on a Hub-and-Spoke architecture. There is one central Hub server that all users interact with, and a lightweight Agent binary deployed on each managed server. Users never communicate directly with agents — everything flows through the Hub.

```mermaid
graph TD
    Browser["Browser (React SPA)"]
    Hub["AeroDocs Hub\n(Go binary)"]
    DB[(SQLite)]
    A1["Agent\nServer 1"]
    A2["Agent\nServer 2"]
    AN["Agent\nServer N"]

    Browser -->|"HTTPS REST + WebSocket"| Hub
    Hub --- DB
    Hub -->|"gRPC (mTLS)"| A1
    Hub -->|"gRPC (mTLS)"| A2
    Hub -->|"gRPC (mTLS)"| AN
```

**Hub** — The single source of truth. It serves the React SPA, enforces authentication and authorization, persists all state in SQLite, and proxies operations to agents via gRPC.

**Agent** — A minimal binary installed on each remote server. It exposes a gRPC interface and executes only what the Hub instructs. Agents have no web interface, no user accounts, and no direct user access.

**Frontend** — A React SPA compiled by Vite and embedded into the Hub binary via `go:embed`. The production deployment is a single self-contained binary.

---

## Request Flow

```mermaid
sequenceDiagram
    participant Browser
    participant Traefik
    participant Hub
    participant SQLite
    participant Agent

    Browser->>Traefik: HTTPS request
    Traefik->>Hub: Proxy to :8080
    Hub->>Hub: Validate JWT (middleware)
    alt DB-only operation
        Hub->>SQLite: Query/write
        SQLite-->>Hub: Result
    else Agent operation
        Hub->>Agent: gRPC call
        Agent-->>Hub: Response
    end
    Hub-->>Browser: JSON response
```

For API requests:
1. The browser sends an HTTPS request to Traefik.
2. Traefik proxies the request to the Hub on `localhost:8080`.
3. The Hub's auth middleware validates the JWT from the `Authorization: Bearer` header.
4. The handler executes the business logic (SQLite queries, or gRPC calls to agents).
5. The Hub returns a JSON response.

For the SPA:
- The Hub's catch-all handler serves `index.html` (embedded in the binary) for any path that doesn't match an API route. React Router handles client-side navigation.

---

## Package Structure

All Hub server code lives under `hub/internal/`:

```
hub/
├── cmd/
│   └── aerodocs/
│       ├── main.go       # Entrypoint: parses flags, wires up dependencies
│       └── admin.go      # CLI admin subcommands (e.g. reset-totp)
├── embed.go              # go:embed directive for web/dist
└── internal/
    ├── auth/             # JWT generation/validation, bcrypt, TOTP
    ├── migrate/          # Schema migration runner + SQL migration files
    ├── model/            # Shared request/response structs and constants
    ├── server/           # HTTP server, routing, handlers, middleware
    └── store/            # SQLite data access layer
```

### Package responsibilities

| Package | Responsibility |
|---------|---------------|
| `auth` | `GenerateTokenPair`, `ValidateToken`, `HashPassword`, `CheckPassword`, TOTP secret generation and code verification |
| `migrate` | Embeds `migrations/*.sql`, runs unapplied migrations in filename order, records each in `_migrations` table |
| `model` | Plain Go structs for all domain types (`User`, `Server`, `AuditEntry`) and all HTTP request/response bodies. No business logic. |
| `server` | `Server` struct, route registration, all HTTP handlers, auth middleware, rate limiter, SPA handler |
| `store` | `Store` struct wrapping `*sql.DB`. All SQL queries. Returns model types. No HTTP awareness. |

---

## Agent Architecture

Agents connect to the Hub via a **persistent bidirectional gRPC stream** — the agent dials out, not the Hub. This means agents work behind NAT and firewalls without requiring inbound port forwarding.

### Proto definition

The service contract is defined at `proto/aerodocs/v1/agent.proto`. The single RPC is:

```proto
service AgentService {
  rpc Connect(stream AgentMessage) returns (stream HubMessage) {}
}
```

The stream carries a union of typed messages in both directions:

**Agent → Hub message types:**

| Message | Purpose |
|---------|---------|
| `Heartbeat` | Periodic liveness ping (every 15 seconds) |
| `Register` | Initial registration with token + sysinfo |
| `FileListResponse` | Directory listing result (response to hub request) |
| `FileReadResponse` | File content result (response to hub request) |
| `LogStreamChunk` | One chunk of tailed log output |
| `LogStreamStop` | Signals the agent stopped tailing (EOF or error) |
| `FileUploadAck` | Acknowledgement of a received file chunk |
| `FileDeleteResponse` | Result of a file deletion request |
| `UnregisterAck` | Confirms that self-cleanup (binary, config, dropzone removal) is complete |

**Hub → Agent message types:**

| Message | Purpose |
|---------|---------|
| `HeartbeatAck` | Acknowledges a heartbeat |
| `RegisterAck` | Confirms successful registration |
| `FileListRequest` | Request a directory listing at a given path |
| `FileReadRequest` | Request file content at a given path |
| `LogStreamRequest` | Start tailing a file (with optional grep filter) |
| `FileUploadRequest` | Send a file chunk to the agent (chunked transfer) |
| `FileDeleteRequest` | Request deletion of a file |
| `UnregisterRequest` | Instructs the agent to stop its service and remove its binary, config, and dropzone |

### Request-response correlation

All request messages carry a `request_id` field (UUID). The Hub stores an in-flight callback in a `PendingRequests` map (`map[string]chan proto.Message`) keyed by `request_id`. When the agent sends a response with the matching `request_id`, the Hub resolves the pending channel and unblocks the waiting HTTP handler.

### Heartbeat monitor

The Hub runs a heartbeat monitor that:
- Ticks every **15 seconds**
- Marks a connected agent as stale if its last heartbeat was more than **30 seconds** ago
- Sets the server status to `offline` in the database for stale agents

### Agent reconnect behavior

The agent reconnects with **exponential backoff** — starting at 1 second and capping at 60 seconds. On reconnect, it re-sends a `Register` message so the Hub can update sysinfo and reset the connection state.

---

## Agent Package Structure

The agent binary is a separate Go module under `agent/`:

```
agent/
├── cmd/
│   └── aerodocs-agent/
│       └── main.go           # Entry point: flag parsing, config load/save, wiring
└── internal/
    ├── client/               # gRPC stream client with reconnect backoff
    ├── heartbeat/            # Periodic heartbeat sender (15s interval)
    ├── sysinfo/              # CPU, memory, disk, and uptime collection
    ├── filebrowser/          # Directory listing and file reading
    ├── logtailer/            # Poll-based file tailing with optional grep filter
    └── dropzone/             # Chunked file upload receiver
```

### Agent package responsibilities

| Package | Responsibility |
|---------|---------------|
| `cmd/aerodocs-agent` | Parses `--hub` and `--token` flags, loads/saves `/etc/aerodocs/agent.conf`, creates and wires all components |
| `client` | Opens the bidirectional gRPC stream, handles all incoming `HubMessage` types by dispatching to the appropriate internal package, manages reconnect backoff |
| `heartbeat` | Sends a `Heartbeat` message to the hub on a 15-second ticker |
| `sysinfo` | Collects CPU usage, memory usage, disk usage, and system uptime; populates the `Register` message |
| `filebrowser` | Handles `FileListRequest` (directory listing) and `FileReadRequest` (file content, base64-encoded) |
| `logtailer` | Handles `LogStreamRequest` — polls the target file for new lines and streams `LogStreamChunk` messages; supports optional grep filtering |
| `dropzone` | Handles `FileUploadRequest` — receives chunked file transfers and writes them to a local staging directory |

### Agent configuration

On first run the agent stores its assigned `server_id` and `hub_url` to `/etc/aerodocs/agent.conf` (JSON). On subsequent runs it loads from this file, skipping re-registration if already registered.

---

## Hub gRPC Packages

Two new packages under `hub/internal/` support the agent gRPC layer:

```
hub/internal/
├── grpcserver/    # gRPC server, Connect handler, PendingRequests map, LogSessions map
└── connmgr/       # Connection manager — tracks active agent streams, SendMu for concurrent write safety
```

### Package responsibilities

| Package | Responsibility |
|---------|---------------|
| `grpcserver` | Implements `AgentService.Connect` — reads incoming `AgentMessage` frames, resolves pending requests, manages log sessions, and dispatches heartbeats. Owns the `PendingRequests map[string]chan proto.Message` and `LogSessions map[string]context.CancelFunc`. |
| `connmgr` | Stores the active gRPC send stream for each connected server. Wraps sends with a `sync.Mutex` (`SendMu`) to prevent concurrent write races on the stream. Provides `SendToServer(serverID, msg)` used by HTTP handlers to forward requests to agents. |

---

## Authentication Flow

AeroDocs uses four distinct JWT token types, each valid for a limited time and accepted only at specific endpoints.

```mermaid
sequenceDiagram
    participant Browser
    participant Hub

    Browser->>Hub: POST /api/auth/login (username + password)
    Hub-->>Browser: totp_token (60s) OR setup_token (10m)

    alt User has TOTP enabled
        Browser->>Hub: POST /api/auth/login/totp (totp_token + TOTP code)
        Hub-->>Browser: access_token (15m) + refresh_token (7d)
    else First login — TOTP not yet configured
        Browser->>Hub: POST /api/auth/totp/setup (setup_token)
        Hub-->>Browser: TOTP secret + QR URL
        Browser->>Hub: POST /api/auth/totp/enable (setup_token + TOTP code)
        Hub-->>Browser: access_token (15m) + refresh_token (7d)
    end

    Note over Browser,Hub: All subsequent API calls use access_token

    Browser->>Hub: POST /api/auth/refresh (refresh_token)
    Hub-->>Browser: new access_token + new refresh_token
```

### Token types

| Type | Expiry | Accepted at | Purpose |
|------|--------|-------------|---------|
| `access` | 15 minutes | All protected API endpoints | Normal API access |
| `refresh` | 7 days | `POST /api/auth/refresh` only | Silent token renewal |
| `totp` | 60 seconds | `POST /api/auth/login/totp` only | Short-lived bridge between password auth and TOTP verification |
| `setup` | 10 minutes | `POST /api/auth/totp/setup` and `POST /api/auth/totp/enable` only | One-time TOTP enrollment flow |

The middleware enforces token type — passing a refresh token to a protected endpoint returns 401, even if the signature is valid.

**Mandatory 2FA**: Every user must complete TOTP setup before receiving an access token. There is no opt-out path.

---

## Database Schema

AeroDocs uses SQLite with WAL mode and foreign key enforcement. Migrations run automatically on startup via the `migrate` package.

### `users`
Stores all Hub user accounts.

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT PK | UUID |
| `username` | TEXT UNIQUE | Login name |
| `email` | TEXT UNIQUE | |
| `password_hash` | TEXT | bcrypt cost 12 |
| `role` | TEXT | `admin` or `viewer` |
| `totp_secret` | TEXT | Nullable; encrypted TOTP seed |
| `totp_enabled` | INTEGER | 0 or 1 |
| `avatar` | TEXT | Nullable; base64 data URL |
| `created_at` | TEXT | ISO 8601 |
| `updated_at` | TEXT | ISO 8601 |

### `audit_logs`
Immutable record of all actions. Rows are never updated or deleted.

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT PK | UUID |
| `user_id` | TEXT FK | Nullable (system actions) |
| `action` | TEXT | Dot-notation constant (e.g. `user.login`) |
| `target` | TEXT | Nullable; the affected resource |
| `detail` | TEXT | Nullable; human-readable context |
| `ip_address` | TEXT | Nullable |
| `created_at` | TEXT | ISO 8601 |

Indexed on `user_id`, `action`, and `created_at` for filtered queries.

Notable audit actions related to server lifecycle:

| Action | Trigger |
|--------|---------|
| `server.created` | Admin creates a server record |
| `server.updated` | Admin edits name or labels |
| `server.registered` | Agent completes initial registration |
| `server.connected` | Agent establishes a live gRPC stream |
| `server.disconnected` | Agent's gRPC stream drops |
| `server.unregistered` | Admin (or agent re-install) unregisters a server — cleanup command sent to agent, then record deleted from DB |

### `_config`
Key-value store for internal Hub configuration (e.g. the JWT signing key).

| Column | Type |
|--------|------|
| `key` | TEXT PK |
| `value` | TEXT |

### `servers`
Registry of all managed servers.

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT PK | UUID |
| `name` | TEXT | Display name |
| `hostname` | TEXT | Nullable; set by agent on registration |
| `ip_address` | TEXT | Nullable; set by agent on registration |
| `os` | TEXT | Nullable; set by agent on registration |
| `status` | TEXT | `pending`, `online`, or `offline` |
| `registration_token` | TEXT UNIQUE | Nullable; single-use token for agent registration |
| `token_expires_at` | TEXT | Nullable |
| `agent_version` | TEXT | Nullable |
| `labels` | TEXT | JSON object string |
| `last_seen_at` | TEXT | Nullable |
| `created_at` | TEXT | ISO 8601 |
| `updated_at` | TEXT | ISO 8601 |

### `permissions`
Per-user, per-server, per-path access grants for Viewer role scoping.

| Column | Type | Notes |
|--------|------|-------|
| `id` | TEXT PK | UUID |
| `user_id` | TEXT FK | References `users(id)` CASCADE |
| `server_id` | TEXT FK | References `servers(id)` CASCADE |
| `path` | TEXT | Filesystem path prefix (default `/`) |
| `created_at` | TEXT | ISO 8601 |

Unique constraint on `(user_id, server_id, path)`.

### `_migrations`
Internal migration tracking table. Managed by the `migrate` package.

| Column | Type |
|--------|------|
| `id` | INTEGER PK AUTOINCREMENT |
| `filename` | TEXT UNIQUE |
| `applied_at` | TEXT |

---

## API Endpoint Reference

### Auth endpoints

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| GET | `/api/auth/status` | None | Returns `{"initialized": bool}` |
| POST | `/api/auth/register` | None (rate-limited) | Create first admin user (disabled once any user exists) |
| POST | `/api/auth/login` | None (rate-limited) | Password login; returns `totp_token` or `setup_token` |
| POST | `/api/auth/login/totp` | `totp` token (rate-limited) | Complete TOTP login; returns access + refresh tokens |
| POST | `/api/auth/refresh` | `refresh` token | Exchange refresh token for new token pair |
| POST | `/api/auth/totp/setup` | `setup` token | Generate TOTP secret + QR URL |
| POST | `/api/auth/totp/enable` | `setup` token | Verify TOTP code and activate 2FA |
| GET | `/api/auth/me` | `access` token | Return current user profile |
| PUT | `/api/auth/password` | `access` token | Change own password |
| PUT | `/api/auth/avatar` | `access` token | Update own avatar (base64 data URL) |
| POST | `/api/auth/totp/disable` | `access` token + admin | Disable another user's TOTP (requires admin TOTP code) |

### User management endpoints (admin only)

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| GET | `/api/users` | `access` + admin | List all users |
| POST | `/api/users` | `access` + admin | Create a new user (returns temporary password) |
| PUT | `/api/users/{id}/role` | `access` + admin | Update a user's role |
| DELETE | `/api/users/{id}` | `access` + admin | Delete a user |

### Audit log endpoints (admin only)

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| GET | `/api/audit-logs` | `access` + admin | List audit log entries (filterable by user, action, date range) |

### Server endpoints

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| GET | `/api/servers` | `access` | List servers (all users) |
| POST | `/api/servers` | `access` + admin | Create a server record + registration token |
| GET | `/api/servers/{id}` | `access` | Get a single server |
| PUT | `/api/servers/{id}` | `access` + admin | Update server name/labels |
| DELETE | `/api/servers/{id}` | `access` + admin | Delete a server |
| POST | `/api/servers/batch-delete` | `access` + admin | Delete multiple servers by ID array |
| POST | `/api/servers/register` | None | Agent self-registration using a registration token |

### Agent operation endpoints

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| GET | `/api/servers/{id}/files` | `access` | List directory contents (`?path=` query param) |
| GET | `/api/servers/{id}/files/read` | `access` | Read file content as base64 (`?path=` query param) |
| GET | `/api/servers/{id}/logs/tail` | `access` | SSE log streaming (`?path=&grep=` query params) |
| POST | `/api/servers/{id}/upload` | `access` + admin | Multipart file upload to server dropzone |
| GET | `/api/servers/{id}/dropzone` | `access` + admin | List files in server dropzone |
| DELETE | `/api/servers/{id}/dropzone` | `access` + admin | Delete a dropped file (`?filename=` query param) |
| GET | `/api/servers/{id}/paths` | `access` + admin | List all path permissions for a server |
| POST | `/api/servers/{id}/paths` | `access` + admin | Grant a user path access on a server |
| DELETE | `/api/servers/{id}/paths/{pathId}` | `access` + admin | Revoke a path permission |
| GET | `/api/servers/{id}/my-paths` | `access` | List the current user's allowed paths on a server |
| GET | `/install.sh` | None | Agent install script (one-command install) |
| GET | `/install/{os}/{arch}` | None | Download agent binary for the specified OS and architecture |

### Server removal endpoints

Server removal is handled through **Unregister** rather than a plain delete. Two endpoints cover different scenarios:

| Method | Path | Auth required | Description |
|--------|------|--------------|-------------|
| DELETE | `/api/servers/{id}/unregister` | `access` + admin | Send cleanup command to agent (stop service, remove binary, config, and dropzone), then delete from DB. If agent is offline, deletes from DB only. |
| DELETE | `/api/servers/{id}/self-unregister` | None (public) | Called by the agent install script during re-install to remove the old server entry from the Hub before registering a fresh one. |

> **Note:** The older `DELETE /api/servers/{id}` endpoint performed a plain database delete with no agent cleanup. Server removal is now exclusively via `/unregister` (admin-initiated) or `/self-unregister` (agent-initiated during re-install). The `POST /api/servers/batch-delete` endpoint has been replaced by batch unregister behaviour on the dashboard.
