# RFPlay Airport System

**rfplay.uk** 代理订阅平台：控制面全部运行在 Cloudflare，节点为自有 VPS（Xray + daemon），用户使用通用客户端导入订阅。

| 服务 | 地址 | 部署 |
| :--- | :--- | :--- |
| 官网 Portal | https://www.rfplay.uk | Cloudflare Pages（`portal/`） |
| 后台 Admin | https://admin.rfplay.uk | Cloudflare Pages（`admin/`） |
| Manager API | https://api.rfplay.uk | Cloudflare Workers（`workers/api/`，D1 + KV + R2） |
| 节点 | `node-*.rfplay.uk` | VPS：Xray-core + `daemon/` |
| 客户端 | — | 无自研 App，用户自备通用客户端，导入订阅 URL |

## 关键决策

| 领域 | 决策 |
| :--- | :--- |
| 登录会话 | HS256 JWT 放 httpOnly cookie + CSRF double-submit；Bearer 头仅作跨站兜底 |
| 客户端 | 无自研客户端；portal 复制订阅 URL → 通用客户端导入 |
| 节点 | daemon 定时拉取 Xray 配置（含有效用户 UUID 列表）并上报每用户流量，请求 HMAC 签名 |
| 支付 | BEpusdt（USDT）+ PayPal；回调打到 Worker，验签后激活/顺延订阅 |

## 代理协议与订阅格式

**入站经 Cloudflare 隐藏源站**：VLESS / VMess over WebSocket；用户连 Cloudflare 边缘 TLS（443），Tunnel 回源到节点本机 `127.0.0.1`。VPS 不对外开放代理端口，DNS 无源站 A 记录。Reality 已删除（见迁移方案 §1）。

> **出口 IP 不隐藏**：代理上网时目标站看到的是 VPS 公网 IP。隐藏的是「谁扫得到你的入站」，不是「你从哪出去」。

**v0.1.0（2026-09-24）** 已在 Android / Ubuntu / MacBook 验证：v2rayNG、v2rayA（Base64）、Clash Verge（`/clash`）。

订阅端点 `GET /api/v1/client/links/:token`：

| 路径 | 格式 | 适用客户端 |
| :--- | :--- | :--- |
| `/links/:token` | 多行分享链接整体 Base64（vless/vmess） | v2rayNG、v2rayA、Shadowrocket、OpenWrt |
| `/links/:token/clash` | Clash YAML | Clash Verge（mihomo）、Stash |
| `/links/:token/singbox` | sing-box JSON | **未完成**（占位） |

响应头 `Subscription-Userinfo` 携带已用流量 / 总流量 / 到期时间。进度见 [cloudflare_migration_plan.md](cloudflare_migration_plan.md)。

## 目录结构

```
airport/
├── workers/api/         # Manager API（TypeScript + Hono on Workers）→ api.rfplay.uk
│   ├── src/routes/      # public / auth / client（订阅）/ web（用户）/ payment / admin
│   ├── src/lib/         # jwt、cookie、csrf、支付、订阅格式、分享链接生成
│   └── migrations/      # D1 schema
├── portal/              # Vue 3 官网 → CF Pages
├── admin/               # Vue 3 后台 → CF Pages
├── daemon/              # 节点 daemon（Go：拉配置 + 流量上报）
├── deploy/
│   ├── cloudflare/      # push-secrets.sh（Worker Secrets）、dump-to-seed.sh（旧数据迁移）
│   ├── node-cf-ws/      # 节点部署脚本（Xray + daemon + cloudflared Tunnel）
│   └── docs/lessons.md  # 开发经验总结（含已退役的 Go/Flutter 时期内容）
└── .github/workflows/   # ci.yml（类型检查 + 测试 + 构建）、deploy-worker.yml
```

## 版本

| 版本 | 说明 |
| :--- | :--- |
| **v0.1.0** | 里程碑 A：Workers + Tunnel 节点；订阅在 v2rayNG / v2rayA / Clash Verge（Android / Ubuntu / MacBook）验证通过 |

后续传输：XHTTP + Cloudflare Tunnel 设计见 [docs/xhttp-cloudflare-design.md](docs/xhttp-cloudflare-design.md)（草案，未实现）。
设备槽位（订阅拉取侧，默认 5）：[docs/devices.md](docs/devices.md)。

## 部署

### Worker

```bash
cd workers/api
npm install
npx wrangler d1 execute rfplay --remote --file=migrations/0001_schema.sql
npx wrangler deploy
../../deploy/cloudflare/push-secrets.sh ../../.env   # 模板见根目录 .env.example
```

`main` 分支上 `workers/**` 有变更时，`deploy-worker.yml` 会自动应用 schema 并部署。

### Pages

| CF Pages 项目 | 根目录 | 域名 | 构建 |
| :--- | :--- | :--- | :--- |
| `rfplay-portal` | `portal` | `xv.rfplay.uk`（及 `www`） | `npm ci && npm run build` → Direct Upload |
| `rfplay-admin` | `admin` | `xva.rfplay.uk` | 同上 |

**部署方式**：不依赖 Pages 的 Git 集成。`main` 上 `portal/**` 或 `admin/**` 变更时，`.github/workflows/deploy-pages.yml` 构建静态资源并用 `wrangler pages deploy` 上传（与 Worker 同一套 `CLOUDFLARE_API_TOKEN`）。也可在 Actions 里手动 `workflow_dispatch`。

> **API Token 权限**：GitHub secret `CLOUDFLARE_API_TOKEN` 除 Workers 外，还必须包含 **Account → Cloudflare Pages → Edit**，否则 `deploy-pages` 会在 Deploy 步骤失败（构建仍可通过）。可在 [API Tokens](https://dash.cloudflare.com/?to=/:account/api-tokens) 编辑现有 token 或新建。

环境变量模板：`portal.env.example`、`admin.env.example`。CI 构建注入 `VITE_API_BASE_URL=https://api.rfplay.uk`（portal 另有 `VITE_SUBSCRIPTION_BASE_URL`）。

本地救急：

```bash
cd portal && npm ci && VITE_API_BASE_URL=https://api.rfplay.uk npm run build
npx wrangler pages deploy dist --project-name=rfplay-portal --branch=main
```

### 节点

后台建节点 → 生成节点 token（`nd_...`）→ 在 VPS 上执行 `deploy/node-cf-ws/deploy-node-cf-ws.sh` → 在 Cloudflare Tunnel 里为 `node-xx.rfplay.uk` 配置回源到 `http://127.0.0.1:<节点端口>`。daemon 配置示例见 `daemon/daemon.example.json`。

## DNS（rfplay.uk）

| 记录 | 类型 | 目标 |
| :--- | :--- | :--- |
| `www` | CNAME | CF Pages（portal） |
| `admin` | CNAME | CF Pages（admin） |
| `api` | Worker Custom Domain | `rfplay-api`（wrangler 自动创建） |
| `node-*` | CNAME（橙云） | `<tunnel-id>.cfargotunnel.com`（Tunnel 自动创建，不出现源站 IP） |

## 文档

* **[cloudflare_migration_plan.md](cloudflare_migration_plan.md)**：决策、架构、部署运维、已知 bug 与修复计划 ← **必读**
* [airport_system_design.md](airport_system_design.md)：早期完整设计（Go Manager + Flutter 时期），仅部分章节仍有效，见文首说明
