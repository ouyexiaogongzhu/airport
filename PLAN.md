# RFPlay Airport System — 开发规划

> 开发组长视角 | 参考: `airport_system_design.md` (1953行), `implementation_plan.md`, `task.md`
> 
> 设计文档已定义全部 API 合约、DB Schema、Token 格式。本规划**不重复**设计细节，
> 仅做任务分解、并行策略、团队分工。实现时直接引用 `airport_system_design.md` §章节号。

---

## 1. 团队分工

| 角色 | Agent (Telegram) | Pipeline | 核心产出 |
|:---|:---|:---|:---|
| **开发组长** | @ubuntu_game_bot | 全局 | 规划、Git 管理、环境搭建、任务调度、集成测试、部署 |
| **后端** | @ubuntu_game_combat_bot | A | Manager API (Go+Fiber), Daemon, Xray-core fork, CF-WS 部署 |
| **前端** | @ubuntu_game_ui_bot | B | Portal Vue 3 → www.rfplay.uk, Admin Vue 3 → admin.rfplay.uk |
| **移动端** | @ubuntu_game_char_bot | C | Flutter Android Client → App Store |
| **QA** | 前端/后端交叉验证 | — | API 测试、联调验证、上线 checklist |

---

## 2. Monorepo 目录骨架

```
airport-system/                        # GitHub: vincent/airport-system (私有)
├── PLAN.md                            # ← 本文件
├── README.md                          # 已有
├── airport_system_design.md           # 已有 — 核心设计文档
├── implementation_plan.md             # 已有
├── task.md                            # 已有
├── .gitignore                         # 已有
├── .github/
│   └── workflows/                     # CI (可选)
│       ├── portal.yml                 #   Portal 自动部署到 CF Pages
│       └── admin.yml                  #   Admin 自动部署到 CF Pages
│
├── manager/                           # Go Fiber API → api.rfplay.uk
│   ├── go.mod
│   ├── main.go
│   ├── cmd/
│   │   └── init-admin/               # --init-admin CLI (§4.6)
│   ├── internal/
│   │   ├── config/                    # 配置加载
│   │   ├── db/                        # SQLite schema + migrations
│   │   ├── middleware/                 # CORS, CSRF, Auth (cookie vs Bearer)
│   │   ├── handler/                    # HTTP handlers
│   │   │   ├── auth.go                # §4.2 — login/register/validate/token-login
│   │   │   ├── web.go                 # §4.3 — Portal API (plans/orders/account)
│   │   │   ├── admin.go               # §4.6 — Admin API (users/tokens/plans/nodes)
│   │   │   ├── client.go              # §4.4 — Flutter API (config/subscription)
│   │   │   ├── node.go                # §4.1 — Daemon sync + verify-token
│   │   │   └── payment.go             # §4.5 — Payment callbacks
│   │   ├── model/                     # DB models + queries
│   │   ├── token/                     # §19 — 68-byte dynamic token
│   │   ├── payment/                   # §4.7 — BEpusdt + Payoneer providers
│   │   └── cron/                      # §7 — auto-disable expired users
│   └── data/                          # SQLite DB (gitignored)
│
├── portal/                            # Vue 3 → www.rfplay.uk (CF Pages)
│   ├── package.json
│   ├── vite.config.ts
│   ├── public/_redirects              # SPA fallback
│   └── src/
│       ├── main.ts
│       ├── App.vue
│       ├── api/                       # axios instance (withCredentials + CSRF)
│       ├── views/
│       │   ├── Home.vue               # /
│       │   ├── Login.vue              # /login
│       │   ├── Register.vue           # /register
│       │   ├── Plans.vue              # /plans
│       │   ├── Checkout.vue           # /checkout/:plan_id
│       │   ├── Pay.vue                # /pay/:order_id (poll)
│       │   ├── PayResult.vue          # /pay/result
│       │   └── Account.vue            # /account + /account/devices
│       └── components/
│
├── admin/                             # Vue 3 → admin.rfplay.uk (CF Pages)
│   ├── package.json                   # 独立构建
│   ├── vite.config.ts
│   ├── public/_redirects
│   └── src/
│       ├── main.ts
│       ├── App.vue
│       ├── api/                       # axios (admin_session + CSRF)
│       └── views/
│           ├── Login.vue
│           ├── Dashboard.vue
│           ├── Nodes.vue              # Node CRUD + CF DNS
│           ├── Users.vue              # 用户管理
│           ├── Tokens.vue             # at_ 发放/批次/吊销/续期
│           ├── Plans.vue              # 套餐 CRUD
│           ├── Orders.vue             # 订单列表
│           └── Settings.vue           # CF API Token, HMAC, etc.
│
├── daemon/                            # Node agent (Go)
│   ├── go.mod
│   ├── main.go                        # 60s sync loop (§9)
│   └── internal/
│       ├── verify/                    # verify-token client + 60s cache
│       ├── sync/                      # traffic report + config pull
│       └── loki/                      # log push (§15)
│
├── client/                            # Flutter
│   ├── pubspec.yaml
│   └── lib/
│       ├── main.dart
│       ├── models/
│       ├── services/
│       │   ├── api_service.dart       # X-Client: flutter + Bearer
│       │   ├── auth_service.dart      # §3.2 — login / token-login / secure storage
│       │   ├── subscription_service.dart
│       │   └── vpn_service.dart       # §3.5 — Xray config gen + VPN
│       ├── screens/
│       │   ├── login/
│       │   ├── home/
│       │   ├── account/
│       │   └── devices/
│       └── widgets/
│
└── deploy/                            # 部署脚本
    ├── manager/                       # Manager systemd service + CF IP firewall
    ├── node-cf-ws/                    # Nginx 伪装站 + WS 反代 (§14.5)
    └── node-reality/                  # REALITY 配置模板
```

