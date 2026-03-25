# AeroDocs Deployment Guide

## Prerequisites

- Go 1.26+
- Node.js 25+
- Make
- A Linux server (amd64 or arm64)
- A domain name pointing to your server
- Traefik v2+ as a reverse proxy (recommended)

---

## Building from Source

```bash
git clone https://github.com/wyiu/aerodocs.git
cd aerodocs
make build
```

The `make build` target runs three steps in sequence:

1. `build-web` — Runs `npm run build` inside `web/`, producing `web/dist/`
2. `embed-web` — Copies `web/dist/` to `hub/web/dist/` so it's picked up by `go:embed`
3. `build-hub` — Compiles `hub/cmd/aerodocs/` into `bin/aerodocs`

The output is a single self-contained binary at `bin/aerodocs`. No Node.js, no separate web server, no external dependencies.

---

## Running the Binary

```bash
./bin/aerodocs [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `:8080` | HTTP listen address and port |
| `--grpc-addr` | `:9090` | gRPC listen address for agent connections |
| `--db` | `aerodocs.db` | Path to the SQLite database file |
| `--agent-bin-dir` | `` | Directory containing agent binaries served via `/install/{os}/{arch}` |
| `--dev` | `false` | Enable development mode (permissive CORS, no embedded frontend served) |

**Example:**

```bash
./bin/aerodocs \
  --addr 127.0.0.1:8080 \
  --grpc-addr 0.0.0.0:9090 \
  --db /var/lib/aerodocs/aerodocs.db \
  --agent-bin-dir /opt/aerodocs/agent-bins
```

Bind HTTP to `127.0.0.1` (loopback only) when running behind a reverse proxy. The gRPC port (`9090`) must be reachable by agents — bind to `0.0.0.0` or the server's public interface.

On first run, migrations execute automatically and the database file is created. Navigate to your domain in a browser to complete initial setup.

---

## Systemd Service

Create `/etc/systemd/system/aerodocs.service`:

```ini
[Unit]
Description=AeroDocs Hub
After=network.target

[Service]
Type=simple
User=aerodocs
Group=aerodocs
WorkingDirectory=/opt/aerodocs
ExecStart=/opt/aerodocs/bin/aerodocs \
  --addr 127.0.0.1:8080 \
  --grpc-addr 0.0.0.0:9090 \
  --db /var/lib/aerodocs/aerodocs.db \
  --agent-bin-dir /opt/aerodocs/agent-bins
Restart=on-failure
RestartSec=5s

# Harden the service
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/aerodocs /opt/aerodocs/agent-bins
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
# Create the user and directories
useradd --system --no-create-home --shell /usr/sbin/nologin aerodocs
mkdir -p /opt/aerodocs/bin /var/lib/aerodocs
cp bin/aerodocs /opt/aerodocs/bin/aerodocs
chown -R aerodocs:aerodocs /opt/aerodocs /var/lib/aerodocs

# Enable and start
systemctl daemon-reload
systemctl enable aerodocs
systemctl start aerodocs
systemctl status aerodocs
```

---

## Reverse Proxy with Traefik

AeroDocs is designed to run behind Traefik. Traefik handles both HTTP (the web UI and REST API) and gRPC (agent connections) routing.

Create `/etc/traefik/dynamic/aerodocs.yml`:

```yaml
http:
  routers:
    # HTTP and REST API traffic
    aerodocs:
      rule: "Host(`aerodocs.example.com`)"
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
      service: aerodocs

    # gRPC traffic from agents (path prefix matches the proto package)
    aerodocs-grpc:
      rule: "Host(`aerodocs.example.com`) && PathPrefix(`/aerodocs.v1.`)"
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
      service: aerodocs-grpc

  services:
    aerodocs:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:8080"

    # h2c (cleartext HTTP/2) is required for gRPC backends without TLS
    aerodocs-grpc:
      loadBalancer:
        servers:
          - url: "h2c://127.0.0.1:9090"
```

Traefik handles TLS termination (Let's Encrypt). The Hub sees plain HTTP on port 8080 and cleartext gRPC (h2c) on port 9090. The `PathPrefix('/aerodocs.v1.')` matcher routes agent gRPC connections correctly because gRPC uses the proto package path as the HTTP/2 `:path` header.

Make sure your main `traefik.yml` has the file provider enabled:

```yaml
providers:
  file:
    directory: /etc/traefik/dynamic
    watch: true
```

---

## Agent Deployment

Agents are lightweight Go binaries deployed on each managed server. They dial out to the Hub's gRPC port — no inbound firewall rules are needed on the agent host.

### One-command install (recommended)

The Hub serves an install script and the agent binaries. On the managed server, run:

```bash
curl -sSL https://aerodocs.example.com/install.sh | sudo bash -s -- \
  --token <REGISTRATION_TOKEN> \
  --hub aerodocs.example.com:443
