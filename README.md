# RFPlay Airport System

Proxy service platform for **rfplay.uk**.

| Service | URL | Deploy |
| :--- | :--- | :--- |
| User Portal (官网) | https://www.rfplay.uk | Cloudflare Pages |
| Admin Dashboard | https://admin.rfplay.uk | Cloudflare Pages |
| Manager API | https://api.rfplay.uk | **Cloudflare Workers**（TS，`workers/api`） |
| 客户端 | 用户自备通用 Clash（Meta/mihomo 系） | 无自研 App，订阅 URL 导入 |

## Key Decisions (Summary)

> Full spec: [airport_system_design.md §24](airport_system_design.md#24-additional-recommendations-suggested-not-yet-decided)

| Area | Decision |
| :--- | :--- |
| **官网/Admin 登录** | httpOnly cookie + CSRF；**禁止** localStorage JWT |
| **客户端** | 无自研客户端；portal 复制订阅 URL（`/clash`）→ 通用 Clash 导入 |
| **节点认证** | 连接时 `POST /api/node/verify-token`；sync **无** user_list |
| **支付** | BEpusdt(USDT) + PayPal；webhook → Worker |

Full spec: [airport_system_design.md](airport_system_design.md)

## Repository Layout (Monorepo)

```
airport-system/
├── workers/api/         # Manager API（TS/Hono on Workers）→ api.rfplay.uk
├── portal/              # Vue 3 官网 → CF Pages
├── admin/               # Vue 3 后台 → CF Pages
├── daemon/              # 节点代理（Go：拉配置 + 流量上報）→ 部署於節點 VPS
├── xray-core/           # Fork of XTLS/Xray-core（節點內核）
├── deploy/              # 部署/運維腳本（含 cloudflare/ 自動化）
├── admin.env.example / portal.env.example
└── cloudflare_migration_plan.md  # 遷移與上線文檔（§12 上線手冊）
```

### Cloudflare Pages

| CF Pages Project | Root | Domain | Build |
| :--- | :--- | :--- | :--- |
| `rfplay-portal` | `portal` | `www.rfplay.uk` | `npm ci && npm run build` |
| `rfplay-admin` | `admin` | `admin.rfplay.uk` | `npm ci && npm run build` |

Env (both): `VITE_API_BASE_URL=https://api.rfplay.uk`  
Portal/Admin: `axios.withCredentials = true` + CSRF header

> Portal only: subscription links are built from `VITE_API_BASE_URL` with the
> `/api/v1` prefix appended automatically, or from the optional
> `VITE_SUBSCRIPTION_BASE_URL` override (see `portal.env.example`).

## Docs

* **[PLAN.md](PLAN.md)** — 当前交付计划（Phase 0-5，MVP 优先） ← **必读**
* **[appstore_plan_a.md](appstore_plan_a.md)** — （已归档）App Store 上架方案 A；客户端方向已改为通用 Clash，见迁移方案
* **[cloudflare_migration_plan.md](cloudflare_migration_plan.md)** — 退役 Go，Manager 全量 TS 重写上 Workers（Workers/D1/KV/R2）
* [Architecture & API](airport_system_design.md) — 系统架构设计
* [Implementation plan](implementation_plan.md) — （归档）旧 Sprint 计划
* [Task checklist](task.md) — （归档）旧任务清单

## Environment Variables

Template files (copy to `.env` and fill in real values):

* `manager.env.example` — Manager API server
* `portal.env.example` — Portal (Vite)
* `admin.env.example` — Admin Dashboard (Vite)

See [Appendix B](airport_system_design.md#appendix-b-environment-variables) for the full variable reference.

## DNS (rfplay.uk)

| Record | Type | Target |
| :--- | :--- | :--- |
| `www` | CNAME | CF Pages (portal) |
| `admin` | CNAME | CF Pages (admin) |
| `api` | A / CNAME | Manager IP (proxied) |
| `node-*` | A | Node IPs (proxied, CF-WS) |
