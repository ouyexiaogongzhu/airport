# RFPlay Airport System

Proxy service platform for **rfplay.uk**.

| Service | URL | Deploy |
| :--- | :--- | :--- |
| User Portal (官网) | https://www.rfplay.uk | Cloudflare Pages |
| Admin Dashboard | https://admin.rfplay.uk | Cloudflare Pages |
| Manager API | https://api.rfplay.uk | VPS `:443` + CF Proxy |
| Flutter Client | — | App stores（账号登录 / Token: `rf_` or `at_`） |

## Key Decisions (Summary)

> Full spec: [airport_system_design.md §24](airport_system_design.md#24-additional-recommendations-suggested-not-yet-decided)

| Area | Decision |
| :--- | :--- |
| **官网/Admin 登录** | httpOnly cookie + CSRF；**禁止** localStorage JWT |
| **Flutter 登录** | JWT Bearer + `X-Client: flutter` + secure storage |
| **节点认证** | 连接时 `POST /api/node/verify-token`；sync **无** user_list |
| **支付** | BEpusdt + Payoneer；仅官网；webhook → `api.rfplay.uk` |

Full spec: [airport_system_design.md](airport_system_design.md)

## Repository Layout (Monorepo)

```
airport-system/
├── manager/             # Go Fiber API → api.rfplay.uk
├── portal/              # Vue 3 官网 → CF Pages
├── admin/               # Vue 3 后台 → CF Pages
├── daemon/              # Node agent（verify + sync + Loki）
├── client/              # Flutter VPN app
├── xray-core/           # Fork of XTLS/Xray-core
├── shared/              # Portal & Admin 共用代码 (types, api, utils)
├── deploy/              # 部署脚本
├── .env.example files   # 各服务环境变量模板
└── docs/                # airport_system_design.md, PLAN.md, task.md
```

### Cloudflare Pages

| CF Pages Project | Root | Domain | Build |
| :--- | :--- | :--- | :--- |
| `rfplay-portal` | `portal` | `www.rfplay.uk` | `npm ci && npm run build` |
| `rfplay-admin` | `admin` | `admin.rfplay.uk` | `npm ci && npm run build` |

Env (both): `VITE_API_BASE_URL=https://api.rfplay.uk`  
Portal/Admin: `axios.withCredentials = true` + CSRF header

## Docs

* **[PLAN.md](PLAN.md)** — 当前交付计划（Phase 0-5，MVP 优先） ← **必读**
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
