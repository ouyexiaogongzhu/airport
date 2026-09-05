# Cloudflare 遷移方案 — 退役 Go，Manager 全量 TypeScript 重寫上 Workers

> **狀態**: 規劃（2026-09-05）
> **決策**: 徹底退役 Go manager（Fiber + GORM/SQLite + Docker），以 Workers (TS/Hono) + D1 + KV + R2 重寫。
> **客戶端決策（2026-09-05）**: **退役 Flutter 客戶端（`client/`）**，用戶端改用**通用 Clash 客戶端**（Clash Meta/mihomo 系）+ 訂閱 URL 導入，自研 App 歸零。
> **關聯**: [README.md](README.md)、[airport_system_design.md](airport_system_design.md)

---

## 1. 目標與非目標

| 目標 | 非目標 |
| :--- | :--- |
| 退役 `manager/` 全部 Go 代碼與其 Docker 部署 | 不退役 xray 節點進程；**節點統一為 CF-WS + Tunnel 回源（零公網端口），Reality 已從方案刪除** |
| **退役 `client/` Flutter 客戶端**，訂閱導入交給通用 Clash 客戶端（用戶自備） | 不改 admin / daemon 代碼（契約保持則無需動）；portal 僅一處小改（訂閱連結默認輸出 Clash 格式） |
| API 契約 100% 不變：portal/admin（cookie+CSRF）、訂閱 URL（無鑑）、daemon（HMAC） | 不引進付費依賴；超額對策僅備檔 `$5/月 Workers Paid` |
| 基礎設施全部遷到 CF 免費層（Workers/D1/KV/R2/Pages/Tunnel/Access/Turnstile） | 不重寫 daemon（它本來就是 Node，只需改 `DAEMON_MANAGER_URL`） |
| BEpusdt 支付進程留 VPS（第三方 Go 服務，不屬於本次重寫範圍）；**新增 PayPal 通道（大陸用戶可用）**，XunhuPay 已從計劃刪除 | |

---

## 2. 現狀盤點（`manager/` 全量清單）

### 2.1 路由（`cmd/server/main.go`，約 50 條）

| 分組 | 端點 | 鑑權 | Workers 對策 |
| :--- | :--- | :--- | :--- |
| 健康檢查 | `GET /health` | 無 | Worker 內 `__scheduled`/健康路由 |
| 驗證碼 | `GET /captcha` | 無 | **刪除**，換 Turnstile（無後端狀態） |
| 公開 | `POST /public/register` `/login` | 無 + 限流 | Worker + Turnstile 校驗 |
| ~~公開~~ | `POST /public/token-login` | — | **刪除**：Flutter token 導入專屬，退役後全庫無消費者 |
| 支付回調 | `POST /public/payment/callback[/:provider]` | 簽名 | Worker + 簽名驗證 |
| 客戶端訂閱 | `GET /client/config`、`GET /client/links/:token{,/clash,/singbox,/qrcode}` | 無（token 即憑證）+ 限流 | **Worker 第一批**（最高頻）；**`/clash` 為主力輸出** |
| 客戶端訂閱（JWT） | `GET /client/subscription` | Bearer JWT | Worker；**消費者是 portal 三個頁面**（Dashboard/Account/Subscription），必須保留 |
| Web 會話 | `GET /auth/csrf` `/validate`、`POST /auth/refresh` `/logout` | httpOnly cookie JWT | Worker（WebCrypto HMAC 重簽） |
| Admin 會話 | `POST /admin/auth/login` `/logout`、`GET /admin/auth/csrf` | cookie | 同上 |
| 節點 daemon | `GET /node/:token/config`、`POST /node/:token/traffic/report` | node token + HMAC | Worker（第一批/第二批） |
| 用戶 | `GET/PUT /user/profile`、`POST/GET /user/orders[/:id]` | cookie + CSRF | Worker |
| 訂閱憑證 | `GET /web/client-token`、`POST /web/client-token/regenerate` | cookie + CSRF | Worker |
| Admin 管理 | users / orders(+refund) / stats / nodes(+token/config) / traffic / products | admin cookie + CSRF | Worker |

### 2.2 數據層（GORM + SQLite，5 張表 → D1）

```
users(id, username✦, password_hash, role, balance, status, client_token✦,
      subscription_status, subscription_tier, traffic_limit_bytes, traffic_used_bytes,
      expire_time, rate_limit_bps, traffic_period_start, vless_uuid, ss_password,
      trojan_password, created_at, updated_at)
nodes(id, name, type, address, port, protocol, status, traffic_up/down, user_id,
      network, security, ws_path, server_name, reality_public_key, reality_short_id,
      token✦, last_heartbeat, created_at, updated_at)
orders(id, user_id, product_id, amount, status, provider, payment_url, created_at, updated_at)
products(id, name, type, price, stock, status, created_at, updated_at)
traffic_records(id, node_id, user_id, upload_bytes, download_bytes, recorded_at)
```
✦ = uniqueIndex。全部是簡單關聯，無 stored procedure / 觸發器 → D1 直遷。

### 2.3 對遷移有利的既有事實

- **會話無狀態**：HS256 JWT 裝在 httpOnly cookie（`session`/`admin_session` + `refresh` + `csrf` double-submit），**無服務端 session 存储** → Workers 零狀態重現，只需 WebCrypto + 同名 cookie。
- **無後台 cron**：`internal/cron/` 是空目錄；訂閱過期是讀時計算 → 不需要 Cron Triggers 也能跑（可選加）。
- **限流是進程內 `sync.Map`**（本來就有 TODO 要換 TTL-LRU）→ Workers 上天然失效（多 isolate），直接刪，改用 CF WAF 免費規則 + Turnstile，升級路徑為 DO 限流器。
- **daemon 是 Node**：只調 `GET /node/:token/config` + `POST /node/:token/traffic/report`，改一個環境變量即指向 Worker。
- **portal 已有 Clash 引導**：`SetupGuide.vue` 直接指向 clash-verge-rev 下載並教用戶貼 `/clash` 訂閱——「通用客戶端」方向 portal 側已就緒，本次只需把訂閱連結默認輸出改為 `/clash` 格式。

---

## 3. 目標架構

