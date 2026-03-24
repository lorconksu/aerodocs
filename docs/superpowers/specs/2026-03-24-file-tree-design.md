# AeroDocs Sub-project 4: File Tree — Design Spec

**Date:** 2026-03-24
**Status:** Approved
**Scope:** Remote file browsing through the agent gRPC stream, file content viewing with syntax highlighting and Markdown rendering, admin-configured path access control per user per server.

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Path access model | Admin-configured root paths per user per server | Users can only browse paths explicitly granted by an admin. No filesystem exposure beyond allowed roots. Admins have unrestricted access (root `/`). |
| Layout | Sidebar tree + content pane | IDE-like layout: expandable directory tree on the left, file viewer on the right. Server info in header bar. |
| File content viewing | Static view with manual refresh | Show file content with syntax highlighting. No live tailing — that's sub-project 5. Manual refresh button to re-fetch. |
| Proto approach | Separate FileReadRequest/Response | Clean separation between directory listing (FileListRequest) and file reading (FileReadRequest). |
| File size handling | Last 1MB for large files | Files under 1MB shown in full. Over 1MB: agent returns last 1MB (tail). Total size available from directory listing's FileNode.size. |
| Markdown rendering | Toggle raw/rendered | Markdown files get a toggle button to switch between syntax-highlighted source and rendered preview. |
| Path management UI | Server detail page | Admins manage allowed paths per user directly on the server detail page. |

## 2. Access Control

### Database

Reuse the existing `permissions` table (migration `005_create_permissions.sql`):

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
```

This table already exists and is used by `ListServersForUser` for server-level visibility. The `path` column (default `/`) now doubles as the filesystem root path for file browsing. Multiple rows per (user, server) pair allow granting access to multiple paths (e.g., `/var/log` and `/etc/nginx`).

### Rules

- Admins bypass path restrictions — they browse from `/` on any server
- Non-admin users can only browse paths that appear in `permissions` for their user_id and the target server_id
- All requested paths are validated on both Hub and Agent:
  - Hub checks the path is under an allowed root before forwarding to agent
  - Agent validates no `..` traversal or symlink escape beyond the requested path
- If a user has no paths configured for a server, the file browser shows an empty state: "No paths configured. Ask an admin to grant access."

## 3. Proto Changes

### Updated existing stubs

Add `request_id` and `error` fields to existing stub messages:

```protobuf
message FileListRequest {
    string request_id = 1;  // was: string path = 1
    string path = 2;        // was: (new field number)
}
```

**Important:** The existing stubs have never been used (they were placeholders). Since no agent or hub code references them yet, we can safely redefine them with new field numbers. The generated `.pb.go` files will be regenerated.

Updated definitions for all four messages:

```protobuf
message FileListRequest {
    string request_id = 1;
    string path = 2;
}

message FileListResponse {
    string request_id = 1;
    repeated FileNode files = 2;
    string error = 3;
}

message FileNode {
    string name = 1;
    string path = 2;
    bool is_dir = 3;
    int64 size = 4;
    bool readable = 5;
}

message FileReadRequest {
    string request_id = 1;
    string path = 2;
    int64 offset = 3;
    int64 limit = 4;       // max bytes to read (default/max: 1MB)
}

message FileReadResponse {
    string request_id = 1;
    bytes data = 2;
    int64 total_size = 3;
    string mime_type = 4;
    string error = 5;
}
```

### New oneof entries

Add to `AgentMessage` oneof:
```protobuf
FileReadResponse file_read_response = 13;
```

Add to `HubMessage` oneof:
```protobuf
FileReadRequest file_read_request = 13;
```

Existing oneof entries remain:
- `AgentMessage.file_list_response` (field 10)
- `HubMessage.file_list_request` (field 10)

## 4. Request-Response Correlation

The gRPC stream is bidirectional but not request-response by nature. To implement request-response semantics:

**Hub side (`hub/internal/grpcserver/handler.go`):**
- New `PendingRequests` struct: a `sync.Mutex`-protected map of `request_id → chan proto.Message`
- HTTP handler generates a UUID `request_id`, registers a channel in PendingRequests, sends the request via stream, waits on the channel with 10-second timeout
- The gRPC handler's receive loop matches incoming `FileListResponse`/`FileReadResponse` by `request_id` and delivers to the waiting channel
- On timeout: remove entry from map, return HTTP 504

**Concurrent Send safety (Hub):**
- Add a `sendMu sync.Mutex` to `AgentConn` in connmgr
- All `stream.Send()` calls (heartbeat acks from the gRPC handler loop, file requests from HTTP handlers) must acquire this mutex
- Alternative: use a send channel that the gRPC handler drains, but a mutex is simpler

**Concurrent Send safety (Agent):**
- The agent currently sends from a single `select` loop (heartbeat ticker)
- With file request handling, the receive goroutine will also need to send responses
- Route all sends through a single send channel: both heartbeat ticker and request handler push messages to the channel, the main loop drains it to `stream.Send()`

## 5. Hub HTTP API

### File listing

```
GET /api/servers/{id}/files?path=/var/log
Authorization: Bearer <token>

Response 200:
{
    "files": [
        { "name": "nginx", "path": "/var/log/nginx", "is_dir": true, "size": 4096, "readable": true },
        { "name": "syslog", "path": "/var/log/syslog", "is_dir": false, "size": 524288, "readable": true }
    ]
}

