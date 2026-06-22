# Task List: Airport Proxy System Implementation

## Sprint 0 — 组长 @ubuntu_game_bot
- [ ] 安装 Go 1.22+
- [ ] 安装 Flutter + Android SDK
- [ ] 创建 monorepo 目录骨架
- [ ] 初始化 Go module (manager/)
- [ ] 初始化 Vue 3 项目 (portal/ + admin/)
- [ ] 初始化 Flutter 项目 (client/)
- [ ] 配置 agent 工作权限
- [ ] git add && commit && push

## Sprint 1 — Manager Core (后端 @ubuntu_game_combat_bot)
- [ ] Phase 1: Set Up Directory & Clone Xray-core
  - [ ] Clone `XTLS/Xray-core` and checkout latest stable tag
  - [ ] Verify Go build
  - [x] Token layout: **Option B** (design §19)
- [ ] Phase 2: Xray-core Modifications
  - [ ] Inbound → Daemon `/internal/verify` (not local HMAC)
  - [ ] Rate limiter, P2P audit, traffic stats, log push hooks
  - [ ] Unit tests for verify callback path
- [ ] Phase 3: Manager API
  - [ ] SQLite schema (users, issued_tokens, nodes, traffic_logs, …)
  - [ ] Go `:443` TLS (Origin PEM); CF IP allowlist
  - [ ] CORS `AllowCredentials` + CSRF middleware
  - [ ] Cookie auth: GET /api/auth/csrf, login/logout/refresh
  - [ ] Dual login: browser cookies vs Flutter `X-Client: flutter` Bearer
  - [ ] POST /api/auth/token-login (rf_ + at_), /validate, /token
  - [ ] POST /api/node/verify-token; sync without user_list/hmac_secret
  - [ ] issued_tokens: batch, immutable, /renew, /revoke; max_devices=0 unlimited
  - [ ] /api/web/*, /api/client/*, /api/admin/*, payment callbacks
  - [ ] Resend/Brevo register verify email
  - [ ] traffic_logs aggregation for Admin charts
  - [ ] GET /api/client/config; remove go:embed

## Sprint 2 — Frontend (并行 @ubuntu_game_ui_bot)
- [ ] Portal CF Pages (`portal/`)
  - [ ] Vue 3: register, login, plans, checkout, pay, account
  - [ ] withCredentials + X-CSRF-Token; no localStorage JWT
  - [ ] Client token copy/regenerate (rf_)
  - [ ] Deploy → www.rfplay.uk
- [ ] Admin CF Pages (`admin/`)
  - [ ] Vue 3: login (admin_session cookie + CSRF), nodes, users, plans, orders
  - [ ] Issued tokens: batch, traffic_mode, max_devices, renew
  - [ ] Deploy → admin.rfplay.uk

## Sprint 3 — Flutter (@ubuntu_game_char_bot)
- [ ] Flutter Client
  - [ ] api_base_url from /api/client/config
  - [ ] Login: 账号登录 | Token 导入 (rf_ / at_); `X-Client: flutter`
  - [ ] Secure storage; auto token-login on relaunch
  - [ ] token_only: no 续费; renew 后提示重新导入 at_
  - [ ] 续费 → www.rfplay.uk/plans; expiry reminders; devices; VPN

## Sprint 4 — Node Daemon + Deploy (后端 @ubuntu_game_combat_bot)
- [ ] Daemon: verify-token client + 60s cache; sync traffic only
- [ ] CF-WS: Nginx decoy static site + WS reverse proxy → Xray localhost
- [ ] REALITY: dynamic port; same verify path
- [ ] Loki push; CF IP firewall on origin

## Sprint 5 — Integration (@ubuntu_game_bot 总负责)
- [ ] 官网购套餐 → webhook → Flutter 连接 E2E
- [ ] 多设备连不同节点
- [ ] CORS + cookie/CSRF + Flutter Bearer 全链路

## Deferred
- [ ] Origin CA auto-renew, RBAC, PostgreSQL, 24h grace, Loki cold storage