```
                              Cloudflare（免費層）
  ┌──────────────────────────────────────────────────────────────────┐
  │  Pages: www.rfplay.uk（portal：複製 /clash 訂閱連結 + 二維碼）    │
  │         admin.rfplay.uk（+ Access 零信任門）                      │
  │                                                                  │
  │  Worker「rfplay-api」= 新 manager（TS + Hono，單 Worker）          │
  │   ├─ routes: api.rfplay.uk/api/v1/*（路徑級灰度切流）              │
  │   ├─ D1   rfplay（users/nodes/orders/products/traffic_records）   │
  │   ├─ KV   訂閱緩存 60s / traffic report 聚合暫存                   │
  │   ├─ R2   backups（backup.sh 歸檔）                                │
  │   ├─ Turnstile 校驗（register/login）                              │
  │   ├─ Secrets: JWT_SECRET / BEPUSDT_TOKEN / BEPUSDT_SECRET / ...   │
  │   └─ Workers Cron（可選：traffic 彙總落 D1）                       │
  │                                                                  │
  │  Tunnel 主機名：pay.rfplay.uk → VPS BEpusdt（僅 Worker 可達）      │
  └───────┬───────────────────────────┬──────────────────────────────┘
          │                           │
   通用 Clash 客戶端              daemon（Node，改 URL 即可）
 （用戶自備：Clash Meta/mihomo、         │
   Clash Verge、ClashX、Stash）   VPS（保留，無公網端口）
          │                     ├─ xray 節點（vless+ws，cloudflared Tunnel 回源）
          ▼                     └─ BEpusdt（USDT 收款網關，Go 第三方）
  portal 複製訂閱 URL → 貼進 Clash 客戶端 → 訂閱/連接
  用戶瀏覽器 → BEpusdt 託管收銀台（購買，僅 portal）
```

**支付數據流**（重點）：

```
①  portal 下單    → Worker POST /user/orders（cookie+CSRF）
②  Worker → BEpusdt  POST pay.rfplay.uk/api/v1/order/create-transaction
                  （MD5 簽名 + Bearer token）→ 拿託管收銀台 URL 存 orders.payment_url
③  用戶付款       → BEpusdt 託管頁（USDT 鏈上確認）
④  BEpusdt → Worker  POST /public/payment/callback/bepusdt（MD5 簽名回調）
⑤  Worker 驗簽    → D1 batch：orders.status=paid ＋ 用戶激活/順延 expire_time（原子）
```

**單 Worker 原則**：不搞微服務。一個 `rfplay-api` 用 Hono 路由分層（public/client/web/admin/node），共享 D1 binding。日後有獨立伸縮需求再拆。

---

## 4. 新代碼庫結構

```
workers/api/
├── wrangler.jsonc          # bindings: D1/KV/R2, routes, crons
├── package.json            # hono, @cloudflare/vitest-pool-workers
├── src/
│   ├── index.ts            # Hono app：中間件鏈（CORS/cookie/CSRF/error）
│   ├── routes/
│   │   ├── public.ts       # register/login/token-login/captcha→turnstile
│   │   ├── client.ts       # config + links/:token{,/clash,/singbox,/qrcode} + subscription
│   │   ├── auth.ts         # csrf/validate/refresh/logout + admin/auth
│   │   ├── web.ts          # client-token, profile, orders
│   │   ├── admin.ts        # users/orders/nodes/products/traffic/stats
│   │   └── node.ts         # daemon config + traffic report（HMAC）
│   ├── lib/
│   │   ├── jwt.ts          # WebCrypto HS256 簽發/校驗（對齊 jwt/v5 claims）
│   │   ├── cookies.ts      # httpOnly cookie 寫/清（對齊 auth.go）
│   │   ├── csrf.ts         # double-submit 恆定時間比較
│   │   ├── nodehmac.ts     # 對齊 middleware/node_auth.go 簽名算法
│   │   ├── subformats.ts   # base64 / Clash YAML / sing-box JSON 生成（對齊 subscription.go）
│   │   └── xrayuri.ts      # vless/vmess/ss/trojan URI 生成（對齊 links.go）
│   └── types.ts            # Env bindings + 行類型
├── migrations/             # D1 SQL（從 GORM AutoMigrate 落成）
│   ├── 0001_schema.sql
│   └── 0002_indexes.sql
└── test/                   # vitest-pool-workers（契約對拍用例）
```

**對拍策略**：Go manager 繼續在 VPS 跑；每條端點寫「同請求 → 比較 Go 響應 vs Worker 響應」的合約測試，全綠才切流。

---

## 5. 關鍵技術映射（Go → Workers）

