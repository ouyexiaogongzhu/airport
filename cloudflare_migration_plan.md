# RFPlay 运维与 backlog

> **状态（2026-09-25）**：v0.1.1 — 里程碑 A 已验收；**Bug fix** 已合入生产；同站 `/api` proxy 与 Free WAF 登录规则已生效；支付（里程碑 B）暂缓。  
> **架构**：[airport_system_design.md](airport_system_design.md)（现行）。入口：[README.md](README.md)。

---

## 1. 已确定决策

| 项 | 决策 |
| :--- | :--- |
| 后端 | 单 Worker `rfplay-api`（Hono），共享 D1；不拆微服务 |
| 客户端 | 通用客户端导入订阅：Clash `/clash` 主推；Base64 `/links/:token` |
| 节点 | CF Tunnel + VLESS/VMess + **XHTTP** + TLS；Xray 只听 `127.0.0.1` |
| 协议 | 仅 `vless` / `vmess`；SS / Trojan / WS 产品路径已下线 |
| 支付 | 暂缓；Admin `grant` 开通 |
| 出口 IP | 不隐藏（VPS `freedom`） |
| 会话 | JWT cookie + CSRF；密钥 KV 日轮换（`jwtkeys.ts`）；`JWT_SECRET` bootstrap |
| 人机 | Turnstile non-interactive；**不**对 `api`/`xva` 挂 Access（CORS） |
| 登录 | 用户名或邮箱 + 密码；可选 Google OAuth |

---

## 2. 架构速览

```
  Pages   xv.rfplay.uk（portal） / xva.rfplay.uk（admin）
          浏览器打同站 /api/v1；Function 经 binding API → rfplay-api
          （两项目 wrangler.toml [[services]]，生产 deployment_configs 已绑定）
  Worker  api.rfplay.uk  → D1 + KV（订阅缓存 + JWT 密钥）+ Cron
  订阅 / Google OAuth start·callback 仍绝对 https://api.rfplay.uk
  Tunnel  w1/w2…rfplay.uk → VPS 127.0.0.1:<port>（Xray + gateway + cloudflared）
```

Secrets：`deploy/cloudflare/push-secrets.sh` + `.env.example`。  
Vars：`MOCK_PAY_ENABLED="0"`、`PORTAL_URL`、`COOKIE_DOMAIN`。

---

## 3. 部署与运维

### 3.1 CI/CD

- **Worker**：`main` → `deploy-worker.yml`（migrations + deploy）
- **Pages**：`deploy-pages.yml` Direct Upload；`VITE_API_BASE_URL=/api/v1`（订阅基址仍 `https://api.rfplay.uk`）；`rfplay-portal` 与 `rfplay-admin` 均有 Service Binding `API` → `rfplay-api`（生产已绑定）；注入 `VITE_TURNSTILE_SITE_KEY` / 可选 `VITE_GOOGLE_CLIENT_ID`
- **本地**：`npm run db:migrate`、`wrangler dev`、`npx vitest run`
- **回滚**：重部署上一 Worker；D1 Time Travel ~30 天

### 3.2 新增 / 升级节点

1. Admin 建节点 → 发 `nd_` token  
2. `deploy/node-gateway/deploy-node-gateway.sh`（或 `UPGRADE.md`：daemon → gateway）  
3. Tunnel 公共主机名 → `http://127.0.0.1:<port>`

### 3.3 人工事项

| # | 事项 | 说明 |
| :--- | :--- | :--- |
| 1 | **里程碑 B** | 支付商户 / webhook（暂缓） |
| 2 | **Google OAuth** | 见 [docs/oauth-google.md](docs/oauth-google.md) |

**已配**：Access 已从 api/xva 移除；Turnstile 已启用（非 interactive）；生产节点 `w1`/`w2` 为 XHTTP + rfplay-gateway。

### 3.4 免费额度

| 资源 | 注意 |
| :--- | :--- |
| Workers | 免费请求额度共享：Pages Function 每次调用，与绑定的 Worker 调用，都可能各计一次；静态 HTML/JS 不计。订阅拉取为主；超限升 Paid |
| D1 写 | 流量上报是大户；控制上报间隔 |
| KV 写 | 订阅缓存勿按请求写；JWT 轮换日 ≤2 写 |
| WAF Free | **1** 条 Rate Limiting；**5** 条 Custom rules — 见 [docs/waf-free-api-protect.md](docs/waf-free-api-protect.md) |

