#!/bin/bash
set -euo pipefail

# AeroDocs Deploy Script
# Builds and deploys Hub + Agent to /opt/aerodocs on this machine.
# Usage: ./scripts/deploy.sh

DEPLOY_DIR="/opt/aerodocs"
PROJECT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "==> Building frontend..."
cd "$PROJECT_DIR/web" && npm run build --silent

echo "==> Embedding frontend into Hub..."
rm -rf "$PROJECT_DIR/hub/web/dist"
mkdir -p "$PROJECT_DIR/hub/web"
cp -r "$PROJECT_DIR/web/dist" "$PROJECT_DIR/hub/web/dist"

echo "==> Building Hub binary..."
cd "$PROJECT_DIR/hub" && go build -o "$PROJECT_DIR/bin/aerodocs" ./cmd/aerodocs/

echo "==> Building Agent binary..."
cd "$PROJECT_DIR/agent" && GOOS=linux GOARCH=amd64 go build -o "$PROJECT_DIR/bin/aerodocs-agent-linux-amd64" ./cmd/aerodocs-agent/

echo "==> Stopping services..."
pkill -9 aerodocs-agent 2>/dev/null || true
sudo systemctl kill -s SIGKILL aerodocs 2>/dev/null || true
sleep 2

echo "==> Deploying binaries to $DEPLOY_DIR..."
cp "$PROJECT_DIR/bin/aerodocs" "$DEPLOY_DIR/aerodocs"
cp "$PROJECT_DIR/bin/aerodocs-agent-linux-amd64" "$DEPLOY_DIR/aerodocs-agent-linux-amd64"
mkdir -p "$DEPLOY_DIR/static"
cp "$PROJECT_DIR/hub/static/install.sh" "$DEPLOY_DIR/static/install.sh"

echo "==> Starting Hub..."
sudo systemctl start aerodocs
sleep 2

if systemctl is-active --quiet aerodocs; then
    echo "==> Hub is running."
    journalctl -u aerodocs --no-pager -n 3
else
    echo "==> ERROR: Hub failed to start."
    journalctl -u aerodocs --no-pager -n 10
    exit 1
fi

echo ""
echo "==> Deploy complete!"