| Go 現狀 | Workers 對策 | 備註 |
| :--- | :--- | :--- |
| `jwt/v5` HS256（Bearer + cookie 雙通道） | WebCrypto `crypto.subtle` HMAC，claims 對齊（`user_id/username/role/exp`） | Secret 存 Worker Secrets；cookie 名/TTL/Secure/SameSite 逐項對齊 `auth.go:383-` |
| CSRF double-submit（`subtle.ConstantTimeCompare`） | 恆定時間比較（自寫 10 行，別拉依賴） | `csrf.ts` |
| GORM SQLite + AutoMigrate | D1 + SQL migrations（把 AutoMigrate 落成 `0001_schema.sql`） | `wrangler d1 execute --local` 起步 → `d1 import` 遷數據 |
| `sync.Map` 限流（無限增長 TODO） | **刪除**：CF WAF 免費自定義規則（IP 限速）+ Turnstile 擋 register/login | 升級路徑：DO（SQLite 免費層）做 /public/* 限流 |
| 圖形驗證碼 `/captcha` | **刪除**，Turnstile siteverify（`fetch` 一個 POST） | 後端零狀態 |
| links v2ray base64 / Clash YAML / sing-box JSON | 純字符串生成，TS 重寫（`subformats.ts`） | **`/clash` 是唯一用戶入口**（Flutter 退役後），對拍優先級最高；`Subscription-Userinfo` 響應頭是用戶唯一的流量/到期展示渠道（Clash 客戶端原生讀取），必保；當前節點協議 ss/vmess/vless/trojan 全部落在 Clash 範圍（vless 需 Meta 內核 mihomo） |
| qrcode PNG（Go 裡是 stub） | `qrcode` npm → SVG（純 JS）先交付，PNG 需 pngjs | 或砍掉——portal 已有二維碼，見 §8-P1 註記 |
| node token + HMAC（`node_auth.go`） | `nodehmac.ts` 逐字節對齊簽名算法 | daemon 無感 |
| BEpusdt webhook 簽名校驗 | TS 重寫驗簽；訂單狀態機不變 | 回調 URL 指向 Worker 路由（Tunnel 域名） |
| Stripe provider | TS 重寫（官方 REST，無 SDK） | 或同階段擱置，見 §8 |
| 訂單/餘額事務 | D1 `batch()`（原子批次） | 無跨庫事務需求 |

### 5.1 支付鏈路細節（BEpusdt / Payoneer / Stripe / Mock）

Go 端實現 = `payment.go`(496行) + `payment_provider.go`(295行) + `provider_stripe.go`(145行)，TS 重寫約 400 行：

| Provider | 現狀 | TS 重寫要點 |
| :--- | :--- | :--- |
| **BEpusdt**（主力，USDT） | ✅ 完整：建單（`POST /api/v1/order/create-transaction`，MD5 簽名 + Bearer token）→ 託管收銀台 URL；回調 8 字段 MD5 驗簽，`status==2` 為 paid，**無 secret 配置直接拒收**（fail closed） | ⚠️ **WebCrypto 不支持 MD5** → 自帶 40 行純 JS `md5.ts`（BEpusdt 是遺留簽名方案，無法更換，只能重現）；建單/回調的欄位順序、金額格式（`FormatFloat(-1)`）逐項對拍 |
| **PayPal**（**新增**，大陸用戶可用） | Go 端無此通道，本次新增 | REST **Orders v2** 建單（OAuth2 client credentials，access_token 緩存 KV，避免每次建單取 token）→ 返回 `approve` 託管頁 URL 存 `orders.payment_url`；webhook `PAYMENT.CAPTURE.COMPLETED` → 調 `/v1/notifications/verify-webhook-signature` 官方接口驗簽（免自行處理證書鏈）；費率 ~4.4%+固定費，**拒付窗口 180 天**；資金提現到國內銀行（美元結匯） |
| Payoneer | ⚠️ 半成品：`CreatePayment` 返回 `checkout.payoneer.example` 佔位 URL；回調 HMAC-SHA256 驗簽已寫好 | HMAC-SHA256 用 WebCrypto 原生；**未投產 → 整個 provider 延後**（保留接口形狀即可） |
| Stripe | 未投產 | **待定**：等真實跨境收款需求出現再評估（需海外主體）；接口形狀與 §5.1 其他通道相同，要做時 ≈ 100–150 行 |
| Mock | 開發用 | 保留（本地 vitest 對拍用） |

**訂單激活事務**（`activateSubscriptionTx` → D1）：

```ts
// 回調驗簽通過後（冪等：order 已是 paid 直接返回 ok）
env.DB.batch([
  env.DB.prepare("UPDATE orders SET status='paid', updated_at=? WHERE id=? AND status='pending'"),
  env.DB.prepare("UPDATE users SET subscription_status='active', subscription_tier=?, expire_time=? WHERE id=?"),
]);
// expire_time = max(now, 現有 expire_time) + 產品時長 —— 順延語義與 Go 一致
```

**Worker Secrets 清單**：`JWT_SECRET`、`BEPUSDT_API_URL`、`BEPUSDT_TOKEN`、`BEPUSDT_SECRET`（回調驗簽，缺省回落 TOKEN，與 Go 一致）、`PAYPAL_CLIENT_ID`、`PAYPAL_CLIENT_SECRET`、`PAYPAL_WEBHOOK_ID`、`TURNSTILE_SECRET`。

**網絡位址**：BEpusdt 留 VPS，掛 Tunnel 主機名 `pay.rfplay.uk`；Worker 服務端調用與 BEpusdt 回調全走這個域名，VPS 不開任何公網端口。回調 URL 在建單時傳 `notify_url = https://api.rfplay.uk/api/v1/public/payment/callback/bepusdt`（即 Worker route）。

**無需獨立 VPS**：支付邏輯（建單/驗簽/激活）全在 Worker + D1，唯一 VPS 組件是 BEpusdt 進程，與 xray 節點同居即可——Tunnel 已消除「支付暴露 IP 被節點牽連」的舊問題，BEpusdt 無狀態（訂單真源在 D1），掛掉只暫停新收款、`docker run` 數分鐘恢復。若擔心節點 VPS 被供應商封號連累收款，屆時拆一台 $3–4/月 小雞專跑 BEpusdt + Tunnel 即可（隨時可拆，不必預先承擔）。做到支付 0 VPS 的可選路線：換託管型 USDT 收單（Cryptomus/NOWPayments，webhook 同構），代價是收款地址託管第三方 + 費率 ~0.4–1%，不默認。

**BEpusdt 資源**（官方 `docs/faq/server.md`）：最低 1 核 / 1GB / 10GB SSD；單鏈掃塊內存 <100MB，多鏈線性增長；**持續掃塊日流量數 GB**（唯一真實成本項）；必配 NTP；避免網絡受限地區。本項目 VPS 節點流量本為大戶、SSD、牆外機房，全部天然滿足——**建議只開 TRON 單鏈**（USDT-TRC20 為主流付款方式），內存與掃塊流量最小化，僅需留意 VPS 月流量配額。

**冪等與重試**：BEpusdt 回調會重試；Worker 端以 `order.status='pending'` 為更新條件（已 paid 的重複回調零副作用），語義與 Go 一致。

### 5.2 新增通道：PayPal（大陸用戶可用）

定位：BEpusdt 只覆蓋 USDT；PayPal 補上**卡類跨境通道**——大陸用戶註冊 PayPal 後可綁銀聯/Visa/Master 卡付款。XunhuPay（人民幣掃碼）已從計劃刪除；Stripe / Paddle 等 MoR **待定**，接口形狀相同。

**集成流程**（與 BEpusdt 同構，走同一條激活事務）：

```
① portal 下單     POST /user/orders（provider="paypal"）
② Worker → PayPal  OAuth2 client credentials 取 access_token（KV 緩存）
                   → POST /v2/checkout/orders（intent=CAPTURE）
                   → 返回 approve 託管頁 URL，存 orders.payment_url
③ 用戶付款         PayPal 託管頁登錄/綁卡付款
④ PayPal → Worker  POST /public/payment/callback/paypal
                   （webhook 事件 PAYMENT.CAPTURE.COMPLETED）
⑤ Worker 驗簽       POST /v1/notifications/verify-webhook-signature（官方驗簽接口，
                   免自行處理證書鏈）→ 同 §5.1 的 D1 batch 激活事務（冪等、順延語義共用）
```

**TS 實現量**：~180 行（`provider_paypal.ts`）——OAuth2 token 管理 + Orders 建單 + webhook 驗簽，全部官方 REST，無 SDK（Workers 上不裝 Node SDK，直接 `fetch`）。`PaymentProvider` 接口與 Go 時代形狀一致。沙箱：`api-m.sandbox.paypal.com` 先行對拍，生產切 `api-m.paypal.com`。

**多幣種**：`products` 表加 `currency` 欄位（`0001_schema.sql` 直接帶上）：PayPal 產品以 **USD** 計價（付款人卡自動換匯），BEpusdt 維持現行錨定計價，未來 Stripe/MoR 直接復用。

**依賴與風險**：
- **費率高**：跨境 ~4.4%+固定費 + 貨幣轉換價差；**拒付（chargeback）窗口 180 天** → 訂閱交付設計為「到期/止付即停」，遭遇惡意拒付時損失封頂
- **大陸付款體驗**：無人民幣掃碼，付款人需註冊 PayPal 並綁卡（銀聯），轉化率低於支付寶/微信——USDT 為保底通道；若日後要恢復人民幣掃碼，按 §5.1 形狀接易支付類即可（MD5 復用 `md5.ts`）
- 商戶賬戶實名 + 提現到國內銀行（美元結匯，個人年度便利化額度內）
| Docker/ nginx / certbot | 全部退役：Worker route + Pages + Tunnel | VPS 只剩 xray + BEpusdt |

---

## 6. 數據遷移（GORM SQLite → D1）

1. **Schema 落地**：本機起 Go manager 跑一次 AutoMigrate → `sqlite3 .dump` 整理為 `migrations/0001_schema.sql`（人工審一遍索引）。
2. **數據導出**：`sqlite3 manager.db ".mode insert" > seed.sql`（或 `wrangler d1 export` 反向不可用，用 dump）。
3. **導入**：`wrangler d1 import rfplay --file=seed.sql --remote`。
4. **敏感字段**：`password_hash`(bcrypt)、`client_token`、`node.token` 全部原樣遷移——**哈希算法不變**，用戶無感。
5. **雙寫窗口不需要**：Go 退役採「停機窗口切換」（小用戶量，維護頁 5 分鐘），比雙寫簡單一個量級。

---

## 7. 額度風險與對策（免費層實測數據）

| 資源 | 免費額度 | 本項目預估 | 風險/對策 |
| :--- | :--- | :--- | :--- |
| Workers | 10 萬 req/天，10ms CPU/次 | 訂閱拉取 + verify 為大頭 | 冷啟動夠；**verify 每連接一次**，用戶量漲先爆 → $5/月（1000 萬 req/月）一檔全解 |
| D1 | 500 萬行讀/天，**10 萬行寫/天**，5GB | 讀為主 | ⚠️ **traffic report 是寫入大戶**：方案見 §8-P2「寫入合併」；公式 = 節點數×1440×上報頻率倒數 + 用戶彙總行 |
| KV | 10 萬讀/天，**1 千寫/天** | 訂閱/token 只讀緩存 | 僅 token 變更時寫；禁止 per-request 寫 |
| R2 | 10GB，出口免費 | DB 每日備份 | 隨便用 |
| Pages | 請求無限，500 builds/月 | portal/admin | 無 |
| Workers Logs | 20 萬 events/天 | 替代自建 Loki | 留 3 天；歸檔走 R2 |
| DO (SQLite) | 10 萬 req/天 | （可選限流器） | 先不用，見上 |
| Turnstile / Access / Tunnel / Email Routing | 免費 | P0 直接吃 | 無 |

---

## 8. 分階段執行計劃

### P0 — 零代碼紅利（半天）
1. `cloudflared` Tunnel 收編 api 源站與 BEpusdt webhook 域名；VPS 防火牆關 443/80，退役 nginx + certbot 容器
2. **退役 `client/` Flutter 目錄**（git 留檔即可恢復）；README 架構表同步為「通用 Clash 客戶端」
3. CF Access（免費 50 席）套 `admin.rfplay.uk`
4. Turnstile 申請 site key/secret（portal 改造放在 P1 一起驗收）
5. `backup.sh` 加 rclone 推 R2（S3 API）
6. Email Routing：`support@rfplay.uk` → 個人郵箱
7. Claude Code 安裝官方 skills：`/plugin marketplace add cloudflare/skills` → `/plugin install cloudflare@cloudflare`

### P1 — Worker 骨架 + 訂閱數據面（1–2 天）
1. `workers/api` 腳手架（Hono + wrangler + vitest-pool-workers）；D1 schema + `d1 import`
2. 實現：`/health`、`/client/config`、`/client/links/:token{,/clash,/singbox}`、`/client/subscription`（JWT）、`/node/:token/config`（HMAC）——**先切最高頻的讀路徑**
3. KV 緩存層（60s）替換原 `linkRateLimiters` 位置
4. route 灰度：`api.rfplay.uk/api/v1/client/*` 與 `/api/v1/node/*` 掛 Worker route（路徑級，**舊訂閱 URL 不死**）；Go 與 Worker 並行對拍
5. qrcode：先交付 SVG；PNG 確認 portal 端無依賴後直接砍（YAGNI）

### P2 — 節點上報 + 寫入合併（1 天）
1. `POST /node/:token/traffic/report` → Worker
2. **寫入合併**（D1 寫額度保命）：
   - daemon 每 60s 的報告先進 KV（累加），Workers Cron（每 5 分鐘）批量 `INSERT` traffic_records + `UPDATE users.traffic_used_bytes`
   - 寫入公式 ≈ 288(次/天)×(節點數+用戶數) → 100 用戶約 3 萬行寫/天，安全
3. daemon 只改 `DAEMON_MANAGER_URL` 指向 Worker

### P3 — 會話與管理面（2.5–3.5 天）
1. `lib/jwt.ts` 對拍 Go 簽發的 token（**同一 `JWT_SECRET` 下 Worker 能驗舊 token**，會話不斷）
2. `/public/*`（register/login）+ Turnstile siteverify；刪圖形驗證碼
3. `/auth/*`、`/admin/auth/*`、`/user/*`、`/web/*`、`/admin/*` 全量端點 + CSRF
4. 支付（第一批兩通道）：BEpusdt（USDT，驗簽 + 訂單狀態機）+ **PayPal（新增，§5.2）**；共用 D1 激活事務；XunhuPay 已刪除；Stripe/MoR **待定**
5. portal 小改（~1 小時）：`subscriptionUrl.ts` **雙格式展示**——默認 `/clash` 連結（Clash Meta 系）+ 保留 base64 通用連結 `/links/:token`（v2rayA/v2rayN/OpenWrt 路由器等非 Clash 客戶端）（含對應測試 `subscriptionUrl.test.ts`）；SetupGuide 文案確認；支付頁 provider 選項為「USDT / PayPal」
6. portal/admin 其餘零改動驗收（cookie 名/CSRF 頭不變）

### P4 — 退役收尾（半天）
1. 維護窗口：DNS/route 全量切 Worker → `docker compose down manager nginx certbot`
2. 刪 `manager/` 目錄（git 歷史留檔）；更新 README 架構表
3. `manager-data` volume 最後一次備份推 R2 後刪除
4. VPS 只剩：xray 節點 + BEpusdt（連 Tunnel）

---

## 9. 驗收與測試

- **契約對拍**：`test/` 內每端點「Go 響應 vs Worker 響應」斷言（JSON 結構、cookie 屬性、`Subscription-Userinfo` 頭、YAML 可被 clash 解析）
- **通用 Clash 客戶端**：Clash Meta / Clash Verge / Stash 導入 `/links/:token/clash` 訂閱 → 連接真實節點 → 訂閱詳情頁顯示流量/到期（`Subscription-Userinfo`）
- **portal**：Dashboard/Account/Subscription 三頁數據正常（`/client/subscription` 走 Worker）；複製的訂閱連結為 `/clash` 格式
- **daemon**：verify/sync 對 Worker 跑 24h 無誤
- **D1 寫入量監控**：Dashboard → D1 metrics 每日檢查，逼近 80% 額度即觸發 §8-P2 進一步降頻

---

## 10. 風險清單

| 風險 | 緩解 |
| :--- | :--- |
| Workers 無 Go runtime（本方案的起因） | 全量 TS 重寫，§4 結構已劃分到檔案級 |
| D1 寫額度爆（traffic report） | P2 寫入合併 + Cron 批量，§8-P2 公式 |
| HMAC/細節對不齊導致 daemon 全掛 | `nodehmac.ts` 逐字節對拍 + 灰度 route（node 流量最後切） |
| 舊 JWT 在切換日失效 | 同 `JWT_SECRET` 遷移，Worker 直接驗舊 token（§8-P3.1） |
| 免費額度隨用戶增長見頂 | 升級路徑單一：$5/月 Workers Paid（額度×100），無架構變更 |
| 單 Worker request/天超限 | 臨界前拆「node 面」獨立 Worker（路由已分層，拆分成本低） |
| 用戶端換成第三方 Clash，行為/版本不可控 | 訂閱格式對拍主流內核（Clash Meta/Verge/Stash）；portal SetupGuide 引導下載 clash-verge-rev；vless 節點要求 Meta 內核（文檔與 SetupGuide 標註） |
| 原 Flutter 用戶（若有側載）失去 App | 訂閱 URL 本就是憑證，直接貼進任意 Clash 客戶端即可遷移；portal SetupGuide 承接 |
| PayPal 高費率/拒付（180 天窗口） | 訂閱止付即停，惡意拒付損失封頂；**USDT（BEpusdt）保底通道永遠在線** |
| 無人民幣掃碼通道（XunhuPay 已刪除） | 大陸用戶走 PayPal 綁卡（銀聯）或 USDT；要恢復 RMB 掃碼按 §5.1 形狀接易支付類即可（MD5 復用 `md5.ts`）；大陸個人通道實名不可避免——需身份隔離用境外渠道或 USDT，惟身份隔離不消除法律風險本身 |
| 全部節點流量走 CF 邊緣（Reality 已刪除，CF-WS Tunnel 回源定案） | 用戶連接的都是 CF anycast IP：源站永不暴露、永不因 IP 被封；代價是 **CF 被 QoS/限速時無直連備胎** → 客戶端配置改用 CF 償選 IP/域名緩解（SNI/Host 不變）；**訂閱即切換**：改 D1 節點記錄 → 訂閱版本 +1 → 用戶更新訂閱即恢復 ＋ Worker Cron 撥測告警（P2 後補） |

---

## 11. 節點面二次開發選型：3X-UI vs Marzban（評估結論 2026-09-06）

背景：成熟機場用開源面板（Marzban / 3X-UI）承擔節點面（xray 入站管理、用戶級流量強制、訂閱生成）。評估是否引入以替代自研 daemon + Worker 節點面。

### 11.1 對比

| 維度 | Marzban | 3X-UI |
| :--- | :--- | :--- |
| 技術棧 | Python (FastAPI) + React；SQLAlchemy（MySQL/SQLite） | Go 單二進制 + Vue |
| **多節點架構** | ✅ 原生：面板 + marzban-node agent，中央編排入站/用戶 | ❌ 單機工具：每台 VPS 一個獨立實例，無中央編排 |
| API | ✅ API-first，REST 完整（用戶 CRUD/流量/過期/訂閱），為被編排而生 | ⚠️ 有 API 但偏管理 UI 向，非為被另一控制面調用設計 |
| 訂閱端點 | ✅ 內置多格式（v2ray/clash/clash-meta/sing-box/outline…），模板可配 | ⚠️ 近年版本有，非重點 |
| 附帶 | Telegram bot 管理、webhook | Telegram bot |
| 許可 | **AGPL-3.0**（網絡服務型二次開發有開源義務） | GPL-3.0 |
| 上游健康 | ⚠️ Gozargah 商業化風波 → 社區分叉（Marzneshin 等），長期節奏有不确定性 | ✅ MHSanaei 持續高頻維護 |

### 11.2 結論

**二選一則 Marzban**——唯一有中央編排 + API-first + 內置多格式訂閱的，與本項目「中央控制面 + 邊緣節點」形狀同構；3X-UI 是單機管理 UI，與中央化架構相性差，僅適合純手動小規模。

**但引入前先想清楚兩種玩法**：

| 玩法 | 做法 | 代價/收益 |
| :--- | :--- | :--- |
| **A. 不引入（當前方案默認）** | 自研 daemon 已存在且是 Node，P2 改 `DAEMON_MANAGER_URL` 即用；Workers + D1 唯一大腦 | 零新工作量、單一事實源；節點面靠自研 |
| **B. 節點面外包給 Marzban** | 一台 VPS 跑 Marzban 面板（唯一節點大腦），節點跑 marzban-node（xray 監聽 127.0.0.1，Tunnel 回源不變）；Workers 瘦身為 portal BFF + 支付 + 會員，經 Marzban REST 讀寫節點/用戶數據 | 白拿多節點編排/TG bot/訂閱模板；代價精算見下 |

**代價精算（B 的四項成本逐項核實）**：

| 成本 | 必須？ | 說明 |
| :--- | :--- | :--- |
| D1 降級為計費庫 | ✅ 本架構下硬 | Marzban 用 SQLAlchemy（MySQL/SQLite 線協議），D1 只暴露 HTTP API，技術上接不上；除非魔改（不如不引入）。但「面板管節點、計費管錢」是行業常見分工（SSPanel 生態同構），屬架構取捨而非缺陷 |
| 雙庫同步 | ⚠️ 存在，可做薄 | 兩條單向管道：① D1 業務事件 → Marzban REST（建戶/續費/到期止付，實時 push）② Cron 定時拉流量彙總回 D1（小時級）。≈ 百行級同步模組，非雙向一致性問題 |
| AGPL 義務 | ❌ 可避免 | 傳染條件是「修改源碼且對公網提供服務」。不改 Marzban 內部、只從 Workers 調 REST API → 不受傳染；魔改內部才觸發 |
| 上游分叉風險 | ⚠️ 必在，可控 | 選活躍分叉（Marzneshin）或自行 vendor 鎖版本（AGPL 允許），轉成自己的維護預算 |

**建議**：A 走完 P0–P2 起步；若一開始就預期多節點擴張/想用面板生態，B 的真實門檻只 = 接受 D1 降級 + 一條薄同步管道，直接上 B 也成立。兩者都比自研面板省——差別只在事實源放哪。

### 11.3 玩法 A 的設計借鑑清單（抄設計不抄代碼，2026-09-06）

許可現實：本生態無寬鬆許可項目（Marzban 系 AGPL、3X-UI/V2bX GPL、edgetunnel GPL-2.0）——代碼一律不混入，只做設計級借鑑；模板文件（YAML/TOML 配置數據）參考結構不受限。

| 需求 | 借鑑來源 | 抄什麼 | 自研實現量 |
| :--- | :--- | :--- | :--- |
| 多節點編排 | Marzban 面板↔節點協議、V2bX agent 行為 | 配置版本號 push、心跳/在線狀態機、流量定時 pull、斷點續傳上報 | daemon 已有 80%，補節點註冊表+撥測 ≈ 200 行 |
| TG bot | Marzban bot 功能清單 | 查流量/續費鏈接/到期提醒/節點狀態/管理廣播；技術用 CF 官方 TG-webhook 模式 + grammY | ≈ 400 行，Cron 免費額度內 |
| 訂閱模板 | Marzban clash/sing-box 模板、subconverter 策略組 | 模板結構與策略組思路 → `subformats.ts` 模板常量 | 已在 P1 內，+0 |

同架構（Workers 控制面 + 真實節點 + 計費）無現成開源可二開；edgetunnel 系（Worker 內直接跑代理）為「無 VPS 純邊緣」路線，CF ToS 灰色 + 100k req/day 限制，僅個人兜底實驗價值，不進主方案。結論：**A + 設計借鑑 ≈ 600 行 TS 拿到 B 的功能清單，零許可/上游/D1 代價**。

---

## 12. 上線手冊（部署 / 測試 / 驗證）

### 12.0 前置憑證（一次性，安全紅線）

| 憑證 | 用途 | 存放位置（**永不進聊天/代碼/文檔**） |
| :--- | :--- | :--- |
| CF API Token | wrangler CLI / GitHub Actions 部署 | `wrangler login`（瀏覽器 OAuth，本地免 token）；CI 用 GitHub Secrets `CLOUDFLARE_API_TOKEN`，權限最小化：Workers Scripts:Edit + D1:Edit + KV:Edit + R2:Edit + Zone Routes:Edit |
| GitHub PAT | 僅 CI 用（本地推送走已有 git 憑證） | GitHub Settings 建 fine-grained（僅本 repo）→ repo Secrets |
| cloudflared 認證 | Tunnel 管理 | VPS 上 `cloudflared tunnel login`（一次性瀏覽器授權） |
| 業務 Secrets | JWT/BEPUSDT_*/PAYPAL_*/TURNSTILE_SECRET | `wrangler secret put <NAME>`（互動輸入，落 Workers Secrets） |