---

## 3. 并行流水线

### Pipeline A：后端（@ubuntu_game_combat_bot）— 全周期独立

```
Sprint 1 ────────────────────────────────────── Sprint 2-3 ────────────────── Sprint 4
┌─────────────────────────┐   ┌──────────────────────────────────────┐   ┌──────────┐
│ S1.0  Git init + 骨架    │   │ S2.1 Xray inbound verify 回调        │   │ 联调 +   │
│ S1.1  Xray-core clone    │   │ S2.2 Xray rate limiter + 流量统计   │   │ 部署上线  │
│ S1.2  Manager 骨架+TLS   │   │ S2.3 Xray P2P audit + Loki push     │   │          │
│ S1.3  Manager 认证体系    │   │ S2.6 Manager 剩余 API + 支付回调     │   │          │
│ S1.4  Manager 节点 API   │   │ S2.7 Daemon 开发                     │   │          │
│       (verify+sync)      │   │ S3.5 CF-WS 节点部署脚本              │   │          │
└─────────────────────────┘   └──────────────────────────────────────┘   └──────────┘
```

- **Sprint 1**：Manager Core（认证 + 节点 API）→ 前端可以对接实时 API
- **Sprint 2**：Xray-core 改造（这是最复杂的模块）+ 剩余 Manager API + Daemon
- Xray-core fork 与 Manager API 可并行开发（不同目录、独立 Go module）

### Pipeline B：前端（@ubuntu_game_ui_bot）— 与后端并行

```
Sprint 1 ─────────────────────────────── Sprint 2 ─────────────────── Sprint 4
┌──────────────────────────┐   ┌──────────────────────────────┐   ┌──────────┐
│ S1.5 Portal Vue 骨架     │   │ S2.4 Portal 全功能           │   │ CF Pages │
│  (login/register/plans)  │   │  (checkout/pay/account/dev)  │   │ 部署      │
│  对接实时 Manager API     │   │  对接实时 API               │   │          │
│                          │   │                              │   │          │
│ S1.6 Admin Vue 骨架      │   │ S2.5 Admin 全功能            │   │          │
│  (login + users 列表)    │   │  (nodes/users/tokens/orders) │   │          │
└──────────────────────────┘   └──────────────────────────────┘   └──────────┘
```

- Sprint 1 用 **实时 Manager API** 对接（Pipeline A 同时产出）
- 前端不堵后端 — S1.3 (auth) 完成后 Portal/Admin login 就可接上
- SPA 路由、_redirects、withCredentials + CSRF 拦截器一次配好

### Pipeline C：移动端（@ubuntu_game_char_bot）— Mock 先行

