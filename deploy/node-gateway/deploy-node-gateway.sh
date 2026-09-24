#!/usr/bin/env bash
# RFPlay — 節點部署（VLESS/VMess over XHTTP，cloudflared Tunnel 回源）
#
#   客戶端 ──TLS 443──> Cloudflare 邊緣 ──Tunnel──> cloudflared ──> Xray 127.0.0.1:<local-port>
#
# 傳輸由後台 nodes.network 決定（生產僅 xhttp）；本腳本不寫死 streamSettings，
# gateway 拉取配置後啟動 Xray。Xray 只監聽 127.0.0.1；節點不開任何代理端口、不需要證書。
# TLS 在 CF 邊緣終結。後台先建節點（域名 = --hostname，本地端口 = --local-port），
# 再點 Token 生成 nd_... token。
#
# Tunnel：在 Cloudflare Zero Trust → Networks → Tunnels 建一個 cloudflared Tunnel，複製其 token。
#   - 給了 --cf-api-token（權限：Account > Cloudflare Tunnel: Edit，Zone > DNS: Edit）時，
#     腳本自動寫入 Tunnel ingress（hostname → http://127.0.0.1:<local-port>）並建橙雲 CNAME；
#   - 否則需在 Tunnel 的 Public Hostname 頁手動添加同樣的映射（會自動建 CNAME）。
#   - XHTTP 的 Tunnel ingress 形態為 HTTP 明文回源。
#
# 依賴：Debian/Ubuntu（apt）。AlmaLinux/RHEL 請手動安裝 Xray + Go gateway + cloudflared（rpm），
# 配置與單元文件可參照本腳本後續步驟。
#
# Usage（新裝）:
#   sudo ./deploy-node-gateway.sh --manager-url https://api.rfplay.uk \
#        --node-token nd_xxx --tunnel-token eyJ... \
#        --hostname node-hk.rfplay.uk --local-port 20001 [--cf-api-token XXX] [--xray-version vX.Y.Z]
#
# --- 從 rfplay-daemon 升級到 rfplay-gateway（不銷毀節點）---
#   1) 在已 clone 的倉庫根目錄拉最新代碼（含 gateway/ 與本腳本）
#   2) 用與當初相同的 --manager-url / --node-token / --tunnel-token / --hostname / --local-port 再跑本腳本
#   3) 腳本會：stop+disable rfplay-daemon → 安裝 rfplay-gateway 二進制與 unit →
#      寫入 /etc/rfplay-gateway.json（CLI 參數）→ 啟用並重啟 rfplay-gateway；
#      cloudflared / Xray 數據目錄 /var/lib/rfplay 保留；舊 /etc/rfplay-daemon.json 不刪以便回滾
#   4) 確認：systemctl status rfplay-gateway && journalctl -u rfplay-gateway -f
#   5) 可選清理：rm -f /usr/local/bin/rfplay-daemon /etc/rfplay-daemon.json /etc/systemd/system/rfplay-daemon.service
#      （腳本停用舊 unit 但默認不刪配置/二進制，以便回滾）
set -euo pipefail

MANAGER_URL=""
NODE_TOKEN=""
TUNNEL_TOKEN=""
HOSTNAME_FQDN=""
LOCAL_PORT=""
CF_API_TOKEN=""
# 已驗證版本（含 XHTTP）
XRAY_VERSION="v26.3.27"

usage() {
  echo "usage: $0 --manager-url URL --node-token nd_... --tunnel-token TOKEN --hostname FQDN --local-port PORT" >&2
  echo "          [--cf-api-token TOKEN] [--xray-version vX.Y.Z]" >&2
  exit 1
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manager-url) MANAGER_URL="$2"; shift 2;;
    --node-token) NODE_TOKEN="$2"; shift 2;;
    --tunnel-token) TUNNEL_TOKEN="$2"; shift 2;;
    --hostname) HOSTNAME_FQDN="$2"; shift 2;;
    --local-port) LOCAL_PORT="$2"; shift 2;;
    --cf-api-token) CF_API_TOKEN="$2"; shift 2;;
    --xray-version) XRAY_VERSION="$2"; shift 2;;
    *) usage;;
  esac
done

[[ -n "$MANAGER_URL" && -n "$NODE_TOKEN" && -n "$TUNNEL_TOKEN" && -n "$HOSTNAME_FQDN" && -n "$LOCAL_PORT" ]] || usage
[[ "$LOCAL_PORT" =~ ^[0-9]+$ ]] && (( LOCAL_PORT >= 1 && LOCAL_PORT <= 65535 )) || { echo "invalid --local-port" >&2; exit 1; }
[[ $EUID -eq 0 ]] || { echo "run as root" >&2; exit 1; }

log() { printf '\033[1;34m[node-gateway]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[node-gateway]\033[0m %s\n' "$*" >&2; exit 1; }

