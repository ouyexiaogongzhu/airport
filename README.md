# RFPlay Airport System

Proxy service platform for **rfplay.uk**.

| Service | URL | Deploy |
| :--- | :--- | :--- |
| User Portal (官网) | https://www.rfplay.uk | Cloudflare Pages |
| Admin Dashboard | https://admin.rfplay.uk | Cloudflare Pages |
| Manager API | https://api.rfplay.uk | VPS `:443` + CF Proxy |
| Flutter Client | — | App stores（账号登录 / Token: `rf_` or `at_`） |

## Key Decisions (Summary)

| Area | Decision |
| :--- | :--- |
| **官网/Admin 登录** | httpOnly cookie + CSRF；**禁止** localStorage JWT |
| **Flutter 登录** | JWT Bearer + `X-Client: flutter` + secure storage |
| **Token 导入** | `rf_`（官网用户）/ `at_`（Admin 发放，免注册） |
| **`at_` 规则** | immutable；renew = 作废+发新；`max_devices=0` 不限设备 |
| **节点认证** | 连接时 `POST /api/node/verify-token`；sync **无** user_list |
| **CF-WS 节点** | Nginx 伪装站 + WS 反代；Origin PEM；CF IP 防火墙 |
| **REALITY 节点** | 直连；同 online verify |
| **支付** | BEpusdt + Payoneer；仅官网；webhook → `api.rfplay.uk` |
| **邮件** | Resend/Brevo 发信；CF Email Routing 收信转 Gmail |

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
├── airport_system_design.md
├── implementation_plan.md
├── task.md
└── README.md
```

### Cloudflare Pages

| CF Pages Project | Root | Domain | Build |
| :--- | :--- | :--- | :--- |
| `rfplay-portal` | `portal` | `www.rfplay.uk` | `npm ci && npm run build` |
| `rfplay-admin` | `admin` | `admin.rfplay.uk` | `npm ci && npm run build` |

Env (both): `VITE_API_BASE_URL=https://api.rfplay.uk`  
Portal/Admin: `axios.withCredentials = true` + CSRF header

## Docs

* [Architecture & API](airport_system_design.md)
* [Implementation plan](implementation_plan.md)
* [Task checklist](task.md)

## DNS (rfplay.uk)

| Record | Type | Target |
| :--- | :--- | :--- |
| `www` | CNAME | CF Pages (portal) |
| `admin` | CNAME | CF Pages (admin) |
| `api` | A / CNAME | Manager IP (proxied) |
| `node-*` | A | Node IPs (proxied, CF-WS) |
