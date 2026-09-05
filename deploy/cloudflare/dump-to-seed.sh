#!/usr/bin/env bash
# dump-to-seed.sh — GORM SQLite 庫 → D1 可導入 seed.sql（自動化清單 §12.8 #8）
#
# 用法：
#   ../deploy/cloudflare/dump-to-seed.sh /path/to/manager.db > seed.sql
#   cd workers/api && wrangler d1 import rfplay --remote --file=seed.sql
#
# 處理：剔除 sqlite 內部表/事務包裹，schema 已由 migrations/0001_schema.sql 建立，
# 因此只保留 5 張業務表的 INSERT（列與 D1 schema 對齊，products 自動補 currency=NULL）。
set -euo pipefail

DB_FILE="${1:?usage: $0 /path/to/manager.db}"
[[ -f "$DB_FILE" ]] || { echo "not found: $DB_FILE"; exit 1; }

TABLES="users nodes orders products traffic_records"

echo "-- RFPlay seed — 自 manager.db 生成於 $(date -u +%FT%TZ)"
echo "-- 執行前提：migrations/0001_schema.sql 已在目標 D1 執行"

for t in $TABLES; do
  # 先清空，保證可重跑（冪等）
  echo "DELETE FROM $t;"
done

for t in $TABLES; do
  echo "-- ── $t ──"
  sqlite3 "$DB_FILE" \
    -cmd ".mode insert $t" \
    -cmd ".headers off" \
    "SELECT * FROM $t;" || { echo "-- [warn] 表 $t 導出失敗，跳過"; continue; }
done
