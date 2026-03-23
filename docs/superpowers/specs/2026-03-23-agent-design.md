# AeroDocs Sub-project 3: Agent — Design Spec

**Date:** 2026-03-23
**Status:** Approved
**Scope:** Go agent binary, gRPC protocol definition, Hub-side gRPC server with connection manager, heartbeat/online status, install script, agent binary distribution

## 1. Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Connection direction | Agent connects to Hub | Works behind NAT/firewalls. Hub doesn't need to reach agents. |
| Heartbeat mechanism | Bidirectional gRPC stream | Instant offline detection. Same connection carries future commands for file tree, log tailing, uploads. Avoids building a separate stream later. |
| Agent scope | Stream + heartbeat + proto stubs | Fully deployable agent that shows online/offline. Proto stubs defined for sub-projects 4-6 so they don't need to modify the .proto. |
| Install method | Download binary + register + systemd | One curl command: downloads agent, registers with Hub, installs as systemd service. |
| Binary distribution | Hub serves binary | Agent binary served at `/install/agent-linux-amd64`. Self-contained, no external dependencies. GitHub Releases backlogged for future. |

## 2. gRPC Protocol Definition

File: `proto/aerodocs/v1/agent.proto`

```protobuf
syntax = "proto3";
package aerodocs.v1;
option go_package = "github.com/wyiu/aerodocs/proto/aerodocs/v1";

service AgentService {
  // Bidirectional stream — agent connects, Hub sends commands, agent responds
  rpc Connect(stream AgentMessage) returns (stream HubMessage);
}

// Messages from Agent → Hub
message AgentMessage {
  oneof payload {
    Heartbeat heartbeat = 1;
    RegisterAgent register = 2;
    // Stubs for sub-projects 4-6
    FileListResponse file_list_response = 10;
    LogStreamChunk log_stream_chunk = 11;
    FileUploadAck file_upload_ack = 12;
  }
}

// Messages from Hub → Agent
message HubMessage {
  oneof payload {
    HeartbeatAck heartbeat_ack = 1;
    RegisterAck register_ack = 2;
    // Stubs for sub-projects 4-6
    FileListRequest file_list_request = 10;
    LogStreamRequest log_stream_request = 11;
    FileUploadRequest file_upload_request = 12;
  }
}

message Heartbeat {
  string server_id = 1;
  int64 timestamp = 2;
  SystemInfo system_info = 3;
}

message HeartbeatAck {
  int64 timestamp = 1;
}

message RegisterAgent {
  string token = 1;
  string hostname = 2;
  string ip_address = 3;
  string os = 4;
  string agent_version = 5;
}

message RegisterAck {
  bool success = 1;
  string server_id = 2;
  string error = 3;
}

message SystemInfo {
  double cpu_percent = 1;
  double memory_percent = 2;
  double disk_percent = 3;
  int64 uptime_seconds = 4;
}

// Stubs — defined now, implemented in sub-projects 4-6
message FileListRequest { string path = 1; }
message FileListResponse { repeated FileNode files = 1; }
message FileNode {
  string name = 1;
  string path = 2;
  bool is_dir = 3;
  int64 size = 4;
  bool readable = 5;
}
message LogStreamRequest { string path = 1; int64 offset = 2; string grep = 3; }
message LogStreamChunk { bytes data = 1; int64 offset = 2; }
message FileUploadRequest { string path = 1; bytes chunk = 2; bool done = 3; }
message FileUploadAck { bool success = 1; string error = 2; }
```

### 2.1 Protocol Flow

**First connection (registration):**
```
Agent opens Connect stream
→ Agent sends: AgentMessage { register: { token, hostname, ip, os, version } }
← Hub validates token, activates server in DB
← Hub sends: HubMessage { register_ack: { success: true, server_id: "uuid" } }
→ Agent saves server_id to /etc/aerodocs/agent.conf
→ Agent enters heartbeat loop
```

**Reconnection (known agent):**
```
Agent opens Connect stream
→ Agent sends: AgentMessage { heartbeat: { server_id: "uuid", timestamp, system_info } }
← Hub verifies server_id exists, registers connection
← Hub sends: HubMessage { heartbeat_ack: { timestamp } }
→ Agent continues heartbeat loop
```

**Heartbeat loop:**
```
Every 10 seconds:
→ Agent sends: AgentMessage { heartbeat: { server_id, timestamp, system_info } }
← Hub updates last_seen_at, stores system_info
← Hub sends: HubMessage { heartbeat_ack: { timestamp } }
```

**Disconnect detection:**
```
Stream breaks (network, agent crash, agent stop)
→ Hub's Connect handler returns
→ Hub unregisters from ConnManager
→ Hub's heartbeat monitor (every 15s) marks server offline
```

## 3. Agent Architecture

### 3.1 Project Structure

