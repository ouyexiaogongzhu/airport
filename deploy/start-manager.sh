#!/usr/bin/env bash
# RFPlay Airport — Start Manager API
# Usage: ./deploy/start-manager.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "[start-manager] Starting RFPlay Manager API..."

# Default configuration via environment
export PORT="${PORT:-8080}"
export DATA_DIR="${DATA_DIR:-${PROJECT_DIR}/data}"
export JWT_SECRET="${JWT_SECRET:-dev-secret}"

echo "[start-manager] PORT=$PORT"
echo "[start-manager] DATA_DIR=$DATA_DIR"

# Ensure data directory exists
mkdir -p "$DATA_DIR"

# Change to the manager directory
cd "$PROJECT_DIR/manager"

# Run the manager (go run for development; use go build for production)
echo "[start-manager] Running manager on :$PORT..."
exec go run ./cmd/server/