### 12.1 資源開通（依賴順序）

```bash
# 1. D1：建庫 → schema → 數據遷移（切換日才導數據）
wrangler d1 create rfplay                      # database_id 回填 wrangler.jsonc
wrangler d1 execute rfplay --remote --file=migrations/0001_schema.sql
# 切換日： sqlite3 manager.db ".dump" 整理後
wrangler d1 import rfplay --remote --file=seed.sql

# 2. KV（訂閱緩存/OAuth token 緩存）
wrangler kv namespace create CACHE             # id 回填 wrangler.jsonc

# 3. R2（備份歸檔；dashboard 先啟用 R2 一次）
wrangler r2 bucket create rfplay-backups

# 4. Worker 部署（灰度 routes 寫在 wrangler.jsonc：
#    api.rfplay.uk/api/v1/client/* → 先讀路徑）
wrangler deploy

# 5. Secrets（逐個互動輸入）
wrangler secret put JWT_SECRET
wrangler secret put BEPUSDT_API_URL   # https://pay.rfplay.uk
wrangler secret put BEPUSDT_TOKEN
wrangler secret put BEPUSDT_SECRET
wrangler secret put PAYPAL_CLIENT_ID
wrangler secret put PAYPAL_CLIENT_SECRET
wrangler secret put PAYPAL_WEBHOOK_ID   # PayPal developer dashboard 建 webhook 後取得
wrangler secret put TURNSTILE_SECRET
```