```
Sprint 1 ─────────────────────────────── Sprint 3 ─────────────────── Sprint 4
┌──────────────────────────┐   ┌──────────────────────────────┐   ┌──────────┐
│ S3.0 Flutter SDK + 骨架   │   │ S3.1 Flutter 登录模块        │   │ 联调     │
│  (API service, routing)  │   │  (账号/Token/ZJ导入)          │   │          │
│  本地 mock 起手           │   │  对接实时 API                │   │          │
│                          │   │                              │   │          │
│                          │   │ S3.2 主界面+节点列表+续费     │   │          │
│                          │   │ S3.3 设备管理                │   │          │
│                          │   │ S3.4 VPN 集成               │   │          │
└──────────────────────────┘   └──────────────────────────────┘   └──────────┘
```

- Sprint 1 用本地 mock data（硬编码 JSON）搭 Flutter 全部 UI
- Sprint 3 Manager API 稳定后切到实时 API
- **不阻塞其它 Pipeline**

### Pipeline D：组长集成测试（@ubuntu_game_bot）

```
Sprint 1 ────────────── Sprint 2 ────────────── Sprint 3 ──────────── Sprint 4
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌────────────────┐
│ 环境搭建      │    │ API 回归测试  │    │ 全链路联调   │    │ 安全审计       │
│ 目录骨架      │    │ Frontend     │    │ Flutter 测试  │    │ 生产部署       │
│ 工具链安装    │    │ 功能验证     │    │ 节点部署验证   │    │ 上线 checklist │
└──────────────┘    └──────────────┘    └──────────────┘    └────────────────┘
```

---

## 4. Sprint 排期

### Sprint 1：基础设施 + 核心后端（A+B+D 并行）

**目标**：Manager 认证体系跑通，前端骨架可登录，Flutter 骨架搭建

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
| 8.0 | 本机开发环境搭建（Go/Flutter/Android SDK） | @ubuntu_game_bot | — | §22 | S |
| 8.1 | Monorepo 目录骨架创建 + 初始化 | @ubuntu_game_bot | 8.0 | §22 | S |
| 8.2 | Xray-core 克隆 + 验证 Go build | @ubuntu_game_combat_bot | 8.1 | task.md Phase 1 | S |
| 8.3 | Manager Go 项目 init + SQLite schema + TLS :443 + CF IP middleware | @ubuntu_game_combat_bot | 8.1 | §3 DB schema, §14 | M |
| 8.4 | Manager Cookie + CSRF 中间件 (portal + admin 双套) | @ubuntu_game_combat_bot | 8.3 | §4.2.1, §10.12 | M |
| 8.5 | Manager 双模式 login (cookie + Bearer) + logout/refresh/validate | @ubuntu_game_combat_bot | 8.4 | §4.2.1, §4.2.2 | M |
| 8.6 | Manager verify-token + sync API（无 user_list） | @ubuntu_game_combat_bot | 8.3 | §4.1, §4.1.1 | M |
| 8.7 | Portal Vue 骨架 + axios WithCredentials + login/register 页 + Plans | @ubuntu_game_ui_bot | 8.5 或 mock | §10.4, §2.2 | M |
| 8.8 | Admin Vue 骨架 + admin_session login + Users 列表页 | @ubuntu_game_ui_bot | 8.5 或 mock | §10.5, §10.6 | M |
| 8.9 | Flutter 骨架 + API Service + routing + mock data 模式 | @ubuntu_game_char_bot | 8.0 | §3 | M |
| 1.10 | ~~QA 测试策略文档 + 测试环境搭建 + API 测试脚本 (curl)~~ | — | — | 无需独立 QA agent；整合到集成测试 |

**并行分析**：

```
|Week 1                Week 2                Week 3
├─┤8.0 ubuntu_game_bot init─────┤
  ├─┤8.2 Xray clone──┤
  ├─┤8.3 Manager─────┤
  │                    ├─┤8.4 CSRF──────┤
  │                    │                 ├─┤8.5 Auth───────┤
  │                    └─┤8.6 Node API──┤
  ├─┤8.9 Flutter mock─────────────┤
  ├─┤8.7 Portal───────────────────┤                ← mock 先行，等 8.5 后切 real
  ├─┤8.8 Admin────────────────────┤                ← 同上
```