```
agent/
├── cmd/
│   └── aerodocs-agent/
│       └── main.go          # Entry point: parse flags, connect to Hub
├── internal/
│   ├── client/
│   │   └── client.go        # gRPC stream client: Connect(), send/receive loop, reconnect
│   ├── heartbeat/
│   │   └── heartbeat.go     # Periodic heartbeat sender with system metrics
│   └── sysinfo/
│       └── sysinfo.go       # Collect CPU, memory, disk, OS info
├── go.mod
└── go.sum
```

### 3.2 Agent Lifecycle

1. Agent starts with `--hub <url> --token <token>` (first run) or reads `/etc/aerodocs/agent.conf` (subsequent runs)
2. Dials Hub's gRPC endpoint (default port 9090)
3. Opens `Connect` bidirectional stream
4. First run: sends `RegisterAgent` with token. Hub validates, sends `RegisterAck` with `server_id`. Agent saves `server_id` to config file.
5. Subsequent runs: sends `Heartbeat` with `server_id` from config file
6. Enters heartbeat loop: sends `Heartbeat` every 10s, listens for commands
7. On disconnect: exponential backoff reconnect (1s, 2s, 4s, 8s, ... capped at 60s)
8. On SIGINT/SIGTERM: graceful shutdown, closes stream

### 3.3 Agent Config File

Location: `/etc/aerodocs/agent.conf`

```json
{
  "server_id": "uuid-from-hub",
  "hub_url": "aerodocs.yiucloud.com:9090"
}
```

Written after first successful registration. On subsequent starts, agent reads this file. If file exists, `--token` flag is ignored.

### 3.4 Agent CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--hub` | (required on first run) | Hub gRPC address (e.g., `aerodocs.yiucloud.com:9090`) |
| `--token` | (required on first run) | One-time registration token from Hub |
| `--config` | `/etc/aerodocs/agent.conf` | Path to config file |

### 3.5 Agent Dependencies

| Library | Purpose |
|---------|---------|
| `google.golang.org/grpc` | gRPC client |
| Generated proto code | From `proto/aerodocs/v1/agent.proto` |
| `runtime` (stdlib) | CPU count, memory stats |
| `os` (stdlib) | Hostname, disk usage |

## 4. Hub-side gRPC Server

### 4.1 New Hub Packages

```
hub/internal/
├── grpcserver/
│   ├── server.go         # gRPC server: Listen, register AgentService
│   └── handler.go        # Connect stream handler: registration, heartbeat, dispatch
├── connmgr/
│   └── connmgr.go        # Connection manager: track active agent streams
```

### 4.2 Connection Manager

```go
type ConnManager struct {
    mu      sync.RWMutex
    streams map[string]*AgentConn  // server_id → active connection
}

type AgentConn struct {
    ServerID  string
    Stream    pb.AgentService_ConnectServer
    LastSeen  time.Time
}
```

Methods:
- `Register(serverID string, stream pb.AgentService_ConnectServer)` — add connected agent
- `Unregister(serverID string)` — remove on disconnect
- `GetConn(serverID string) *AgentConn` — look up stream to send commands (used by sub-projects 4-6)
- `ActiveServerIDs() []string` — list currently connected servers
- `UpdateHeartbeat(serverID string)` — update LastSeen timestamp

### 4.3 New Store Methods Required

| Method | Description |
|--------|-------------|
| `UpdateServerStatus(id, status string) error` | Set server status to `online` or `offline` |
| `UpdateServerLastSeen(id string, systemInfo *model.SystemInfo) error` | Update `last_seen_at` and optionally store system metrics |
| `GetOnlineServersNotIn(activeIDs []string) ([]model.Server, error)` | Find servers marked `online` in DB but not in ConnManager (stale) |

### 4.4 REST Registration Endpoint

The existing `POST /api/servers/register` REST endpoint (in `handlers_servers.go`) is **removed** — registration now happens exclusively via the gRPC `Connect` stream. This avoids two registration paths that could drift.

### 4.5 Connect Stream Handler

1. Receive first message from agent
2. If `RegisterAgent`:
   - Hash token with SHA-256
   - Look up server by token hash in store
   - Validate token not expired
   - Call `store.ActivateServer()` (sets status to `online`, populates hostname/IP/OS)
   - Clear registration token from DB
   - Send `RegisterAck` with `server_id`
   - Register in ConnManager
3. If `Heartbeat` (reconnecting agent):
   - Verify `server_id` exists in store
   - Update server status to `online` if it was `offline`
   - Register in ConnManager
   - Send `HeartbeatAck`
4. Enter receive loop:
   - On `Heartbeat`: update `last_seen_at` in DB, update ConnManager, send `HeartbeatAck`
   - On other messages: dispatch to appropriate handler (no-op for stubs)
5. On stream error: unregister from ConnManager

### 4.4 Heartbeat Monitor

Background goroutine started with the gRPC server:
- Runs every 15 seconds
- For each entry in ConnManager:
  - If `LastSeen > 30s ago`: unregister, update server status to `offline` in DB
