-- 用戶資料欄位：聯絡方式 / 顯示名 / 輕量帳單地址（訂單列表仍走既有 /user/orders）

ALTER TABLE users ADD COLUMN email TEXT;
ALTER TABLE users ADD COLUMN phone TEXT;
ALTER TABLE users ADD COLUMN display_name TEXT;
ALTER TABLE users ADD COLUMN billing_address TEXT;

-- email 可空；非空時唯一（SQLite UNIQUE 允許多個 NULL）
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users (email);