**实际并发**：
- Backend (combat_bot) 可同时跑：8.3（Manager 骨架）+ 8.2（Xray clone）— 不同目录
- Frontend (ui_bot) 8.7 + 8.8 需要等 8.5（auth API）或先用 mock → 建议 mock 先行，等 8.5 完成后再切 real API
- Mobile (char_bot) 8.9 完全独立（mock data）
- @ubuntu_game_bot 8.0 完成后可并行做其它事（集成测试准备）

### Sprint 2：完整功能 + Xray 改造（A+B+C+D 四线全开）

**目标**：Portal/Admin 全功能就绪，Xray verify 回调跑通，Daemon MVP

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
|| 2.1 | Manager 支付流程：plans/orders/payment callback + BEpusdt + Payoneer | @ubuntu_game_combat_bot | 8.5 | §4.3, §4.5, §4.7 | L |
|| 2.2 | Manager rf\\_/at\\_ token-login + issued\\_tokens immutable CRUD | @ubuntu_game_combat_bot | 8.5 | §3.1.1, §3.2 | M |
|| 2.3 | Manager Resend/Brevo 注册验证邮件 | @ubuntu_game_combat_bot | 8.5 | §14A | S |
|| 2.4 | Manager client config + subscription + traffic aggregation | @ubuntu_game_combat_bot | 8.5 | §4.4, §6.3 | M |
|| 2.5 | Manager 自动到期 cron + Admin API（users/nodes/plans CRUD） | @ubuntu_game_combat_bot | 8.5 | §7, §4.6 | M |
|| 2.6 | Xray-core：inbound verify 回调 → Daemon /internal/verify | @ubuntu_game_combat_bot | 8.2 | §4.1.1, Phase 2 | L |
|| 2.7 | Xray-core：rate limiter (Go token bucket per session) | @ubuntu_game_combat_bot | 2.6 | §5 | M |
|| 2.8 | Xray-core：P2P audit + traffic stats + Loki push hooks | @ubuntu_game_combat_bot | 2.6 | §13, §15 | M |
|| 2.9 | Portal 全功能：checkout, pay poll, account, token copy/regenerate, devices | @ubuntu_game_ui_bot | 8.7, 2.1-2.2 | §2.2, §3.2 | L |
|| 2.10 | Admin 全功能：nodes CRUD + CF DNS, users, tokens batch/renew, orders, charts | @ubuntu_game_ui_bot | 8.8, 2.5 | §10.11 | L |
|| 2.11 | Daemon：verify-token client + 60s cache + sync loop + dynamic port | @ubuntu_game_combat_bot | 8.6 | §9, §4.1 | M |
|| 2.12 | Flutter login + token 导入 + secure storage | @ubuntu_game_char_bot | 8.9 | §3.2 | M |
|| 2.13 | @ubuntu_game_bot 集成测试：Portal/Admin/API 全链路验证 | @ubuntu_game_bot | 2.9, 2.10 | full spec | M |

**注意**：
- 2.6-2.8 (Xray-core) 是最大技术风险点。建议 **后端优先** 2.1-2.5（Manager 完整 API）再攻 Xray
- 2.11 (Daemon) 可与 Xray 改造并行 — Daemon 先写 HTTP client 层，等 verify-token 就绪再联调
- Frontend + Mobile 主要依赖 Manager API（2.1-2.5），不依赖 Xray

### Sprint 3：Flutter 完成 + 节点部署（A+C 为主）

**目标**：Flutter 全流程可连接，CF-WS/REALITY 节点部署就绪

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
|| 3.1 | Flutter 主界面：节点列表 + 连接状态 + 续费入口 | @ubuntu_game_char_bot | 2.12 | §3.3 | M |
|| 3.2 | Flutter 设备管理 + 到期提醒 | @ubuntu_game_char_bot | 2.12 | §3.4 | M |
|| 3.3 | Flutter VPN 集成：Xray config 生成 + VPN service | @ubuntu_game_char_bot | 2.12 | §3.5 | L |
|| 3.4 | Daemon 完整：Loki push + CF IP firewall | @ubuntu_game_combat_bot | 2.11 | §15, §14.4 | S |
|| 3.5 | CF-WS Nginx 伪装站部署脚本 | @ubuntu_game_combat_bot | 8.2 | §14.5 | M |
|| 3.6 | REALITY 节点部署脚本 | @ubuntu_game_combat_bot | 8.2 | §14 | S |
|| 3.7 | Manager 生产部署 + systemd service | @ubuntu_game_bot | 2.1-2.5 | §14.2 | S |
|| 3.8 | @ubuntu_game_bot 集成验证：Flutter + 节点全链路 | @ubuntu_game_bot | 3.1-3.6 | full spec | M |

