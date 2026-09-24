-- Google OAuth：以 google_sub 綁定帳號；email 已在 0003 有 UNIQUE（可空）

ALTER TABLE users ADD COLUMN google_sub TEXT;

-- 可空；非空時唯一（SQLite UNIQUE 允許多個 NULL）
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub ON users (google_sub);
