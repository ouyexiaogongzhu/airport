-- traffic_records 每日彙總表：cron 聚合 14 天前的舊數據後刪除明細，控制表無限增長
CREATE TABLE IF NOT EXISTS traffic_daily (
  day            TEXT PRIMARY KEY, -- 'YYYY-MM-DD'（substr(recorded_at,1,10)）
  upload_bytes   INTEGER NOT NULL DEFAULT 0,
  download_bytes INTEGER NOT NULL DEFAULT 0,
  records        INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_traffic_records_recorded_at ON traffic_records (recorded_at);
