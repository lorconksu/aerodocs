#!/bin/bash
set -euo pipefail

# AeroDocs Agent Installer
# Usage: curl -sSL https://<hub>/install.sh | sudo bash -s -- --token <TOKEN> --hub <GRPC_ADDR>
# When piped from curl, --force is implied (no interactive prompt).
# To run interactively: bash install.sh --token <TOKEN> --hub <GRPC_ADDR>

TOKEN=""
HUB=""
URL=""
FORCE=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --token) TOKEN="$2"; shift 2 ;;
    --hub)   HUB="$2";   shift 2 ;;
    --url)   URL="$2";   shift 2 ;;
    --force) FORCE=true;  shift ;;
    *)       echo "Unknown argument: $1"; exit 1 ;;
  esac
done

# Auto-detect piped input (curl | bash) — can't prompt, so force replace
if [[ ! -t 0 ]]; then
  FORCE=true
fi

if [[ -z "$TOKEN" ]] || [[ -z "$HUB" ]]; then
  echo "Usage: sudo bash install.sh --token <TOKEN> --hub <GRPC_ADDR>"
  echo "  --token   One-time registration token from Hub"
  echo "  --hub     Hub gRPC address (e.g., 10.0.1.5:9090)"
  echo "  --force   Replace existing installation without prompting"
  exit 1
fi

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

if [[ "$OS" != "linux" ]]; then
  echo "Unsupported OS: $OS (only linux is supported)"
  exit 1
fi

# --- Check for existing installation ---
EXISTING=false
if systemctl is-active --quiet aerodocs-agent 2>/dev/null; then
  EXISTING=true
elif [[ -f /usr/local/bin/aerodocs-agent ]] || [[ -f /etc/systemd/system/aerodocs-agent.service ]]; then
  EXISTING=true
fi

if [[ "$EXISTING" = true ]]; then
  if [[ "$FORCE" = true ]]; then
    echo "==> Existing installation detected — replacing automatically."
  else
    echo ""
    echo "    An existing AeroDocs Agent installation was detected."
    if systemctl is-active --quiet aerodocs-agent 2>/dev/null; then
      echo "    Status: RUNNING"
    else
      echo "    Status: installed but not running"
    fi
    echo ""
    echo "    [R] Replace — stop the current agent and install the new one"
    echo "    [K] Keep    — keep the current installation and cancel"
    echo ""
    while true; do
      read -rp "    Choose [R/K]: " choice </dev/tty
      case "$choice" in
        [Rr]) break ;;
        [Kk])
          echo ""
          echo "==> Keeping current installation. Exiting."
          exit 0
          ;;
        *) echo "    Please enter R or K." ;;
      esac
    done
  fi

  # Unregister old server from Hub before teardown
  if [[ -x /usr/local/bin/aerodocs-agent ]] && [[ -f /etc/aerodocs/agent.conf ]]; then
    echo "==> Removing old server from Hub..."
    /usr/local/bin/aerodocs-agent --self-unregister 2>/dev/null || true
  fi
  echo "==> Removing previous installation..."
  systemctl stop aerodocs-agent 2>/dev/null || true
  systemctl disable aerodocs-agent 2>/dev/null || true
  pkill -9 -f aerodocs-agent 2>/dev/null || true
  sleep 1
  rm -f /usr/local/bin/aerodocs-agent
  rm -f /etc/aerodocs/agent.conf
  rm -f /etc/systemd/system/aerodocs-agent.service
  systemctl daemon-reload 2>/dev/null || true
fi

# --- Download agent binary ---
if [[ -z "$URL" ]]; then
  HUB_HOST=$(echo "$HUB" | cut -d: -f1)
  URL="https://${HUB_HOST}"
fi
DOWNLOAD_URL="${URL}/install/${OS}/${ARCH}"

echo "==> Downloading AeroDocs Agent (${OS}-${ARCH})..."
curl -sSL "$DOWNLOAD_URL" -o /usr/local/bin/aerodocs-agent
chmod +x /usr/local/bin/aerodocs-agent

# Verify binary was downloaded
if [[ ! -x /usr/local/bin/aerodocs-agent ]]; then
  echo "ERROR: Failed to download agent binary" >&2
  exit 1
fi

# --- Create config directory ---
echo "==> Creating config directory..."
mkdir -p /etc/aerodocs

# --- Install systemd service ---
echo "==> Installing systemd service..."
cat > /etc/systemd/system/aerodocs-agent.service <<EOF
[Unit]
Description=AeroDocs Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/aerodocs-agent --hub ${HUB} --token ${TOKEN}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now aerodocs-agent

# --- Verify agent started and registered ---
echo "==> Waiting for agent to register..."
TRIES=0
MAX_TRIES=10
while [[ $TRIES -lt $MAX_TRIES ]]; do
  sleep 2
  if journalctl -u aerodocs-agent --no-pager -n 5 2>/dev/null | grep -q "registered successfully\|connected to hub"; then
    echo ""
    echo "==> AeroDocs Agent installed and connected!"
    echo "    Service: systemctl status aerodocs-agent"
    echo "    Logs:    journalctl -u aerodocs-agent -f"
    exit 0
  fi
  TRIES=$((TRIES + 1))
  echo "    Waiting... (${TRIES}/${MAX_TRIES})"
done

# If we get here, registration didn't succeed in time
echo ""
echo "==> AeroDocs Agent installed but registration not confirmed yet."
echo "    Check the logs: journalctl -u aerodocs-agent -f"
echo "    The agent will keep retrying in the background."
