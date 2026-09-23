-- Subscription-side device slots (see docs/devices.md).
-- max_devices: 0 = unlimited; default 5 (DEFAULT_MAX_DEVICES in lib/devices.ts).
-- products.max_devices: NULL = do not override user limit on grant; set to copy onto user.

ALTER TABLE users ADD COLUMN max_devices INTEGER NOT NULL DEFAULT 5;

ALTER TABLE products ADD COLUMN max_devices INTEGER;

CREATE TABLE IF NOT EXISTS user_devices (
  id                 INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id            INTEGER NOT NULL,
  device_fingerprint TEXT    NOT NULL,
  device_name        TEXT,
  platform           TEXT,
  user_agent         TEXT,
  last_seen          INTEGER NOT NULL,
  created_at         INTEGER NOT NULL,
  UNIQUE (user_id, device_fingerprint)
);
CREATE INDEX IF NOT EXISTS idx_user_devices_user ON user_devices (user_id);
