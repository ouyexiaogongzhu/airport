-- admin 自建 at_ 訂閱令牌：每個 token = 一行 account_type='token_only' 的合成用戶，
-- 訂閱/流量記賬/設備槽/節點用戶列表全部復用 users 表既有鏈路。'registered' 為既有用戶。

ALTER TABLE users ADD COLUMN account_type TEXT NOT NULL DEFAULT 'registered';