apt-get update -qq
apt-get install -y -qq curl jq iproute2 >/dev/null

# --- 1. Xray-core（由 gateway 管理，停用安裝腳本自帶的 xray.service）---
if ! command -v xray >/dev/null 2>&1; then
  log "installing Xray-core $XRAY_VERSION"
  bash -c "$(curl -fsSL https://github.com/XTLS/Xray-install/raw/main/install-release.sh)" @ install --version "$XRAY_VERSION"
else
  log "xray already installed: $(xray version | head -1)"
fi
systemctl disable --now xray.service 2>/dev/null || true
XRAY_BIN="$(command -v xray)"
mkdir -p /var/lib/rfplay /var/log/xray

# --- 2. rfplay-gateway（原 rfplay-daemon）---
GATEWAY_BIN=/usr/local/bin/rfplay-gateway
GATEWAY_CFG=/etc/rfplay-gateway.json
GATEWAY_UNIT=/etc/systemd/system/rfplay-gateway.service
REPO_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

# 升級路徑：先停舊 daemon，避免與新 gateway 同時拉起兩個 Xray
if systemctl list-unit-files rfplay-daemon.service >/dev/null 2>&1 \
   || [[ -f /etc/systemd/system/rfplay-daemon.service ]]; then
  log "migrating from rfplay-daemon → rfplay-gateway"
  systemctl disable --now rfplay-daemon.service 2>/dev/null || true
fi
# 若仍有舊二進制在跑（無 unit），盡力結束
if command -v pkill >/dev/null 2>&1; then
  pkill -x rfplay-daemon 2>/dev/null || true
fi

log "building rfplay-gateway"
if ! command -v go >/dev/null 2>&1; then
  log "Go not found — installing golang"
  apt-get install -y -qq golang-go >/dev/null
fi
(cd "$REPO_DIR/gateway" && go build -o /tmp/rfplay-gateway ./cmd/main.go)
install -m 0755 /tmp/rfplay-gateway "$GATEWAY_BIN"

# node_id 由配置接口響應提供；:9090 只監聽本機。
# 舊 /etc/rfplay-daemon.json 保留不刪，便於回滾；本輪以 CLI 參數寫 gateway 配置。
cat > "$GATEWAY_CFG" << EOF
{
  "manager_url": "${MANAGER_URL}",
  "manager_token": "${NODE_TOKEN}",
  "sync_interval": 60000000000,
  "listen_addr": "127.0.0.1:9090",
  "data_dir": "/var/lib/rfplay",
  "xray_binary": "${XRAY_BIN}"
}
EOF
chmod 0600 "$GATEWAY_CFG"

# 舊版腳本的獨立 xray unit 會與 gateway 拉起的 Xray 搶端口
if [[ -f /etc/systemd/system/rfplay-xray.service ]]; then
  systemctl disable --now rfplay-xray.service 2>/dev/null || true
  rm -f /etc/systemd/system/rfplay-xray.service
fi

cat > "$GATEWAY_UNIT" << 'EOF'
[Unit]
Description=RFPlay Node Gateway (config pull, Xray supervision, traffic report)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/rfplay-gateway /etc/rfplay-gateway.json
Restart=always
RestartSec=5
KillMode=control-group
User=root
LimitNOFILE=1048576
ProtectSystem=full
ReadWritePaths=/var/lib/rfplay /var/log/xray
SyslogIdentifier=rfplay-gateway
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Xray error.log 滾動（access 已關閉）
cat > /etc/logrotate.d/rfplay-xray << 'EOF'
/var/log/xray/error.log {
  weekly
  rotate 8
  missingok
  notifempty
  compress
  delaycompress
  copytruncate
}
EOF

# --- 3. cloudflared（Tunnel token 註冊為系統服務）---
if ! command -v cloudflared >/dev/null 2>&1; then
  case "$(dpkg --print-architecture)" in
    amd64) CF_ARCH=amd64;;
    arm64) CF_ARCH=arm64;;
    armhf) CF_ARCH=arm;;
    *) die "unsupported architecture for cloudflared";;
  esac
  log "installing cloudflared ($CF_ARCH)"
  curl -fsSL -o /tmp/cloudflared.deb "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${CF_ARCH}.deb"
  dpkg -i /tmp/cloudflared.deb >/dev/null
fi
if systemctl list-unit-files cloudflared.service >/dev/null 2>&1 && [[ -f /etc/systemd/system/cloudflared.service ]]; then
  log "cloudflared service exists — reinstalling with the given token"
  cloudflared service uninstall >/dev/null 2>&1 || true
fi
cloudflared service install "$TUNNEL_TOKEN"

