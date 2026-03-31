# AeroDocs

Self-hosted infrastructure observability platform. Hub-and-Spoke architecture: Go backend + React frontend (Hub) with lightweight Go agents on remote servers. Current version: **v1.2.11**.

## Project Structure

```
aerodocs/
├── hub/              # Go backend (REST API, gRPC server, SQLite, auth, embedded frontend)
│   ├── cmd/aerodocs/ # Main entry point
│   └── internal/     # Core packages (auth, handlers, store, grpc, email, middleware)
├── agent/            # Go agent binary (runs on remote servers, connects to Hub via gRPC)
│   ├── cmd/aerodocs-agent/
│   └── internal/     # Agent packages (filebrowser, logtailer, uploader)
├── web/              # React 19 + TypeScript + Vite + Tailwind frontend
│   └── src/          # Components, hooks, pages, API client
├── proto/            # Protocol Buffer definitions (gRPC service)
│   └── aerodocs/v1/  # agent.proto
├── docs/
│   ├── engineering/  # Architecture, API reference, deployment, security model, gRPC protocol
│   └── wiki/         # End-user documentation and screenshots
├── scripts/          # Build and deployment scripts
├── Dockerfile        # Multi-stage build (frontend + hub + agent)
├── docker-compose.yml
└── Makefile          # Build orchestration
```

## Build Commands

```bash
make build            # Full production build (frontend + hub + agent)
make build-web        # cd web && npm run build
make build-hub        # cd hub && go build -o ../bin/aerodocs ./cmd/aerodocs/
make build-agent      # Cross-compile agent for linux/amd64 and linux/arm64
make proto            # Regenerate protobuf Go code
make clean            # Remove build artifacts
```

## Test Commands

```bash
make test             # Run all tests (hub + agent)
make test-hub         # cd hub && go test ./...
make test-agent       # cd agent && go test ./...
cd web && npx vitest run  # Frontend unit tests
```

## Development

```bash
make dev-hub          # Run Hub in dev mode (port 8080, hot reload)
make dev-web          # Run Vite dev server (proxies to Hub)
```

## Key Technologies

- **Backend**: Go 1.26+, SQLite (WAL mode, pure Go via modernc.org/sqlite)
- **Frontend**: React 19, TypeScript, Vite, Tailwind CSS v4, shadcn/ui, TanStack Query
- **Communication**: gRPC bidirectional streaming with mTLS (ECDSA P-256), REST + SSE
- **Auth**: JWT (httpOnly cookies), TOTP 2FA (mandatory), bcrypt cost 12, CSRF double-submit

## Deployment

- Docker image: `yiucloud/aerodocs:1.2.11`
- Ports: 8081 (HTTP), 9090 (gRPC)
- Frontend is embedded into the Go binary via `go:embed` (single binary, no Node.js runtime needed)
- Production: LXC 110 on proxmox3, behind Traefik (TLS termination), accessible at aerodocs.yiucloud.com

## Database

SQLite with WAL mode. Schema auto-migrates on startup. Tables: `users`, `servers`, `permissions`, `audit_logs`, `notification_settings`, `sessions`.
