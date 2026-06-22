# RFPlay Airport — 产品交付计划

> **原则**: MVP 优先，用户通过第三方客户端（Shadowrocket/V2rayNG/Clash）使用服务。
> **并行**: 无依赖的 Phase 可同时推进，由 agent 组 ubuntu_game_*_bot 并行执行。
> **范围**: Flutter 客户端（Phase 3）定为 MVP+，首发不包含。

---

## Phase 0 — 订阅系统（核心缺失 · 3天）

**目标**: 用户买套餐 → 拿到订阅链接 → 导入第三方客户端 → 连接

### 后端 Manager API (combat_bot)

| 任务 | 说明 |
|------|------|
| Subscription 数据模型 | user_id, token(唯一), created_at, reset_at, is_active |
| 注册时自动生成 Token | User Create 时同步创建初始订阅 |
| `POST /api/v1/subscription/reset` | 重置 token，旧链接失效 |
| `GET /api/v1/subscription/:token` | 输出 V2ray base64 格式节点配置 + 流量用量 header |
| `GET /api/v1/subscription/:token/clash` | 输出标准 Clash YAML (proxies + proxy-groups + rules) |
| 节点配置编码 | 将节点信息编码为 VMess/VLESS/Shadowsocks URI |
| 流量用量接口 | 已用流量 / 总流量 / 到期时间 |

### Portal 用户前端 (ui_bot)

| 任务 | 说明 |
|------|------|
| 订阅页 `/subscription` | 显示订阅链接（可复制）+ QR 码 |
| 使用引导 | 分 tab: Shadowrocket / V2rayNG / Clash / Sing-box，图文步骤 |
| 重置订阅 | 按钮 + 确认弹窗，调 API |
| 仪表盘集成 | 显示已用/总流量 + 到期时间 + 可用节点数 |

### Admin 管理后台 (ubuntu_game_bot)

| 任务 | 说明 |
|------|------|
| 用户订阅管理 | 用户列表加「订阅」列，查看详情/token/流量 |
| 节点配置预览 | 各节点 vmess:// 链接 + QR 码 |

### 测试

| 检查点 | 方式 |
|--------|------|
| 订阅链接 curl 验证 | 输出格式正确，base64 可解码 |
| Clash YAML 格式 | `yq` 或 Clash 解析 |
| 重置后旧链接失效 | 返回 404 |
| 超量返回限制 header | 流量用完时订阅响应含 `X-UltraUsage-Limit: exceeded` |

**并行度**: combat_bot + ui_bot + ubuntu_game_bot 三路同时

---

## Phase 1 — 支付集成（2天）

| 任务 | 说明 | Agent |
|------|------|-------|
| 选一个真实支付渠道 | Alipay 沙箱 / Stripe 测试模式 | combat_bot |
| 支付回调接口 | 异步通知 → 自动开通套餐 | combat_bot |
| Portal 支付页 | 扫码支付 / 跳转支付 | ui_bot |
| Admin 订单管理 | 查看支付记录/订单状态 | ubuntu_game_bot |

---

## Phase 2 — 安全加固（1天）

| 优先级 | 任务 | Agent |
|--------|------|-------|
| 🔴 | 注册限流 + 验证码（数学/Recaptcha） | combat_bot |
| 🔴 | 去掉硬编码测试账号，改用 seed 脚本 | combat_bot |
| 🟡 | HTTPS 自签证书（开发环境） | ubuntu_game_bot |
| 🟡 | Rate limiting middleware（全 API） | combat_bot |
| 🟢 | CORS 白名单 | combat_bot |

---

## Phase 3 — Flutter 客户端（MVP+ · 可选）

> **注意**: MVP 首发不包含，用户用第三方客户端。此阶段为后续迭代预备。

| 阶段 | 任务 |
|------|------|
| 3.1 | 订阅导入 → 解析配置 → 节点列表 |
| 3.2 | Android VpnService 通道（tun 设备读写） |
| 3.3 | 嵌入 Xray-core 内核（go → .so） |
| 3.4 | 节点测速（ICMP ping） |
| 3.5 | 连接/断开/切节点 UI 联动 |
| 3.6 | 真实流量统计 + 图表 |
| 3.7 | iOS NETunnelProvider（需 Mac） |

---

## Phase 4 — E2E 验收测试（2天）

| 场景 | 步骤 | 预期 |
|------|------|------|
| **用户全流程** | 注册 → 登录 → 选套餐 → 支付 → 拿订阅链接 → 导入客户端 → 连接成功 | ✅ |
| **管理员全流程** | 登录 Admin → 添加节点 → 管理用户 → 创建产品 → 看统计 | ✅ |
| **节点同步** | Manager 增节点 → Daemon 自动同步 → 用户订阅可见 | ✅ |
| **限速/超量** | 用超流量 → 自动断流 → 续费恢复 | ✅ |
| **安全** | 未登录访问/SQL注入/XSS | ✅ |

---

## Phase 5 — 生产部署（1天）

| 任务 |
|------|
| Docker Compose (Manager + Daemon + Nginx + PostgreSQL) |
| Let's Encrypt 自动 HTTPS |
| 域名 + CDN 分发 |
| 服务健康监控 + 告警 |
| 数据库定时备份 |

---

## ⏱ 时间线

```
Phase 0 — 订阅系统        ████████░░  3天  (可并行 → 1.5天)
Phase 1 — 支付集成        ████░░░░░░  2天
Phase 2 — 安全加固        ██░░░░░░░░  1天
Phase 3 — Flutter 客户端  ████████████████  ~14天 (MVP+)
Phase 4 — E2E 验收        ████░░░░░░  2天
Phase 5 — 生产部署        ██░░░░░░░░  1天
                      ──────────────
MVP (砍 Phase 3)          ≈ 9天
Full (含 Phase 3)         ≈ 23天
```

---

## Agent 职责分派 (Phase 0-5)

| Agent | Phase | 核心路径 |
|-------|-------|----------|
| **ubuntu_game_combat_bot** | 0, 1, 2 | `manager/`, `daemon/`, `xray-core/`, `deploy/` |
| **ubuntu_game_ui_bot** | 0, 1 | `portal/` |
| **ubuntu_game_char_bot** | 3 (MVP+) | `client/` |
| **ubuntu_game_bot** (总指挥) | 0-5 | Admin 面板 + 集成测试 + 文档 + 调度 |

---

## 并行执行矩阵

```
Phase 0 ──┬── combat_bot (订阅API) ──┐
           ├── ui_bot (Portal订阅页) ──┼── 三路并行
           └── ubuntu_game_bot (Admin) ┘
                                          ↓ Phase 0 联调收尾
Phase 1 ──┬── combat_bot (支付) ───┐
           ├── ui_bot (支付页) ──────┼── 并行
           └── ubuntu_game_bot (Admin)┘
                                          ↓
Phase 2 ── combat_bot (安全加固) ── (ubuntu_game_bot 协助)
                                          ↓
Phase 3 ── char_bot (Flutter) ──── (MVP+，后续迭代)
                                          ↓
Phase 4 ── ubuntu_game_bot (E2E 验收) + 全 agent 修 bug
                                          ↓
Phase 5 ── ubuntu_game_bot (生产部署)
```

---

## 任务分发格式

各 agent 接收任务时统一使用以下 dispatch 命令：

```bash
hermes chat --profile ubuntu_game_<agent> -q "Phase X: [任务标题]

任务清单:
1. [精确描述，含文件路径/API路径]
2. ...
3. ...

约束: ...
完成后 git add + commit 并回复完成状态" --quiet
```

---

*最后更新: 2026-06-22 (Phase 0-5 完整规划)*