# --- 4. Tunnel ingress + DNS（可選，經 Cloudflare API）---
# token = base64(JSON {"a": account_id, "t": tunnel_id, "s": secret})
TUNNEL_JSON="$(printf '%s' "$TUNNEL_TOKEN" | base64 -d 2>/dev/null || true)"
ACCOUNT_ID="$(jq -r '.a // empty' <<<"$TUNNEL_JSON" 2>/dev/null || true)"
TUNNEL_ID="$(jq -r '.t // empty' <<<"$TUNNEL_JSON" 2>/dev/null || true)"
if [[ -n "$CF_API_TOKEN" ]]; then
  [[ -n "$ACCOUNT_ID" && -n "$TUNNEL_ID" ]] || die "cannot decode account/tunnel id from --tunnel-token"
  CF_API=https://api.cloudflare.com/client/v4
  cf() { curl -fsS -H "Authorization: Bearer ${CF_API_TOKEN}" -H 'Content-Type: application/json' "$@"; }

  log "configuring tunnel ingress: ${HOSTNAME_FQDN} → http://127.0.0.1:${LOCAL_PORT}"
  INGRESS="$(jq -n --arg h "$HOSTNAME_FQDN" --arg s "http://127.0.0.1:${LOCAL_PORT}" \
    '{config: {ingress: [{hostname: $h, service: $s}, {service: "http_status:404"}]}}')"
  cf -X PUT "${CF_API}/accounts/${ACCOUNT_ID}/cfd_tunnel/${TUNNEL_ID}/configurations" -d "$INGRESS" >/dev/null

  ZONE_NAME="$(awk -F. '{print $(NF-1)"."$NF}' <<<"$HOSTNAME_FQDN")"
  ZONE_ID="$(cf "${CF_API}/zones?name=${ZONE_NAME}" | jq -r '.result[0].id // empty')"
  [[ -n "$ZONE_ID" ]] || die "zone ${ZONE_NAME} not found for this API token"
  RECORD="$(jq -n --arg n "$HOSTNAME_FQDN" --arg c "${TUNNEL_ID}.cfargotunnel.com" \
    '{type: "CNAME", name: $n, content: $c, proxied: true}')"
  REC_ID="$(cf "${CF_API}/zones/${ZONE_ID}/dns_records?name=${HOSTNAME_FQDN}" | jq -r '.result[0].id // empty')"
  if [[ -n "$REC_ID" ]]; then
    cf -X PUT "${CF_API}/zones/${ZONE_ID}/dns_records/${REC_ID}" -d "$RECORD" >/dev/null
  else
    cf -X POST "${CF_API}/zones/${ZONE_ID}/dns_records" -d "$RECORD" >/dev/null
  fi
  log "DNS: ${HOSTNAME_FQDN} CNAME ${TUNNEL_ID}.cfargotunnel.com (proxied)"
else
  log "no --cf-api-token: add Public Hostname in the Tunnel dashboard:"
  log "  ${HOSTNAME_FQDN}  →  HTTP  127.0.0.1:${LOCAL_PORT}"
fi

# --- 5. 啟動 ---
systemctl daemon-reload
systemctl enable rfplay-gateway cloudflared >/dev/null
systemctl restart rfplay-gateway
systemctl restart cloudflared

# --- 6. 驗證：Xray 端口只在回環地址監聽 ---
log "waiting for gateway to sync config and start Xray on port ${LOCAL_PORT}..."
for _ in $(seq 1 30); do
  ss -ltnH "sport = :${LOCAL_PORT}" | grep -q . && break
  sleep 3
done
check_loopback_only() {
  local port="$1" addrs bad
  addrs="$(ss -ltnH "sport = :${port}" | awk '{print $4}')"
  [[ -n "$addrs" ]] || return 2
  bad="$(grep -vE '^(127\.[0-9.]+|\[::1\]|\[::ffff:127\.[0-9.]+\]):[0-9]+$' <<<"$addrs" || true)"
  if [[ -n "$bad" ]]; then
    printf '%s\n' "$bad"
    return 1
  fi
}
rc=0
out="$(check_loopback_only "$LOCAL_PORT")" || rc=$?
case "$rc" in
  0) log "OK: port ${LOCAL_PORT} listens on loopback only";;
  1) die "port ${LOCAL_PORT} is listening on a public interface: ${out} — refusing to continue (check local port / Xray config)";;
  2) log "WARN: nothing listening on ${LOCAL_PORT} yet — check the node is active in admin and 'journalctl -u rfplay-gateway'";;
esac
for p in 9090 10085 10086; do
  rc=0
  out="$(check_loopback_only "$p")" || rc=$?
  [[ "$rc" -ne 1 ]] || die "port ${p} is listening on a public interface: ${out}"
done

log "done. Node: https://${HOSTNAME_FQDN} (443 via Cloudflare) → 127.0.0.1:${LOCAL_PORT}"
log "  journalctl -u rfplay-gateway -f    # config sync / traffic report / xray"
log "  journalctl -u cloudflared -f       # tunnel"
log "Recommended firewall: allow inbound SSH only (e.g. ufw default deny incoming && ufw allow OpenSSH && ufw enable)."
