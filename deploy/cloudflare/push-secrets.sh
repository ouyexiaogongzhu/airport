#!/usr/bin/env bash
# push-secrets.sh — 從 .env 讀取業務 Secrets，逐個寫入 Worker（自動化清單 §12.8 #7）
#
# 用法：
#   cd workers/api
#   ../deploy/cloudflare/push-secrets.sh /path/to/.env
#
# .env 需含（缺省的鍵自動跳過）：
#   JWT_SECRET  BEPUSDT_API_URL  BEPUSDT_TOKEN  BEPUSDT_SECRET
#   PAYPAL_CLIENT_ID  PAYPAL_CLIENT_SECRET  PAYPAL_WEBHOOK_ID
#   TURNSTILE_SECRET
#
# ⚠️ 切換日：JWT_SECRET 必須用舊 Go .env 原值（保舊會話 token 不失效）。
set -euo pipefail

ENV_FILE="${1:?usage: $0 /path/to/.env}"
[[ -f "$ENV_FILE" ]] || { echo "not found: $ENV_FILE"; exit 1; }

KEYS=(
  JWT_SECRET
  BEPUSDT_API_URL BEPUSDT_TOKEN BEPUSDT_SECRET
  PAYPAL_CLIENT_ID PAYPAL_CLIENT_SECRET PAYPAL_WEBHOOK_ID
  TURNSTILE_SECRET
)

for key in "${KEYS[@]}"; do
  val=$(grep -E "^${key}=" "$ENV_FILE" | tail -1 | cut -d= -f2- | tr -d '"' || true)
  if [[ -z "$val" ]]; then
    echo "skip (unset in .env): $key"
    continue
  fi
  printf '%s' "$val" | npx wrangler secret put "$key"
  echo "done: $key"
done

echo "—— 全部 Secrets 已供應。注意：切勿把 .env 提交進 git。"