### 3.5 Free WAF：登录保护（已生效）

区上规则已生效（非 Terraform；重建仍走 Dashboard / API）。Bot Fight Mode Off；**唯一**一条 RL：login/register/admin-login 的 POST，5 次 / 10s 每 IP，characteristics `cf.colo.id` + `ip.src`，Block，hosts `api`+`xv`+`xva`；1 条 custom：仅这些登录路径空 UA → Block。**不要** RL 全部 `/api`；Turnstile 仍开。同站 proxy 后浏览器打的是 xv/xva，规则必须含这两 host。

重建或改规则：[docs/waf-free-api-protect.md](docs/waf-free-api-protect.md)。

---

## 4. Backlog（未完成）

| 优先级 | 项 | 说明 |
| :--- | :--- | :--- |
| B | 支付闭环 | BEpusdt / PayPal：验签、金额核对、库存回补、退款收权（见历史审查 #14–#22） |
| C | sing-box 订阅 | 现仅占位；portal 入口已下线 |
| C | D1 → R2 备份 | 现靠 Time Travel |
| — | gateway | `sync_interval` 须纳秒整数；用户变更仍整进程重启 Xray |
| — | 限速 | `rate_limit_bps` 不下发（Xray 无按用户限速） |
| — | 运维增强 | 拨测自动 inactive、额度告警、Telegram bot |
| — | Free WAF | **已生效**（2026-09-25）：登录 RL 5/10s + 登录路径空 UA（hosts api+xv+xva，BFM Off）。重建见 [docs/waf-free-api-protect.md](docs/waf-free-api-protect.md)（Free：1 RL / 5 custom） |

**已完成（摘要）**：节点 HMAC 配置/流量、资格判定 + Cron、Tunnel XHTTP、会话 `token_version` + refresh、设备槽、JWT KV 轮换、Google OAuth 代码路径、Admin 复制订阅 URL、portal Dashboard 合并套餐/设备、流量明细 14 天保留 + traffic_daily 每日汇总（0007）、上报幂等 batch UUID 去重（0008，gateway 未确认批次原样重发）、refresh 7 天用时换发 + 存量长寿 token 拒绝（部署切换点：全体重登一次）、Admin `at_` 自建 token 免注册发放/吊销/续期（0009，合成 token_only 用户）、同站 `/api` proxy（Pages binding `API`）与 Free WAF 登录规则。

---

## 5. 里程碑

| 里程碑 | 内容 | 状态 |
| :--- | :--- | :--- |
| **A** | 无支付跑通：建节点、开通、订阅、上网、记账、停服 | ✅ v0.1.0 |
| **Bug fix** | 生产加固与缺陷收敛（见下） | ✅ v0.1.1 |
| **B** | 支付 | 暂缓 |
| **C** | sing-box / 备份 / 体验补全 | 按需 |

### 5.1 里程碑 Bug fix（✅ 2026-09-24 / v0.1.1）

面向线上稳定与可排障，不引入支付：

| 面 | 已做 |
| :--- | :--- |
| 流量 | `traffic_records` 14 天保留 + `traffic_daily` 日汇总（0007）；上报 `batch_id` 幂等去重（0008）；gateway 未确认批次落盘重发 |
| 会话 | refresh 7 天用时换发；拒绝存量超长寿 token；JWT KV 轮换；logout CSRF |
| 安全 | OAuth 不按 email 自动绑账号；jwtkeys 降级不覆写；CORS；登录时序枚举；Turnstile 重试 |
| Admin | `at_` 自建 token 发放/吊销/续期（0009）；UpdateNode 校验 |
| gateway | 原子配置（`.tmp.json`，Xray 可识别格式）；孤儿 Xray 探测；优雅退出；健康同步降噪 |
| 可观测 | Xray `loglevel=error` + `/var/log/xray/error.log`（关 access）；gateway journald |
| 节点 | NY / Amsterdam 均已升到现行 `rfplay-gateway` |