### Sprint 4：集成 + 上线

**目标**：全链路 E2E，生产环境就绪

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
|| 4.1 | 官网购套餐 → webhook → Flutter 连接 E2E | 全员 | all | §6 | L |
|| 4.2 | 多设备连不同节点 + online verify | 全员 | 3.3, 8.6 | §14B | M |
|| 4.3 | 流量计费 + 设备上限 + 到期自动禁用 | @ubuntu_game_combat_bot + @ubuntu_game_bot | 2.5, 2.11 | §6, §7 | M |
|| 4.4 | CF Pages 生产部署（portal + admin） | @ubuntu_game_bot | 2.9, 2.10 | §10.4, §10.5 | S |
|| 4.5 | 安全审计：CORS/CSRF/XSS/Cookie/IP firewall | @ubuntu_game_bot | all | §10.9 | M |
|| 4.6 | Loki + Grafana 监控就绪 | @ubuntu_game_combat_bot | 3.4 | §15 | M |
|| 4.7 | 上线前 checklist 执行 | @ubuntu_game_bot | all | §22.5 | S |

---

## 5. 依赖矩阵（什么可以并行）

```
                    1.0  1.1  1.2  1.3  1.4  1.5  1.6  1.7  1.8  1.9  2.1  2.2  2.3  2.4  2.5  2.6  2.7  2.8  2.9  2.10 2.11 2.12 3.1  3.2  3.3
Backend:
 1.0 Git init          X
 1.1 Flutter install        X
 1.2 Xray clone                   X
 1.3 Manager skeleton                    X
 1.4 CSRF middleware                           X
 1.5 Auth API                                        X
 1.6 Node API                                                                                     X*
 2.1 Payment                                                              X
 2.2 Token login                                                              X
 2.3 Email                                                                          X
 2.4 Client API                                                                           X
 2.5 Admin API                                                                                    X
 2.6 Xray verify                                                                                        X
 2.7 Xray rate limit                                                                                         X
 2.8 Xray audit                                                                                                  X
 2.11 Daemon                                                                                                      X
                                        ^^^ 1.3 ~ 1.6 是 Manager Core 不能并行 ^^^

Frontend:
 1.7 Portal skeleton                          o   o                                              ← 等 1.5 auth 或 mock
 1.8 Admin skeleton                            o   o                                              ← 等 1.5 auth 或 mock
 2.9 Portal full                                                                  o               ← 等 2.1-2.2
 2.10 Admin full                                                                      o           ← 等 2.5

Mobile:
 1.9 Flutter mock          X                                                                      ← 完全独立
 2.12 Flutter login                                                                                       X
 3.1 Flutter main                                                                                                X
 3.2 Flutter devices                                                                                                  X
 3.3 Flutter VPN                                                                                                               X

QA:
 1.10 Test plan                      o                                       ← 等 1.3
 2.13 API regression                                                                   o    o     ← 等 2.9-2.10
 3.8 Flutter / node tests                                                                                         X
 4.1-4.3 E2E                                                                                                                  全依赖
```

**X** = 可以立刻开始 | **o** = 有外部依赖 | **空格** = 依赖上级完成

---

## 6. 技术风险点 & 缓解策略

| 风险 | 等级 | 缓解策略 |
|:---|:---|:---|
| **Xray-core fork 改造** (Phase 2) | 🔴 高 | 后端优先完成 Manager API（Sprint 1），如果 Xray 改造超出预期，Manager + Daemon + Frontend 仍可独立上线，Xray 延后迭代 |
| **Flutter VPN 集成** | 🟡 中 | Mobile 先用 mock config 跑通 UI + 业务逻辑；VPN 集成作为 Sprint 3 独立任务 |
| **支付回调联调** (BEpusdt/Payoneer) | 🟡 中 | Webhook 在开发环境用 ngrok 暴露 localhost 测试；本地用 mock webhook 脚本 |
| **Manager 单点** | 🟢 低 | MVP 单实例 SQLite 够用；PostgreSQL deferred |

