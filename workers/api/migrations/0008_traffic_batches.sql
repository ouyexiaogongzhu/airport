-- 流量上報批次去重：gateway 對未收到 200 的批次原樣重發（同 batch_id），
-- 本表命中即視為重複上報，不再累計流量。約 5% 概率順帶清理 7 天前記錄。
CREATE TABLE IF NOT EXISTS traffic_batches (
  batch_id TEXT PRIMARY KEY,
  node_id INTEGER NOT NULL,
  recorded_at TEXT NOT NULL
);