**Pages**（已 git 連動，零遷移）：portal/admin 加環境變量（`VITE_SUBSCRIPTION_BASE_URL`、Turnstile site key）→ 重新構建；CF Pages 自動管理邊緣證書。

**Tunnel**（VPS 上，收編 api 舊源站過渡期 / BEpusdt / 節點回源）：

```bash
cloudflared tunnel login
cloudflared tunnel create rfplay-vps
cloudflared tunnel route dns rfplay-vps pay.rfplay.uk
# config.yml ingress：
#   pay.rfplay.uk        → http://localhost:8080   (BEpusdt)
#   node-xx.rfplay.uk    → http://127.0.0.1:<xray-ws-port>
#   api.rfplay.uk（過渡期）→ http://localhost:8081  (舊 Go)
cloudflared service install                     # 開機自啟
```

**Access**：Zero Trust → Applications → Self-hosted → `admin.rfplay.uk` → 策略 Email OTP（≤50 席免費）。
**Turnstile**：dashboard 加站點拿 site key（env）+ secret（Worker secret）。

### 12.2 SSL/TLS 結論（無任何證書要買/續）

| 面 | 證書 |
| :--- | :--- |
| 邊緣（用戶↔CF） | Universal SSL 自動覆蓋 `*.rfplay.uk`，零操作 |
| api.rfplay.uk | Worker route 接管，**無源站無證書概念** |
| Tunnel 回源（cloudflared→localhost） | 明文 http 走本機 loopback，不出機器，無需證書 |
| Pages | CF 託管 |