- Also checks servers with status `online` in DB that are NOT in ConnManager (crashed without clean disconnect)

### 4.5 Hub Startup Changes

- `main.go`: add `--grpc-addr` flag (default `:9090`)
- Start gRPC server alongside HTTP server
- Pass `store` and `ConnManager` to both
- Graceful shutdown: stop both servers on SIGINT/SIGTERM

### 4.6 Hub Dependencies (new)

| Library | Purpose |
|---------|---------|
| `google.golang.org/grpc` | gRPC server |
| `google.golang.org/protobuf` | Protobuf runtime |
| `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc` | Code generation (build-time) |

## 5. Install Script

### 5.1 Script Location

- Source: `hub/static/install.sh`
- Served by Hub at: `GET /install.sh`

### 5.2 Script Behavior

```bash
#!/bin/bash
# Usage: curl -sSL https://aerodocs.yiucloud.com/install.sh | sudo bash -s -- --token <TOKEN> --hub <HUB_URL>
```

Steps:
1. Parse `--token` and `--hub` arguments
2. Detect OS (`uname -s` → lowercase) and architecture (`uname -m` → `amd64`/`arm64`)
3. Extract Hub HTTP URL from `--hub` (strip port, use HTTPS)
4. Download agent binary: `curl -sSL https://<hub>/install/agent-<os>-<arch> -o /usr/local/bin/aerodocs-agent`
5. `chmod +x /usr/local/bin/aerodocs-agent`
6. Create config directory: `mkdir -p /etc/aerodocs`
7. Write systemd service file to `/etc/systemd/system/aerodocs-agent.service`:
   ```ini
   [Unit]
   Description=AeroDocs Agent
   After=network-online.target
   Wants=network-online.target

   [Service]
   Type=simple
   ExecStart=/usr/local/bin/aerodocs-agent --hub <HUB_GRPC_URL> --token <TOKEN>
   Restart=always
   RestartSec=5

   [Install]
   WantedBy=multi-user.target
   ```
8. `systemctl daemon-reload && systemctl enable --now aerodocs-agent`
9. Print success message

### 5.3 Hub Binary Serving

- `GET /install.sh` — serves install script from `hub/static/install.sh`
- `GET /install/agent-{os}-{arch}` — serves agent binary from `--agent-bin-dir` (default `./bin/`)
- Only `linux-amd64` and `linux-arm64` supported initially
- Returns 404 for unsupported OS/arch combinations

### 5.4 Hub Route Additions

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/install.sh` | None | Serve install script |
| GET | `/install/agent-{os}-{arch}` | None | Serve agent binary |

These are public — the install script runs on servers that don't have Hub credentials.

## 6. Makefile Changes

```makefile
# Add agent cross-compilation
build-agent:
	cd agent && GOOS=linux GOARCH=amd64 go build -o ../bin/aerodocs-agent-linux-amd64 ./cmd/aerodocs-agent/
	cd agent && GOOS=linux GOARCH=arm64 go build -o ../bin/aerodocs-agent-linux-arm64 ./cmd/aerodocs-agent/

# Add proto generation
proto:
	protoc --go_out=. --go-grpc_out=. proto/aerodocs/v1/agent.proto

# Updated build target
build: build-web embed-web proto build-agent build-hub
```

## 7. Proto Code Generation

Generated Go code goes into:
- `proto/aerodocs/v1/agent.pb.go` — message types
- `proto/aerodocs/v1/agent_grpc.pb.go` — gRPC service interfaces

Both `hub/` and `agent/` import these via a Go module replace directive or by placing generated code in a shared location. Simplest approach: generate into `proto/` directory, and both `hub/go.mod` and `agent/go.mod` use a `replace` directive to point to the local `proto/` module.

Proto module: `proto/go.mod` with module path `github.com/wyiu/aerodocs/proto`.

## 8. Server Status Flow

```
Server created (POST /api/servers) → status: "pending"
Agent connects with token → status: "online"
Agent disconnects → status: "offline" (within 30s)
Agent reconnects → status: "online"
Agent stopped gracefully → status: "offline" (immediate via stream close)
```

## 9. Audit Events

New audit actions:
- `server.connected` — agent established connection (includes server_id, IP)
- `server.disconnected` — agent stream closed (includes server_id)

## 10. Dashboard Integration

No frontend changes needed — the dashboard already shows server status from the DB. When the agent connects and the Hub updates the server's status to `online`, the dashboard's TanStack Query polling will pick it up automatically.

The telemetry bar fleet health counts will update as servers go online/offline.

## 11. Out of Scope

- File listing RPC implementation (sub-project 4)
- Log tailing RPC implementation (sub-project 5)
- File upload RPC implementation (sub-project 6)
- TLS for gRPC (can use Traefik for TLS termination, or add later)
- Agent auto-update mechanism
- Windows/macOS agent support
- GitHub Releases distribution (backlogged)
