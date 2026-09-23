-- 修復計劃 A1：商品單獨配置時長/流量/限速；會話吊銷版本號；清理下線協議與缺失憑證

-- traffic_bytes 為每 30 天的流量額度，0 = 不限；speed_limit_bps 0 = 不限速
ALTER TABLE products ADD COLUMN duration_days   INTEGER NOT NULL DEFAULT 30;
ALTER TABLE products ADD COLUMN traffic_bytes   INTEGER NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN speed_limit_bps INTEGER NOT NULL DEFAULT 0;
ALTER TABLE products ADD COLUMN description     TEXT;

ALTER TABLE users ADD COLUMN token_version INTEGER NOT NULL DEFAULT 0;

-- 節點端與訂閱端必須用同一個 UUID；缺失的一次性補齊（UUIDv4 格式）
UPDATE users SET vless_uuid =
  lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' ||
  substr(lower(hex(randomblob(2))), 2) || '-' ||
  substr('89ab', 1 + (abs(random()) % 4), 1) || substr(lower(hex(randomblob(2))), 2) || '-' ||
  lower(hex(randomblob(6)))
WHERE vless_uuid IS NULL OR vless_uuid = '';

UPDATE nodes SET status = 'inactive' WHERE protocol IN ('shadowsocks', 'trojan');
