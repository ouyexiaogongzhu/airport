-- rfplay D1 schema — 落自 GORM AutoMigrate（manager/internal/model/*.go）
-- created_at/updated_at 沿用 GORM SQLite 的 TEXT datetime 存儲，遷移數據格式不變
-- 遷移現有數據：sqlite3 manager.db ".dump" 整理後 wrangler d1 import rfplay --remote --file=seed.sql

CREATE TABLE IF NOT EXISTS users (
  id                   INTEGER PRIMARY KEY AUTOINCREMENT,
  username             TEXT    NOT NULL UNIQUE,
  password_hash        TEXT    NOT NULL,
  role                 TEXT    NOT NULL DEFAULT 'user',
  balance              REAL    DEFAULT 0,
  status               TEXT    NOT NULL DEFAULT 'active',
  client_token         TEXT    UNIQUE,
  subscription_status  TEXT    NOT NULL DEFAULT 'pending',
  subscription_tier    TEXT,
  traffic_limit_bytes  INTEGER DEFAULT 0,
  traffic_used_bytes   INTEGER DEFAULT 0,
  expire_time          INTEGER DEFAULT 0,
  rate_limit_bps       INTEGER DEFAULT 0,
  traffic_period_start INTEGER DEFAULT 0,
  vless_uuid           TEXT,
  ss_password          TEXT,
  trojan_password      TEXT,
  created_at           TEXT,
  updated_at           TEXT
);
CREATE INDEX IF NOT EXISTS idx_users_sub_status_expire ON users (subscription_status, expire_time);

CREATE TABLE IF NOT EXISTS nodes (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  name               TEXT    NOT NULL,
  type               TEXT    NOT NULL,
  address            TEXT    NOT NULL,
  port               INTEGER NOT NULL,
  protocol           TEXT    NOT NULL,
  status             TEXT    NOT NULL DEFAULT 'inactive',
  traffic_up         INTEGER DEFAULT 0,
  traffic_down       INTEGER DEFAULT 0,
  user_id            INTEGER NOT NULL,
  network            TEXT    DEFAULT 'ws',
  security           TEXT    DEFAULT 'none',
  ws_path            TEXT,
  server_name        TEXT,
  reality_public_key TEXT,
  reality_short_id   TEXT,
  token              TEXT    UNIQUE,
  last_heartbeat     TEXT,
  created_at         TEXT,
  updated_at         TEXT
);
CREATE INDEX IF NOT EXISTS idx_nodes_user_id ON nodes (user_id);

CREATE TABLE IF NOT EXISTS orders (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL,
  product_id  INTEGER NOT NULL,
  amount      REAL    NOT NULL,
  status      TEXT    NOT NULL DEFAULT 'pending',
  provider    TEXT    DEFAULT 'mock',
  payment_url TEXT,
  created_at  TEXT,
  updated_at  TEXT
);
CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders (user_id);
CREATE INDEX IF NOT EXISTS idx_orders_product_id ON orders (product_id);
CREATE INDEX IF NOT EXISTS idx_orders_status_created ON orders (status, created_at);

CREATE TABLE IF NOT EXISTS products (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  name       TEXT    NOT NULL,
  type       TEXT    NOT NULL,
  price      REAL    NOT NULL,
  stock      INTEGER DEFAULT 0,
  status     TEXT    DEFAULT 'active',
  currency   TEXT,               -- 遷移新增（§5.2 多幣種）：USD / CNY，舊數據為 NULL
  created_at TEXT,
  updated_at TEXT
);

CREATE TABLE IF NOT EXISTS traffic_records (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  node_id       INTEGER NOT NULL,
  user_id       INTEGER NOT NULL,
  upload_bytes  INTEGER NOT NULL,
  download_bytes INTEGER NOT NULL,
  recorded_at   TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_traffic_node_time ON traffic_records (node_id, recorded_at);
CREATE INDEX IF NOT EXISTS idx_traffic_user_time ON traffic_records (user_id, recorded_at);