→ **certbot / Let's Encrypt 整條鏈退役**；VPS 防火牆關閉全部公網入站（SSH 可留或改走 Access）。

### 12.3 測試與灰度（對應 P1→P4 順序）

1. **本地**：`wrangler dev`（模擬 D1/KV）+ `@cloudflare/vitest-pool-workers` 契約用例
2. **對拍**：過渡期同一請求打 Go 與 Worker，diff 響應（JSON 結構 / cookie 屬性 / `Subscription-Userinfo` / YAML 可解析）；腳本化，全綠才加下一條 route
3. **灰度順序**：`/api/v1/client/*`（讀，最低風險）→ `/api/v1/node/*`（daemon soak 24h）→ 停機窗口切 `/auth /web /user /admin /public`
4. **支付**：PayPal sandbox 全流程 → BEpusdt 小額真實單；重複回調冪等驗證
5. **客戶端驗收**：Clash Meta / Verge / Stash 導入 `/clash` 訂閱連真實節點；v2rayA 導入 base64 格式；訂閱詳情顯示流量/到期

### 12.4 上線驗證清單（每條可勾）

- [ ] `/health` 200（Worker）
- [ ] 訂閱三格式（base64/clash/singbox）與 Go 輸出對拍一致 + `Subscription-Userinfo` 頭存在
- [ ] 註冊/登入/CSRF/refresh 全鏈路，cookie 屬性（httpOnly/Secure/SameSite/MaxAge）與 Go 一致
- [ ] 舊 JWT（同 `JWT_SECRET`）在 Worker 驗證通過，會話不斷
- [ ] 支付：BEpusdt + PayPal 全流程 + 重複回調冪等 + 拒付止付生效
- [ ] daemon verify/sync 對 Worker 24h 無誤、流量數字與節點側一致
- [ ] D1 日寫入 <80% 額度；Workers 日請求 <80% 額度
- [ ] Tunnel 雙主機名穩定 72h；VPS 公網入站全關
- [ ] admin.rfplay.uk Access 門生效（無 token 訪問被擋）
- [ ] Turnstile 擋未通過校驗的註冊/登入
- [ ] 備份：`backup.sh` 已推 R2 且可恢復演練一次；切換前 `wrangler d1 export` 留底

