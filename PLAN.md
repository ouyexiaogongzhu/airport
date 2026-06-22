# RFPlay Airport System — 开发规划

> 开发组长视角 | 参考: `airport_system_design.md` (1953行), `implementation_plan.md`, `task.md`
> 
> 设计文档已定义全部 API 合约、DB Schema、Token 格式。本规划**不重复**设计细节，
> 仅做任务分解、并行策略、团队分工。实现时直接引用 `airport_system_design.md` §章节号。

---

## 1. 团队分工

| 角色 | 负责人 | Pipeline | 核心产出 |
|:---|:---|:---|:---|
| **后端** | @mac5_developer_backend_bot | A | Manager API (Go), Daemon, Xray-core fork, CF-WS 部署 |
| **前端** | @mac5_developer_frontend_bot | B | Portal Vue 3 → www.rfplay.uk, Admin Vue 3 → admin.rfplay.uk |
| **移动端** | @mac5_developer_mobile_bot | C | Flutter Client → App Store |
| **QA** | @mac5_developer_qa_bot | D | 测试策略、回归、安全审计、E2E 验收 |
| **组长** | vincent | 全局 | 基础设施、Git 仓库、Code Review、部署、协调 |

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

### Pipeline A：后端（Backend Bot）— 全周期独立

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

### Pipeline B：前端（Frontend Bot）— 与后端并行

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

### Pipeline C：移动端（Mobile Bot）— Mock 先行

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

### Pipeline D：QA — 从第一天介入

```
Sprint 1 ────────────── Sprint 2 ────────────── Sprint 3 ──────────── Sprint 4
┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌────────────────┐
│ 测试策略      │    │ API 回归测试  │    │ Xray 集成    │    │ 全链路 E2E     │
│ 测试环境搭建   │    │ Frontend     │    │ Flutter 测试  │    │ 安全审计       │
│ API 测试脚本  │    │ 功能测试      │    │ 节点部署验证   │    │ 上线 checklist │
└──────────────┘    └──────────────┘    └──────────────┘    └────────────────┘
```

---

## 4. Sprint 排期

### Sprint 1：基础设施 + 核心后端（A+B+D 并行）

**目标**：Manager 认证体系跑通，前端骨架可登录，Flutter 骨架搭建

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
| 1.0 | Git 仓库初始化 + monorepo 目录骨架 + GitHub push | 组长 | — | §22 | S |
| 1.1 | 安装 Flutter SDK + `flutter create client/` | 组长 | 1.0 | §3 | S |
| 1.2 | Xray-core 克隆 + 验证 Go build | Backend | 1.0 | task.md Phase 1 | S |
| 1.3 | Manager Go 项目 init + SQLite schema + TLS :443 + CF IP middleware | Backend | 1.0 | §3 DB schema, §14 | M |
| 1.4 | Manager MCPCookie + CSRF 中间件 (portal + admin 双套) | Backend | 1.3 | §4.2.1, §10.12 | M |
| 1.5 | Manager 双模式 login (cookie + Bearer) + logout/refresh/validate | Backend | 1.4 | §4.2.1, §4.2.2 | M |
| 1.6 | Managerverify-token + sync API（无 user_list） | Backend | 1.3 | §4.1, §4.1.1 | M |
| 1.7 | Portal Vue 骨架 + axios WithCredentials + login/register 页 + Plans | Frontend | 1.5 | §10.4, §2.2 | M |
| 1.8 | Admin Vue 骨架 + admin_session login + Users 列表页 | Frontend | 1.5 | §10.5, §10.6 | M |
| 1.9 | Flutter 骨架 + API Service + routing + mock data 模式 | Mobile | 1.1 | §3 | M |
| 1.10 | QA 测试策略文档 + 测试环境搭建 + API 测试脚本 (curl) | QA | 1.3 | full spec | M |

**并行分析**：

```
Week 1                Week 2                Week 3
├─┤1.0 组长 init─────┤
  ├─┤1.2 Xray clone──┤
  ├─┤1.3 Manager─────┤
  │                    ├─┤1.4 CSRF──────┤
  │                    │                 ├─┤1.5 Auth───────┤
  │                    └─┤1.6 Node API──┤
  ├─┤1.1 Flutter install─┤1.9 Flutter mock──────┤
  ├─┤1.10 QA test plan─────────────────────────┤
  │                    ├─┤1.7 Portal────────────┤
  │                    ├─┤1.8 Admin─────────────┤
```

**实际并发**：
- Backend 可同时跑：1.3（Manager 骨架）+ 1.2（Xray clone）— 不同目录
- Frontend 1.7 + 1.8 需要等 1.5（auth API）或先用 mock → 建议 mock 先行，等 1.5 完成后再切 real API
- Mobile 1.9 完全独立（mock data）
- 组长 1.0 完成后可并行做其它事

### Sprint 2：完整功能 + Xray 改造（A+B+C+D 四线全开）