Response 403: { "error": "access denied" }
Response 404: { "error": "server not found" }
Response 502: { "error": "agent not connected" }
Response 504: { "error": "agent timeout" }
```

### File reading

```
GET /api/servers/{id}/files/read?path=/var/log/syslog
Authorization: Bearer <token>

Response 200:
{
    "data": "<base64-encoded content>",
    "total_size": 524288,
    "mime_type": "text/plain"
}

Response 403: { "error": "access denied" }
Response 502: { "error": "agent not connected" }
Response 504: { "error": "agent timeout" }
```

**Large file handling:** The Hub knows the file size from the directory listing's `FileNode.size`. If `size > 1MB`, the Hub sends `FileReadRequest` with `offset = size - 1048576, limit = 1048576` (last 1MB). The frontend displays a banner: "Showing last 1MB of {totalSize}". Hard cap: files over 10MB return HTTP 413.

### Path management (admin only)

```
GET /api/servers/{id}/paths
Response 200: { "paths": [{ "id": "...", "user_id": "...", "username": "...", "path": "/var/log", "created_at": "..." }] }

POST /api/servers/{id}/paths
Body: { "user_id": "...", "path": "/var/log" }
Response 201: { "id": "...", "user_id": "...", "path": "/var/log" }

DELETE /api/servers/{id}/paths/{path_id}
Response 204
```

## 6. Agent Side

### File browser package

New package `agent/internal/filebrowser/filebrowser.go`:

**ListDir(path string) (*pb.FileListResponse, error):**
- Calls `os.ReadDir`, builds FileNode list
- Sorts: directories first, then files, both alphabetical
- Sets `readable` by checking `os.Open` permission
- Validates path: resolves symlinks with `filepath.EvalSymlinks`, rejects `..` traversal

**ReadFile(path string, offset, limit int64) (*pb.FileReadResponse, error):**
- Opens file, seeks to offset, reads up to limit bytes
- Detects MIME type from extension (text/plain, application/json, text/markdown, etc.)
- Hard limit: refuses to read if limit > 1MB (1048576 bytes)
- Returns total_size from `os.Stat`
- Validates path same as listing

### Message dispatcher

Restructure `agent/internal/client/client.go`:

Current architecture: receive goroutine discards messages, main loop only sends heartbeats.

New architecture:
- Receive goroutine reads `HubMessage` and dispatches by type to a handler
- Handler processes `FileListRequest`/`FileReadRequest`, calls filebrowser, builds response
- Handler pushes response `AgentMessage` to a unified send channel
- Heartbeat ticker also pushes to the same send channel
- Main loop drains send channel to `stream.Send()` (single writer, no concurrent sends)

## 7. Frontend

### Server detail page

New file: `web/src/pages/server-detail.tsx` (replace placeholder in `App.tsx` route `/servers/:id`)

**Header bar:**
- Status dot + server name
- OS, IP address, agent version
- Last seen timestamp (auto-refreshes with the 10s polling from dashboard)

**Left sidebar (tree view):**
- Top-level entries are the user's allowed root paths (from `permissions` table)
- For admins: single root entry `/`
- Click a directory to expand (lazy fetch via `GET /api/servers/{id}/files?path=...`)
- Click a file to open in the viewer pane
- Current file highlighted in tree
- Loading spinner on directory expand

**Right pane (file viewer):**
- Breadcrumb path at the top
- File metadata: size, MIME type
- Syntax-highlighted content (highlight.js — lightweight, widely supported)
- For files > 1MB: banner "Showing last 1 MB of {totalSize}"
- Refresh button to re-fetch content
- Markdown files (`.md`): toggle button "Raw / Rendered"
  - Raw: syntax-highlighted markdown source
  - Rendered: rendered using react-markdown
- Empty state when no file selected: "Select a file to view its contents"

**Empty states:**
- Server offline: "Server is offline. File browsing is unavailable."
- No paths configured (non-admin): "No paths configured. Ask an admin to grant access."
- Directory empty: "This directory is empty."
- File not readable: "Permission denied."
- Agent timeout: "Agent did not respond. Try again." with retry button.

### Path management (admin only, on server detail page)

Below the file browser, a collapsible "Manage File Access" section:
- Table: username, path, actions (delete)
- "Add Path" form: user dropdown + path text input
- Only visible to admins

## 8. Audit Events

New audit event constants:

```go
AuditFileRead      = "file.read"       // user read a file via the file browser
AuditPathGranted   = "path.granted"    // admin granted a user access to a path
AuditPathRevoked   = "path.revoked"    // admin revoked a user's path access
```

- `file.read`: logged on each `GET /api/servers/{id}/files/read` call, target = server_id, detail = file path
- `path.granted`: logged on `POST /api/servers/{id}/paths`, target = server_id, detail = user_id + path
- `path.revoked`: logged on `DELETE /api/servers/{id}/paths/{path_id}`, target = server_id, detail = user_id + path

## 9. Error Handling

| Scenario | Behavior |
|----------|----------|
| Agent offline | HTTP 502 "agent not connected" — frontend shows offline state |
| Agent timeout (>10s) | HTTP 504 "agent timeout" — frontend shows error with retry |
| Path not allowed | HTTP 403 — frontend shows access denied |
| File not readable | Agent returns error in FileReadResponse.error — Hub returns HTTP 403 |
| File too large (>10MB) | HTTP 413 — frontend shows "file too large for viewing" |
| Directory not found | Agent returns error in FileListResponse.error — Hub returns HTTP 404 |
| Path traversal attempt | Both Hub and Agent reject — HTTP 403 |