### 12.5 回滾（過渡期內）

Worker route 在 dashboard **一鍵禁用** → 流量瞬時回落舊 Go 源站（同域名，無 DNS 變更、無用戶感知）。P4 刪除 Go 之後回滾=重新部署 git 上一版 Worker。

### 12.6 CI/CD

- **Workers**：GitHub Actions `cloudflare/wrangler-action`，push main → `wrangler deploy`（Secrets：`CLOUDFLARE_API_TOKEN` 最小權限）
- **Pages**：CF Pages Git 集成自動構建 portal/admin；PR 自動 preview
- **分支保護**：main 禁 force-push；部署只認 CI

### 12.7 必須人工提供的信息（最小清單，2026-09-06）

過濾原則：資源 ID / Tunnel / DNS / routes 均由 `wrangler`/`cloudflared` 命令自動產出，不需人工經手。GitHub PAT 不需要（Actions 內建 `GITHUB_TOKEN`，本地推送走現有 git 憑證）。

| # | 信息 | 在哪操作 | 填到哪 |
| :--- | :--- | :--- | :--- |
| 1 | CF API Token（最小權限） | dashboard → API Tokens → Create | GitHub Secrets `CLOUDFLARE_API_TOKEN` |
| 2 | Turnstile Site Key + Secret | dashboard → Turnstile 加站點 | site key → Pages env；secret → `wrangler secret put` |
| 3 | R2 S3 API 憑證（Key/Secret） | dashboard → R2 → Manage API Tokens | VPS `backup.sh`（rclone） |
| 4 | Access 應用 + 管理員郵箱白名單 | Zero Trust → Applications | dashboard 內 |
| 5 | cloudflared 授權（瀏覽器一次） | VPS `cloudflared tunnel login` | 本地憑證 |
| 6 | wrangler 授權（瀏覽器一次） | `wrangler login` | 本地憑證 |
| 7 | `JWT_SECRET` 現值 | 舊 Go `.env` 原樣複製 | `wrangler secret put` |
| 8 | `BEPUSDT_API_URL/TOKEN/SECRET` 現值 | 舊 VPS `.env` 原樣複製 | `wrangler secret put` |
| 9 | PayPal Client ID/Secret/Webhook ID | PayPal developer dashboard | `wrangler secret put` |

### 12.8 自動化清單（2026-09-06 決策：能自動化就自動化）