```

The script will:
1. Detect the OS and CPU architecture
2. **Auto-detect an existing installation** — if `aerodocs-agent` is already installed and has a valid `agent.conf`, the script calls `aerodocs-agent --self-unregister` to remove the old server entry from the Hub before proceeding
3. Download the correct agent binary from `/install/{os}/{arch}`
4. Write the configuration to `/etc/aerodocs/agent.conf`
5. Install and enable a systemd service for the agent
6. **Verify registration** — after starting the agent service, the script checks that the agent successfully registered with the Hub before reporting success

#### Piped vs. manual execution

| Mode | Behaviour |
|------|-----------|
| Piped from `curl` (non-interactive) | Automatically replaces any existing installation without prompting |
| Run as a script manually (interactive terminal) | Prompts **[R]eplace / [K]eep** if an existing installation is detected; exits with a non-zero status if the user selects Keep |

### Manual install

If you prefer not to pipe to bash:

```bash
# Download the binary (example: linux/amd64)
curl -Lo /usr/local/bin/aerodocs-agent \
  https://aerodocs.example.com/install/linux/amd64
chmod +x /usr/local/bin/aerodocs-agent

# Run directly
/usr/local/bin/aerodocs-agent \
  --hub aerodocs.example.com:443 \
  --token <REGISTRATION_TOKEN>
```

### Agent configuration file

After first registration, the agent writes its assigned `server_id` and `hub_url` to:

```
/etc/aerodocs/agent.conf
```

Example contents (JSON):

```json
{
  "server_id": "550e8400-e29b-41d4-a716-446655440000",
  "hub_url": "aerodocs.example.com:443"
}
```

On restart, the agent loads this file and reconnects without needing the registration token again.

### Agent systemd service

The install script creates `/etc/systemd/system/aerodocs-agent.service`:

```ini
[Unit]
Description=AeroDocs Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aerodocs-agent \
  --hub aerodocs.example.com:443 \
  --token <REGISTRATION_TOKEN>
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

### Agent flags

| Flag | Description |
|------|-------------|
| `--hub <addr>` | Hub gRPC address (e.g. `aerodocs.example.com:443` or `192.168.1.10:9090`) |
| `--token <token>` | One-time registration token obtained from the Hub when creating a server record |
| `--self-unregister` | Calls `DELETE /api/servers/{id}/self-unregister` on the Hub to remove the current server entry, then exits. Used by the install script before re-installing to clean up the old registration. Requires a valid `agent.conf` with a known `server_id`. |

### TLS auto-detection

The agent infers the connection security mode from the hub address:
- **Hostname** (e.g. `aerodocs.example.com:443`) → TLS enabled
- **IP address** (e.g. `192.168.1.10:9090`) → insecure (no TLS)

### Reconnect behavior

The agent reconnects with **exponential backoff** — starting at 1 second and capping at 60 seconds. On each reconnect it re-sends a `Register` message so the Hub updates sysinfo and resets connection state.

### Network requirements

| Direction | Protocol | Port | Notes |
|-----------|----------|------|-------|
| Agent → Hub | gRPC (HTTP/2) | 9090 (or 443 via Traefik) | Must be reachable from agent host |
| Hub → Agent | None | — | Agents always dial out; Hub never initiates |

---

## DNS Setup

Point your domain's A record (and AAAA for IPv6) to the public IP of the server running Traefik.

```
aerodocs.example.com.  IN  A  203.0.113.10
```

Traefik will automatically obtain a Let's Encrypt certificate on first request once the DNS record propagates.

---

## Database Management

AeroDocs uses SQLite with WAL mode. The database is a single file (default: `aerodocs.db`).

**Auto-migrations**: On every startup, the Hub checks for and applies any unapplied migration files. No manual schema management is needed after upgrades.

**Backups**: SQLite's WAL mode makes it safe to copy the database file while the Hub is running. To take a consistent snapshot:

```bash
# Method 1: sqlite3 backup (preferred — uses SQLite's online backup API)
sqlite3 /var/lib/aerodocs/aerodocs.db ".backup /var/lib/aerodocs/backup-$(date +%Y%m%d).db"

# Method 2: Simple file copy (safe with WAL mode)
cp /var/lib/aerodocs/aerodocs.db /var/lib/aerodocs/backup-$(date +%Y%m%d).db
```

Store backups off-host. The database contains all user accounts, server registrations, and audit logs.

---

## CLI Break-Glass: Emergency TOTP Reset

If an admin is locked out (lost their authenticator app), use the `admin reset-totp` command directly on the server hosting the Hub. This requires shell access to the machine — it cannot be triggered via the web UI.

```bash
./bin/aerodocs admin reset-totp --username <username> --db /var/lib/aerodocs/aerodocs.db
```

This will:
1. Clear the user's TOTP secret and mark TOTP as disabled
2. Generate a new temporary password
3. Print the temporary password to stdout

The user must set up TOTP again on their next login. The operation is recorded in the audit log.

**Stop the service first if you need to run this on a locked database:**

```bash
systemctl stop aerodocs
./bin/aerodocs admin reset-totp --username admin --db /var/lib/aerodocs/aerodocs.db
systemctl start aerodocs
```

In practice the Hub does not hold exclusive locks on the database (WAL mode), so running the command while the service is live is usually safe.

---

## Updating / Redeploying

1. Build the new binary from source (`make build`)
2. Copy the binary to the server:
   ```bash
   scp bin/aerodocs user@yourserver:/opt/aerodocs/bin/aerodocs.new
   ```
3. Swap the binary and restart:
   ```bash
   mv /opt/aerodocs/bin/aerodocs.new /opt/aerodocs/bin/aerodocs
   systemctl restart aerodocs
   ```
4. New migrations (if any) run automatically on startup.