**目标**：Portal/Admin 全功能就绪，Xray verify 回调跑通，Daemon MVP

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
| 2.1 | Manager 支付流程：plans/orders/payment callback + BEpusdt + Payoneer | Backend | 1.5 | §4.3, §4.5, §4.7 | L |
| 2.2 | Managerrf\_/at\_ token-login + issued\_tokens immutable CRUD | Backend | 1.5 | §3.1.1, §3.2 | M |
| 2.3 | ManagerResend/Brevo 注册验证邮件 | Backend | 1.5 | §14A | S |
| 2.4 | Managerclient config + subscription + traffic aggregation | Backend | 1.5 | §4.4, §6.3 | M |
| 2.5 | Manager 自到期 cron + Admin API（users/nodes/plans CRUD） | Backend | 1.5 | §7, §4.6 | M |
| 2.6 | Xray-core：inbound verify 回调 → Daemon /internal/verify | Backend | 1.2 | §4.1.1, Phase 2 | L |
| 2.7 | Xray-core：rate limiter (Go token bucket per session) | Backend | 2.6 | §5 | M |
| 2.8 | Xray-core：P2P audit + traffic stats + Loki push hooks | Backend | 2.6 | §13, §15 | M |
| 2.9 | Portal 全功能：checkout, pay poll, account, token copy/regenerate, devices | Frontend | 1.7, 2.1-2.2 | §2.2, §3.2 | L |
| 2.10 | Admin 全功能：nodes CRUD + CF DNS, users, tokens batch/renew, orders, charts | Frontend | 1.8, 2.5 | §10.11 | L |
| 2.11 | Daemon：verify-token client + 60s cache + sync loop + dynamic port | Backend | 1.6 | §9, §4.1 | M |
| 2.12 | Flutter login + token 导入 + secure storage | Mobile | 1.9 | §3.2 | M |
| 2.13 | QA API 回归 + Portal/Admin 功能测试 | QA | 2.9, 2.10 | full spec | M |

**注意**：
- 2.6-2.8 (Xray-core) 是最大技术风险点。建议 **后端优先** 2.1-2.5（Manager 完整 API）再攻 Xray
- 2.11 (Daemon) 可与 Xray 改造并行 — Daemon 先写 HTTP client 层，等 verify-token 就绪再联调
- Frontend + Mobile 主要依赖 Manager API（2.1-2.5），不依赖 Xray

### Sprint 3：Flutter 完成 + 节点部署（A+C 为主）

**目标**：Flutter 全流程可连接，CF-WS/REALITY 节点部署就绪

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
| 3.1 | Flutter 主界面：节点列表 + 连接状态 + 续费入口 | Mobile | 2.12 | §3.3 | M |
| 3.2 | Flutter 设备管理 + 到期提醒 | Mobile | 2.12 | §3.4 | M |
| 3.3 | Flutter VPN 集成：Xray config 生成 + VPN service | Mobile | 2.12 | §3.5 | L |
| 3.4 | Daemon 完整：Loki push + CF IP firewall | Backend | 2.11 | §15, §14.4 | S |
| 3.5 | CF-WS Nginx 伪装站部署脚本 | Backend | 1.2 | §14.5 | M |
| 3.6 | REALITY 节点部署脚本 | Backend | 1.2 | §14 | S |
| 3.7 | Manager 生产部署 + systemd service | Backend | 2.1-2.5 | §14.2 | S |
| 3.8 | QA Flutter 验证 + 节点部署验证 | QA | 3.1-3.6 | full spec | M |

### Sprint 4：集成 + 上线

**目标**：全链路 E2E，生产环境就绪

| ID | 任务 | Owner | 依赖 | 设计文档参考 | 预估 |
|:---|:---|:---|:---|:---|:---|
| 4.1 | 官网购套餐 → webhook → Flutter 连接 E2E | 全员 | all | §6 | L |
| 4.2 | 多设备连不同节点 + online verify | 全员 | 3.3, 1.6 | §14B | M |
| 4.3 | 流量计费 + 设备上限 + 到期自动禁用 | QA+Backend | 2.5, 2.11 | §6, §7 | M |
| 4.4 | CF Pages 生产部署（portal + admin） | Frontend | 2.9, 2.10 | §10.4, §10.5 | S |
| 4.5 | 安全审计：CORS/CSRF/XSS/Cookie/IP firewall | QA | all | §10.9 | M |
| 4.6 | Loki + Grafana 监控就绪 | Backend | 3.4 | §15 | M |
| 4.7 | 上线前 checklist 执行 | 组长 | all | §22.5 | S |

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

## 7. 你需要提供的信息汇总

### 开始开发前需要

- [ ] **GitHub Token / SSH Key** — 用于 `git init && push`
- [ ] **仓库名**（建议 `airport-system` 或 `rfplay-airport`）
- [ ] **GitHub 用户名或组织名**

### Sprint 1 结束时需要

- [ ] **VPS IP + SSH 信息** — Manager 生产部署
- [ ] **Cloudflare API Token**（Pages + DNS 权限）+ **Zone ID**
- [ ] **rfplay.uk 已 Cloudflare 托管确认**
- [ ] **BEpusdt 实例地址 + Auth Token**（如有）
- [ ] **Payoneer API Key + Merchant ID**（如有）
- [ ] **Resend/Brevo API Key**（如有）

---

## 8. 产出物总览

| Sprint | 可演示的成果 |
|:---|:---|
| **Sprint 1** | Portal 登录/注册页可填、Admin 登录页可填、Flutter 骨架有 UI、Manager API 认证全链路 curl 可调通 |
| **Sprint 2** | Portal 可下单支付、Admin 可管理节点/用户/Token、Xray 改造验证回调跑通 |
| **Sprint 3** | Flutter App 可连节点（mock/real）、CF-WS 节点部署脚本可跑通、Manager 生产部署 |
| **Sprint 4** | 官网买套餐 → Flutter 连接 → 流量计费 → 到期提醒，全链路 E2E 验证通过，生产环境就绪 |

---

*本文档由组长维护，各 bot 根据本文档 + `airport_system_design.md` §章节号进行开发。*
