#!/usr/bin/env bash
# RFPlay Airport — Start All Services
# Starts Manager API, Node Daemon (and optionally Xray-core) in order.
#
# Usage:
#   ./deploy/start-all.sh                    # Start manager + daemon
#   ./deploy/start-all.sh --with-xray        # Start manager + daemon + xray
#   ./deploy/start-all.sh --background       # Start all in background

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

WITH_XRAY=false
BACKGROUND=false

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --with-xray) WITH_XRAY=true ;;
        --background) BACKGROUND=true ;;
    esac
done

echo "=========================================="
echo " RFPlay Airport — Starting All Services"
echo "=========================================="
echo "Project: $PROJECT_DIR"
echo "With Xray: $WITH_XRAY"
echo "Background: $BACKGROUND"
echo ""

# --- Step 1: Build everything first ---
echo "[build] Building manager..."
(cd "$PROJECT_DIR/manager" && go build -o "$PROJECT_DIR/build/manager" ./cmd/server/)

echo "[build] Building daemon..."
(cd "$PROJECT_DIR/daemon" && go build -o "$PROJECT_DIR/build/daemon" ./cmd/main.go)

if [ "$WITH_XRAY" = true ]; then
    echo "[build] Building xray-core..."
    (cd "$PROJECT_DIR/xray-core" && go build -o "$PROJECT_DIR/build/xray-core" .)
fi

mkdir -p "$PROJECT_DIR/logs"
echo ""

# --- Step 2: Start Manager API ---
echo "[start] Starting Manager API on :${PORT:-8080}..."
if [ "$BACKGROUND" = true ]; then
    PORT="${PORT:-8080}" \
    DATA_DIR="${DATA_DIR:-${PROJECT_DIR}/data}" \
    JWT_SECRET="${JWT_SECRET:-dev-secret}" \
    nohup "$PROJECT_DIR/build/manager" > "$PROJECT_DIR/logs/manager.log" 2>&1 &
    MANAGER_PID=$!
    echo "[start] Manager PID: $MANAGER_PID"
    sleep 2
else
    PORT="${PORT:-8080}" \
    DATA_DIR="${DATA_DIR:-${PROJECT_DIR}/data}" \
    JWT_SECRET="${JWT_SECRET:-dev-secret}" \
    "$PROJECT_DIR/build/manager" &
    MANAGER_PID=$!
fi

# Wait for manager to be ready
echo "[start] Waiting for Manager to be ready..."
for i in $(seq 1 15); do
    if curl -sf http://localhost:${PORT:-8080}/health > /dev/null 2>&1; then
        echo "[start] Manager is ready."
        break
    fi
    sleep 1
done

# --- Step 3: Start Node Daemon ---
echo "[start] Starting Node Daemon on :${DAEMON_LISTEN_ADDR:-9090}..."
if [ "$BACKGROUND" = true ]; then
    DAEMON_MANAGER_URL="${DAEMON_MANAGER_URL:-http://localhost:8080}" \
    DAEMON_MANAGER_TOKEN="${DAEMON_MANAGER_TOKEN:-}" \
    DAEMON_NODE_ID="${DAEMON_NODE_ID:-1}" \
    DAEMON_LISTEN_ADDR="${DAEMON_LISTEN_ADDR:-:9090}" \
    nohup "$PROJECT_DIR/build/daemon" > "$PROJECT_DIR/logs/daemon.log" 2>&1 &
    DAEMON_PID=$!
    echo "[start] Daemon PID: $DAEMON_PID"
else
    DAEMON_MANAGER_URL="${DAEMON_MANAGER_URL:-http://localhost:8080}" \
    DAEMON_MANAGER_TOKEN="${DAEMON_MANAGER_TOKEN:-}" \
    DAEMON_NODE_ID="${DAEMON_NODE_ID:-1}" \
    DAEMON_LISTEN_ADDR="${DAEMON_LISTEN_ADDR:-:9090}" \
    "$PROJECT_DIR/build/daemon" &
    DAEMON_PID=$!
fi

sleep 1

# --- Step 4: (Optional) Start Xray-core ---
if [ "$WITH_XRAY" = true ]; then
    echo "[start] Starting Xray-core..."
    if [ "$BACKGROUND" = true ]; then
        nohup "$PROJECT_DIR/build/xray-core" "$PROJECT_DIR/xray-core/config.json" > "$PROJECT_DIR/logs/xray.log" 2>&1 &
        XRAY_PID=$!
        echo "[start] Xray PID: $XRAY_PID"
    else
        "$PROJECT_DIR/build/xray-core" "$PROJECT_DIR/xray-core/config.json" &
        XRAY_PID=$!
    fi
fi

echo ""
echo "=========================================="
echo " All services started!"
echo "=========================================="
echo ""
echo " Manager API:  http://localhost:${PORT:-8080}"
echo " Daemon API:   http://localhost:${DAEMON_LISTEN_ADDR:-9090}"
if [ "$WITH_XRAY" = true ]; then
    echo " Xray Verify:  http://localhost:1099/verify"
fi
echo ""
echo " Logs:         $PROJECT_DIR/logs/"
echo " Binaries:     $PROJECT_DIR/build/"
echo ""

if [ "$BACKGROUND" = true ]; then
    echo " PIDs:"
    echo "  Manager: $MANAGER_PID"
    echo "  Daemon:  $DAEMON_PID"
    [ "$WITH_XRAY" = true ] && echo "  Xray:    $XRAY_PID"
    echo ""
    echo " To stop: kill $MANAGER_PID $DAEMON_PID ${XRAY_PID:-}"
fi
