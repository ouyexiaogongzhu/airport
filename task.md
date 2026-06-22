# Task List

> ⚠️ **已废弃。** 本文件为早期 Sprint 0-4 任务清单，所有 Checkbox 已不再维护。
> 请参考 **[PLAN.md](PLAN.md)** 获取 Phase 0-5 当前交付计划。

---

## Sprint 0 — 组长 @ubuntu_game_bot
- [x] 安装 Go 1.22+
- [x] 安装 Flutter + Android SDK
- [x] 创建 monorepo 目录骨架
- [x] 初始化 Go module (manager/)
- [x] 初始化 Vue 3 项目 (portal/ + admin/)
- [x] 初始化 Flutter 项目 (client/)
- [x] 配置 agent 工作权限
- [x] git add && commit && push

## Sprint 1 — Manager Core (后端 @ubuntu_game_combat_bot)
- [x] Phase 1: Set Up Directory & Clone Xray-core
  - [x] Clone `XTLS/Xray-core` and checkout latest stable tag
  - [x] Verify Go build
  - [x] Token layout: **Option B** (design §19)
- [x] Phase 2: Xray-core Modifications
  - [x] Inbound → Daemon `/internal/verify` (not local HMAC)
  - [x] Rate limiter, P2P audit, traffic stats, log push hooks
  - [x] Unit tests for verify callback path
- [x] Phase 3: Manager API
  - [x] SQLite schema (users, issued_tokens, nodes, traffic_logs, …)
  - [x] Go `:443` TLS (Origin PEM); CF IP allowlist
  - [x] CORS `AllowCredentials` + CSRF middleware
  - [x] Cookie auth: GET /api/auth/csrf, login/logout/refresh
  - [x] Dual login: browser cookies vs Flutter `X-Client: flutter` Bearer
  - [x] POST /api/auth/token-login (rf_ + at_), /validate, /token
  - [x] POST /api/node/verify-token; sync without user_list/hmac_secret
  - [x] issued_tokens: batch, immutable, /renew, /revoke; max_devices=0 unlimited
  - [x] /api/web/*, /api/client/*, /api/admin/*, payment callbacks
  - [x] Resend/Brevo register verify email
  - [x] traffic_logs aggregation for Admin charts
  - [x] GET /api/client/config; remove go:embed

## Sprint 2 — Frontend (并行 @ubuntu_game_ui_bot)
- [x] Portal CF Pages (`portal/`)
  - [x] Vue 3: register, login, plans, checkout, pay, account
  - [x] withCredentials + X-CSRF-Token; no localStorage JWT
  - [x] Client token copy/regenerate (rf_)
  - [x] Deploy → www.rfplay.uk
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
- [x] Daemon: verify-token client + 60s cache; sync traffic only
- [x] CF-WS: Nginx decoy static site + WS reverse proxy → Xray localhost
- [x] REALITY: dynamic port; same verify path
- [x] Loki push; CF IP firewall on origin