| # | 事項 | 自動化方式 | 檔 |
| :--- | :--- | :--- | :--- |
| 1 | Worker 部署 | GH Actions + wrangler-action，push main 即部署 | ✅ 已在 §12.6 |
| 2 | Pages（portal/admin） | CF Pages git 集成自動構建 | ✅ 已就緒 |
| 3 | D1 schema 遷移 | CI 內 `wrangler d1 execute --remote --file=migrations/*`，隨部署執行 | 🆕 併入 #1 的 workflow |
| 4 | 節點註冊 | admin API 建節點 + 產 token（`setup_nodes.py` 已有） | ✅ 已有 |
| 5 | 節點 VPS 供應 | `deploy-node-cf-ws.sh --manager-url --node-token`（裝 xray+daemon+systemd） | ✅ 已有 |
| 6 | **Tunnel 公共主機名 + DNS**（新增節點的第③步） | 腳本調 CF API：`PUT /cfd_tunnel/{id}/configurations` 加 ingress + `POST dns_records` 加 CNAME→`<tunnel-id>.cfargotunnel.com`；與 #4/#5 合併成 **`add-node.sh` 一條命令新增節點** | 🆕 方案內 |
| 7 | Secrets 供應 | `script: 讀 .env 逐個 wrangler secret put`（一次性執行，不進 git 的本地 .env） | 🆕 方案內 |
| 8 | 數據遷移 dump→seed | 腳本：`sqlite3 .dump` → 過濾 sqlite 內部表/觸發器整理成 D1 可導入 SQL | 🆕 方案內 |
| 9 | Go↔Worker 對拍 | 同請求雙打 diff 腳本，進 CI（過渡期每次部署跑） | 🆕 方案內 |
| 10 | R2 備份 + 輪轉 | backup.sh cron + `rclone delete >30d` | 🆕 併入 backup.sh |
| 11 | 節點撥測→自動摘除 | Worker Cron 撥測失敗 → 自動置 `status=inactive` → 訂閱不再下發（「訂閱即切換」的自動版）+ TG 告警 | 🆕 P2 後（TG bot 一部分） |
| 12 | 流量彙總寫 D1 | Workers Cron 批量 UPSERT（§8-P2 已設計） | 🆕 P2 |
| 13 | 額度監控告警 | CF GraphQL Analytics API 每日查 D1 寫入/Workers 請求，>80% TG 告警 | 🆕 P2 後 |
| 14 | 被牆自動換 IP | 撥測失敗→供應商 API 換 IP→更新 D1→訂閱自動生效 | ⛔ 先不做（節點全走 CF 邊緣，IP 不暴露，需求本身弱化） |
| 15 | 證書續期 | CF 全託管 | ✅ 天然自動 |
| 16 | Secrets 輪換 | 手動（低頻高危，不自動） | ⛔ 保持手動 |

> 落地順序：#6/#7/#8 在 P1 腳手架時順手寫；#9 過渡期必備；#11-13 跟 TG bot（§11.3）一起。

---

## 13. 開發計劃（執行清單，2026-09-06）

> 依賴順序：M1 → M2 → M3 → M4；M5 非阻塞可並行。每項有明確驗收，全綠才進下一項。

### M0 環境與憑證（人工，接近完成）
- [x] wrangler login / cloudflared cert.pem
- [x] D1 `rfplay` / KV `CACHE` / R2 `rfplay-backups` 建立並記錄 ID
- [ ] Turnstile Spin：拿 Site Key + Secret
- [ ] VPS：dashboard 建 Tunnel → token 裝 cloudflared → connector 綠 → 公共主機名（pay / node-xx / api 過渡回源），先 `tunnel-test` 驗證再切 A 記錄
- [ ] Access 套 `admin.rfplay.uk`；Email Routing 開啟

### M1 訂閱讀路徑上 Worker（P1，~2–3 天）
- [ ] `workers/api` 腳手架收尾：npm install、`wrangler dev` 本地 `/health` 200（半成品已落：wrangler.jsonc/index.ts/0001_schema.sql）
- [ ] schema 導入本地 D1 + `d1 import` 演練（dump→seed 腳本，自動化 #8）
- [ ] `lib/subformats.ts`：base64 / Clash YAML / sing-box 生成 + `Subscription-Userinfo` 頭（對拍 `subscription.go`）
- [ ] 端點：`/client/links/:token{,/clash,/singbox}`、`/client/config`（D1 讀 + KV 60s 緩存）
- [ ] 對拍腳本（自動化 #9）：Go vs Worker 同請求 diff
- [ ] **驗收**：三格式與 Go 逐字節一致；route `api.rfplay.uk/api/v1/client/*` 灰度上線，舊訂閱 URL 不死

### M2 節點面上 Worker（P2，~1–2 天）
- [ ] `/node/:token/config`（HMAC 對拍 `node_auth.go` 逐字節）
- [ ] `/node/:token/traffic/report`：KV 聚合 + Cron 批量 UPSERT（寫入合併，§8-P2 公式）
- [ ] daemon 改 `DAEMON_MANAGER_URL` → 24h soak
- [ ] `add-node.sh` 一條命令新增節點（自動化 #6：admin API + CF API ingress/DNS）
- [ ] **驗收**：節點配置與流量數字兩邊一致；D1 日寫入 <80% 額度

### M3 會話 + 管理面 + 支付（P3，~3–4 天）
- [ ] `lib/jwt.ts`（同 `JWT_SECRET` 驗舊 token）+ cookies.ts + csrf.ts
- [ ] `/public/register|login` + Turnstile siteverify（M0 的鑰匙接入）；刪圖形驗證碼
- [ ] `/auth/*`、`/admin/auth/*`、`/user/*`、`/web/*`、`/admin/*` 全量端點
- [ ] 支付：`md5.ts` + BEpusdt provider；`provider_paypal.ts`（sandbox 對拍）；D1 batch 激活事務（冪等 + 順延）
- [ ] portal 小改：subscriptionUrl 雙格式、Turnstile 組件、支付頁 provider=USDT/PayPal
- [ ] **驗收**：portal/admin 全功能過關（cookie 屬性/CSRF 頭不變）；支付沙箱全流程 + 重複回調冪等

### M4 切換與退役（P4，~1 天）
- [ ] 維護窗口：api 全量 route 切 Worker → `docker compose down manager nginx certbot`
- [ ] 觀察 72h → 刪 `manager/`（git 留檔）→ manager-data volume 最後備份推 R2
- [ ] CI/CD：GH Actions wrangler workflow + D1 migration 步驟（自動化 #1/#3）+ R2 備份 cron/輪轉（#10）
- [ ] **驗收**：§12.4 上線清單全勾

### M5 增強（非阻塞，M2 後任意時間）
- [ ] TG bot：查流量/續費鏈接/到期提醒/節點狀態/廣播（§11.3）
- [ ] 節點撥測 → 自動摘除 + TG 告警（#11）
- [ ] 額度監控 >80% 告警（#13）
