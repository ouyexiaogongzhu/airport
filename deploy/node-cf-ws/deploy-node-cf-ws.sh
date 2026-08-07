#!/usr/bin/env bash
# RFPlay — Node deploy script (WebSocket behind Cloudflare variant)
#
# Same daemon+Xray wiring as deploy/node-reality/deploy-node.sh, tuned for a
# node whose inbound is WebSocket transport (typically behind Cloudflare).
# The Manager config for the node must set network=ws, security=tls, and a
# ws path; see the Admin panel node editor.
#
# Usage:
#   sudo ./deploy-node-cf-ws.sh --manager-url https://airport.example.com \
#                               --node-token nd_xxxxxxxxxxxxxxxxxxxxxxxx
set -euo pipefail

MANAGER_URL=""
NODE_TOKEN=""
XRAY_VERSION="v25.3.8"

usage() {
  echo "usage: $0 --manager-url URL --node-token TOKEN [--xray-version vX.Y.Z]"
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manager-url) MANAGER_URL="$2"; shift 2;;
    --node-token) NODE_TOKEN="$2"; shift 2;;
    --xray-version) XRAY_VERSION="$2"; shift 2;;
    *) usage;;
  esac
done

[[ -n "$MANAGER_URL" && -n "$NODE_TOKEN" ]] || usage

log() { printf '\033[1;34m[node-cf-ws]\033[0m %s\n' "$*"; }

# --- 1. Install Xray-core ---
if ! command -v xray >/dev/null 2>&1; then
  log "installing Xray-core $XRAY_VERSION"
  bash -c "$(curl -L https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install --version "$XRAY_VERSION"
else
  log "xray already installed: $(xray version | head -1)"
fi

XRAY_BIN="$(command -v xray)"
mkdir -p /var/lib/rfplay /var/log/xray

# --- 2. Install the RFPlay daemon ---
DAEMON_BIN=/usr/local/bin/rfplay-daemon
if [[ ! -x "$DAEMON_BIN" ]]; then
  log "building and installing rfplay-daemon"
  if ! command -v go >/dev/null 2>&1; then
    log "Go not found — installing golang"
    apt-get update -qq
    apt-get install -y -qq golang-go
  fi
  REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
  (cd "$REPO_DIR/daemon" && go build -o /tmp/rfplay-daemon ./cmd/main.go)
  install -m 0755 /tmp/rfplay-daemon "$DAEMON_BIN"
else
  log "daemon already installed at $DAEMON_BIN"
fi

# --- 3. Daemon config ---
cat > /etc/rfplay-daemon.json << EOF
{
  "node_id": 1,
  "manager_url": "${MANAGER_URL}",
  "manager_token": "${NODE_TOKEN}",
  "sync_interval": 60000000000,
  "listen_addr": "127.0.0.1:9090",
  "data_dir": "/var/lib/rfplay",
  "xray_binary": "${XRAY_BIN}"
}
EOF
chmod 0600 /etc/rfplay-daemon.json

# --- 4. systemd units ---
cat > /etc/systemd/system/rfplay-daemon.service << 'EOF'
[Unit]
Description=RFPlay Node Daemon (config pull + traffic report)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/rfplay-daemon /etc/rfplay-daemon.json
Restart=always
RestartSec=5
User=root
ProtectSystem=full
ReadWritePaths=/var/lib/rfplay /var/log/xray

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/rfplay-xray.service << 'EOF'
[Unit]
Description=RFPlay Xray-core (config managed by rfplay-daemon)
After=rfplay-daemon.service
Requires=rfplay-daemon.service
PartOf=rfplay-daemon.service

[Service]
Type=simple
ExecStart=/usr/local/bin/xray -c /var/lib/rfplay/xray.json
Restart=always
RestartSec=3
User=root
LimitNOFILE=1048576
ProtectSystem=full
ReadWritePaths=/var/log/xray

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable rfplay-daemon rfplay-xray
systemctl restart rfplay-daemon
systemctl restart rfplay-xray

log "daemon + xray started (WS/Cloudflare node)."
log "  journalctl -u rfplay-daemon -f"
log "  journalctl -u rfplay-xray -f"
log "  xray config written to /var/lib/rfplay/xray.json on first sync."
