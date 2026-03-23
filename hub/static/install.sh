#!/bin/bash
set -euo pipefail

# AeroDocs Agent Installer
# Usage: curl -sSL https://aerodocs.yiucloud.com/install.sh | sudo bash -s -- --token <TOKEN> --hub <HUB_URL>

TOKEN=""
HUB=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --token) TOKEN="$2"; shift 2 ;;
    --hub)   HUB="$2";   shift 2 ;;
    *)       echo "Unknown argument: $1"; exit 1 ;;
  esac
done

if [ -z "$TOKEN" ] || [ -z "$HUB" ]; then
  echo "Usage: sudo bash install.sh --token <TOKEN> --hub <HUB_GRPC_URL>"
  echo "  --token   One-time registration token from Hub"
  echo "  --hub     Hub gRPC address (e.g., aerodocs.yiucloud.com:9090)"
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

if [ "$OS" != "linux" ]; then
  echo "Unsupported OS: $OS (only linux is supported)"
  exit 1
fi

# Extract HTTP URL from Hub address for binary download
HUB_HOST=$(echo "$HUB" | cut -d: -f1)
DOWNLOAD_URL="https://${HUB_HOST}/install/${OS}/${ARCH}"

echo "==> Downloading AeroDocs Agent (${OS}-${ARCH})..."
curl -sSL "$DOWNLOAD_URL" -o /usr/local/bin/aerodocs-agent
chmod +x /usr/local/bin/aerodocs-agent

echo "==> Creating config directory..."
mkdir -p /etc/aerodocs

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

echo ""
echo "==> AeroDocs Agent installed and started!"
echo "    Service: systemctl status aerodocs-agent"
echo "    Logs:    journalctl -u aerodocs-agent -f"