---

## 7. 需要提供的信息

### Sprint 0 开始前需要

- [x] ~~GitHub Token / SSH Key~~ — **已有**（仓库已 clone 到本地）
- [x] ~~仓库名~~ — **`airport`（已存在）**
- [x] ~~GitHub 用户~~ — **`ouyexiaogongzhu`（已存在）**

### Sprint 1 结束时需要

- [ ] **VPS IP + SSH 信息** — Manager 生产部署
- [ ] **Cloudflare API Token**（Pages + DNS 权限）+ **Zone ID**
- [ ] **rfplay.uk 已 Cloudflare 托管确认**
- [ ] **BEpusdt 实例地址 + Auth Token**（如有）
- [ ] **Payoneer API Key + Merchant ID**（如有）
- [ ] **Resend/Brevo API Key**（如有）

---

## 8. 任务调度机制

### 工作流程

```
@ubuntu_game_bot (组长)
  │ 1. 群里写任务描述（你可见）
  │ 2. 终端 dispatch：hermes chat --profile <agent> -q "任务" --quiet
  ├─→ @ubuntu_game_combat_bot
  │   完成 → Telegram 群回复 ✅ [做了什么] [关键结果]
  │
  ├─→ @ubuntu_game_ui_bot
  │   完成 → Telegram 群回复 ✅ [做了什么] [关键结果]
  │
  └─→ @ubuntu_game_char_bot
      完成 → Telegram 群回复 ✅ [做了什么] [关键结果]
```

### 为什么不能 @mention

**Telegram 限制**：bot 发的消息不会被群里其他 bot 收到。因此 ubuntu_game_bot 不能通过 @mention 给其他 agent 派发任务。必须用终端 dispatch。

### 终端派发命令格式

```bash
# 派发给后端
hermes chat --profile ubuntu_game_combat_bot -q "
任务: XXX
参考: airport_system_design.md §X.X
分支: feat/xxx
完成后在群中回复 ✅ 结果
" --quiet

# 派发给前端
hermes chat --profile ubuntu_game_ui_bot -q "..." --quiet

# 派发给移动端
hermes chat --profile ubuntu_game_char_bot -q "..." --quiet
```

### 规则

1. **组长调度**：@ubuntu_game_bot 在群中写任务描述（你可见），同时通过终端 dispatch 派发给对应 agent
2. **自主执行**：接到任务后 agent 自主完成，不需要请示
3. **报告完成**：完成后在群中回复 `✅ [概要] [关键结果]`
4. **问题处理**：遇到阻塞（环境/依赖/设计歧义）→ 在群中报告问题并等待组长决策
5. **并行**：不同 agent 可同时工作（写不同目录/文件），无冲突
6. **集成测试**：@ubuntu_game_bot 在每个子任务完成后做集成验证，再派发下一个

### Sprint 0 前置条件

开发开始前，@ubuntu_game_bot 需要先：
- [ ] 安装 Go 1.22+
- [ ] 安装 Flutter + Android SDK
- [ ] 创建 monorepo 目录骨架（manager/ portal/ admin/ daemon/ client/ xray-core/）
- [ ] 初始化 Go module / Vue 3 项目 / Flutter 项目
- [ ] 配置各 agent 的工作目录和权限

---

## 9. 产出物总览

| Sprint | 可演示的成果 |
|:---|:---|
| **Sprint 1** | Portal 登录/注册页可填、Admin 登录页可填、Flutter 骨架有 UI、Manager API 认证全链路 curl 可调通 |
| **Sprint 2** | Portal 可下单支付、Admin 可管理节点/用户/Token、Xray 改造验证回调跑通 |
| **Sprint 3** | Flutter App 可连节点（mock/real）、CF-WS 节点部署脚本可跑通、Manager 生产部署 |
| **Sprint 4** | 官网买套餐 → Flutter 连接 → 流量计费 → 到期提醒，全链路 E2E 验证通过，生产环境就绪 |

---

*本文档由组长维护，各 bot 根据本文档 + `airport_system_design.md` §章节号进行开发。*
