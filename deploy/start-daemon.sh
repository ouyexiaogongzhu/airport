#!/usr/bin/env bash
# RFPlay Airport — Start Node Daemon
# Usage: ./deploy/start-daemon.sh [config_path]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "[start-daemon] Starting RFPlay Node Daemon..."

# Default configuration
DAEMON_CONFIG="${1:-${PROJECT_DIR}/daemon/daemon.json}"
export DAEMON_CONFIG

# Configuration via environment (overrides config file)
export DAEMON_MANAGER_URL="${DAEMON_MANAGER_URL:-http://localhost:8080}"
export DAEMON_MANAGER_TOKEN="${DAEMON_MANAGER_TOKEN:-}"
export DAEMON_NODE_ID="${DAEMON_NODE_ID:-1}"
export DAEMON_LISTEN_ADDR="${DAEMON_LISTEN_ADDR:-:9090}"

echo "[start-daemon] MANAGER_URL=$DAEMON_MANAGER_URL"
echo "[start-daemon] NODE_ID=$DAEMON_NODE_ID"
echo "[start-daemon] LISTEN_ADDR=$DAEMON_LISTEN_ADDR"
echo "[start-daemon] CONFIG=$DAEMON_CONFIG"

# Create default config if it doesn't exist
if [ ! -f "$DAEMON_CONFIG" ]; then
    echo "[start-daemon] Creating default config at $DAEMON_CONFIG..."
    cat > "$DAEMON_CONFIG" << EOF
{
  "node_id": ${DAEMON_NODE_ID},
  "manager_url": "${DAEMON_MANAGER_URL}",
  "manager_token": "${DAEMON_MANAGER_TOKEN}",
  "sync_interval": 60000000000,
  "listen_addr": "${DAEMON_LISTEN_ADDR}",
  "data_dir": "${PROJECT_DIR}/data/daemon"
}
EOF
fi

# Ensure data directory exists
DATA_DIR="${PROJECT_DIR}/data/daemon"
mkdir -p "$DATA_DIR"

# Change to the daemon directory
cd "$PROJECT_DIR/daemon"

# Run the daemon
echo "[start-daemon] Running daemon on $DAEMON_LISTEN_ADDR..."
exec go run ./cmd/main.go "$DAEMON_CONFIG"
