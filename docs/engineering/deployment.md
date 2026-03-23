# AeroDocs Deployment Guide

## Prerequisites

- Go 1.22+
- Node.js 20+
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
| `--addr` | `:8080` | Listen address and port |
| `--db` | `aerodocs.db` | Path to the SQLite database file |
| `--dev` | `false` | Enable development mode (permissive CORS, no embedded frontend served) |

**Example:**

```bash
./bin/aerodocs --addr 127.0.0.1:8080 --db /var/lib/aerodocs/aerodocs.db
```

Bind to `127.0.0.1` (loopback only) when running behind a reverse proxy — do not expose the Hub directly on a public interface.

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
ExecStart=/opt/aerodocs/bin/aerodocs --addr 127.0.0.1:8080 --db /var/lib/aerodocs/aerodocs.db
Restart=on-failure
RestartSec=5s

# Harden the service
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/var/lib/aerodocs
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

AeroDocs is designed to run behind Traefik. Use Traefik's file provider to define a router and service for AeroDocs.

Create `/etc/traefik/dynamic/aerodocs.yml`:

```yaml
http:
  routers:
    aerodocs:
      rule: "Host(`aerodocs.example.com`)"
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
      service: aerodocs

  services:
    aerodocs:
      loadBalancer:
        servers:
          - url: "http://127.0.0.1:8080"
```

Traefik handles TLS termination (Let's Encrypt) and proxies traffic to the Hub. The Hub only sees plain HTTP on the loopback interface.

Make sure your main `traefik.yml` has the file provider enabled:

```yaml
providers:
  file:
    directory: /etc/traefik/dynamic
    watch: true
```

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
