# RFPlay Airport

**rfplay.uk** 代理订阅平台：控制面在 Cloudflare，节点为自有 VPS（官方 Xray + gateway），用户用 Clash / v2rayNG / v2rayA 导入订阅。

| 服务 | 域名 | 代码 |
| :--- | :--- | :--- |
| Portal | https://xv.rfplay.uk | `portal/` → CF Pages `rfplay-portal` |
| Admin | https://xva.rfplay.uk | `admin/` → CF Pages `rfplay-admin` |
| API | https://api.rfplay.uk | `workers/api/` → Worker `rfplay-api`（D1 + KV） |
| 节点 | `w1`/`w2`…（如 `w1.rfplay.uk`） | VPS：Xray + `gateway/` + cloudflared |

## 现状（要点）

| 项 | 说明 |
| :--- | :--- |
| 传输 | **VLESS + XHTTP + TLS**（经 CF Tunnel）；无 WS 产品路径；无 Reality |
| 订阅 | Base64 → v2rayNG/v2rayA；`/clash` → Clash Verge / mihomo（主推） |
| 登录 | 用户名或邮箱 + 密码；Turnstile **non-interactive**；可选 [Google OAuth](docs/oauth-google.md) |
| 会话 | HS256 JWT（httpOnly cookie）+ CSRF；密钥存 KV，约每日轮换，双钥宽限 |
| 节点代理 | `gateway/`（原 rfplay-daemon）HMAC 拉配置 / 报流量 |
| 支付 | **暂缓**；Admin grant 开通 |
| 版本 | **v0.1.1**（里程碑 A + Bug fix）；见 [cloudflare_migration_plan.md](cloudflare_migration_plan.md) §5 |
| 出口 IP | **不隐藏**（VPS 公网 IP） |

## 目录

```
airport/
├── workers/api/          # Hono API + D1 migrations
├── portal/               # 用户站（xv）
├── admin/                # 后台（xva）：用户/节点/一键复制订阅 URL
├── gateway/              # 节点 Go 代理
├── deploy/
│   ├── cloudflare/       # push-secrets.sh、dump-to-seed.sh
│   └── node-gateway/     # 部署与 UPGRADE.md
├── docs/                 # xhttp / devices / oauth-google
└── .github/workflows/    # ci、deploy-worker、deploy-pages
```

## 文档

| 文档 | 用途 |
| :--- | :--- |
| [airport_system_design.md](airport_system_design.md) | **架构设计（现行）** |
| [cloudflare_migration_plan.md](cloudflare_migration_plan.md) | 运维清单与 backlog |
| [docs/xhttp-cloudflare-design.md](docs/xhttp-cloudflare-design.md) | XHTTP + Tunnel |
| [docs/devices.md](docs/devices.md) | 设备槽（默认 5） |
| [docs/oauth-google.md](docs/oauth-google.md) | Google 登录配置 |

## 部署

**Worker**（`main` 上 `workers/**` 自动部署）：

```bash
cd workers/api && npm ci
npx wrangler d1 migrations apply rfplay --remote
npx wrangler deploy
../../deploy/cloudflare/push-secrets.sh ../../.env   # 见 .env.example
```

**Pages**（`portal/**` / `admin/**` 自动 Direct Upload）。Secrets：

| Secret | 权限 |
|--------|------|
| `CLOUDFLARE_API_TOKEN` | Workers Scripts Edit + D1 Edit |
| `CLOUDFLARE_PAGES_API_TOKEN` | 可选；Pages Edit（未设则回退上一列） |
| `VITE_TURNSTILE_SITE_KEY` | Pages 构建注入 |
| `VITE_GOOGLE_CLIENT_ID` | 可选；显示 Google 按钮 |

**节点**：Admin 建节点 → token → `deploy/node-gateway/deploy-node-gateway.sh` → Tunnel 主机名 → `127.0.0.1:<port>`。旧 daemon 升级见 `deploy/node-gateway/UPGRADE.md`。

## 订阅 URL

```
https://api.rfplay.uk/api/v1/client/links/<client_token>        # Base64
https://api.rfplay.uk/api/v1/client/links/<client_token>/clash  # Clash
```
