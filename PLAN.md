# RFPlay Airport — 产品交付计划

> **原则**: MVP 优先，用户通过第三方客户端（Shadowrocket/V2rayNG/Clash）使用服务。
> **并行**: **所有 worker agent 必须同时参与，不能闲置。** 无依赖的 Phase 可同时推进。
> **范围**: Flutter 客户端（Phase 3）定为 MVP+，但 Phase 0-2 中 char_bot 参与模拟层开发。
> **QR 码策略**: 前端 JS 生成（`qrcode.js`），不依赖后端 image 库。
> **VPN 连接**: Phase 0-2 用 UI 模拟动画/状态切换，Phase 3 再做真 VPN 集成。
> **指令**: 总指挥 ubuntu_game_bot 发 "开始" 后，各 agent 自主执行，不再询问。

---

## Phase 0 — 订阅系统（核心缺失 · 3天→1.5天）

**目标**: 用户买套餐 → 拿到订阅链接 → 导入客户端 → 连接

### Day 1 — 三路并行

```
Day 1 ──┬── combat_bot → Manager API 订阅端点
         │    • 订阅 Token 生成/重置
         │    • GET /api/v1/subscription/:token (V2ray base64)
         │    • GET /api/v1/subscription/:token/clash
         │    • 流量用量 header（已用/总/到期）
         │    • 节点配置编码（VMess/VLESS/SS URI）
         │
         ├── ui_bot → Portal 订阅页 + QR 码
         │    • /subscription 页面（显示链接 + QR 码 — 前端 qrcode.js）
         │    • 使用引导 tab（Shadowrocket / V2rayNG / Clash / Sing-box）
         │    • 重置订阅按钮 + 确认弹窗
         │    • 仪表盘流量显示（已用/总/到期/节点数）
         │
         └── char_bot → Flutter 订阅导入 + 节点展示（模拟层）
              • 订阅链接输入页（手动输入 / 扫码导入）
              • 解析 subscription base64 响应 → 节点列表
              • 节点卡片展示（延迟图标; 流量标签; 到期日）
              • VPN 连接按钮 UI 模拟（loading动画 → 已连接/已断开状态切换）
              • QR 码扫码（调用 camera 扫描 Portal 订阅 QR）
```

#### combat_bot 任务清单

| # | 任务 | 文件路径 | 说明 |
|---|------|---------|------|
| 1 | User 模型加 subscription 字段 | `manager/internal/model/user.go` | client_token, subscription_status/tier, traffic_limit/used, expire_time |
| 2 | 注册时自动生成 rf_ token | `manager/internal/handler/auth.go` | crypto/rand 32hex → "rf_" + hex |
| 3 | `POST /api/v1/auth/token-login` | `manager/internal/handler/auth.go` | rf_/at_ token 换 JWT (设计文档 §4.2) |
| 4 | `GET /api/v1/web/client-token` | `manager/internal/handler/subscription.go` | 返回 masked rf_ (设计文档 §4.3) |
| 5 | `POST /api/v1/web/client-token/regenerate` | `manager/internal/handler/subscription.go` | 重置 rf_, 旧失效 (设计文档 §4.3) |
| 6 | `GET /api/v1/client/config` | `manager/internal/handler/subscription.go` | 公共 bootstrap (设计文档 §4.4) |
| 7 | `GET /api/v1/client/subscription` | `manager/internal/handler/subscription.go` | Flutter Xray JSON (设计文档 §4.4) |
| 8 | `GET /api/v1/client/links/:token` | `manager/internal/handler/subscription.go` | V2ray base64 (设计文档 §4.4.1) |
| 9 | `GET /api/v1/client/links/:token/clash` | `manager/internal/handler/subscription.go` | Clash YAML (设计文档 §4.4.1) |
| 10 | `GET /api/v1/client/links/:token/singbox` | `manager/internal/handler/subscription.go` | Sing-box JSON (设计文档 §4.4.1) |
| 11 | `GET /api/v1/client/links/:token/qrcode` | `manager/internal/handler/subscription.go` | QR 码 PNG (设计文档 §4.4.1) |
| 12 | 已有用户批量生成 rf_ token | `manager/internal/db/migration.go` | SELECT id FROM users → INSERT client_token |

#### ui_bot 任务清单

| # | 任务 | 文件路径 | 说明 |
|---|------|---------|------|
| 1 | 订阅页 `/subscription` | `portal/src/views/Subscription.vue` | 显示订阅链接（可复制）+ QR 码（前端 qrcode.js） |
| 2 | 使用引导 | `portal/src/components/SetupGuide.vue` | 分 tab 图文步骤（Shadowrocket/V2rayNG/Clash/Sing-box） |
| 3 | 重置订阅 | `portal/src/components/ResetSubscription.vue` | 按钮 + 确认弹窗，调用 API |
| 4 | 仪表盘集成 | `portal/src/views/Dashboard.vue` | 已用/总流量 + 到期时间 + 可用节点数 |

#### char_bot 任务清单

| # | 任务 | 文件路径 | 说明 |
|---|------|---------|------|
| 1 | 订阅链接输入页 | `client/lib/screens/subscription/input_page.dart` | 手动输入 URL + 粘贴检测；扫码按钮入口 |
| 2 | Subscription 数据模型 | `client/lib/models/subscription.dart` | token, nodes[], traffic_used, traffic_total, expire_at |
| 3 | API service mock/real | `client/lib/services/subscription_service.dart` | 解析 subscription base64 → 节点列表；本地 mock JSON 先行 |
| 4 | 节点列表页 | `client/lib/screens/home/node_list_page.dart` | 节点卡片列表：名称/延迟图标/流量标签/到期日 |
| 5 | VPN 按钮 UI 模拟 | `client/lib/widgets/vpn_button.dart` | 圆形按钮: 未连接(灰) → 连接中(旋转) → 已连接(绿)/已断开(红)；纯 UI，无真正 VPN |
| 6 | QR 码扫码 | `client/lib/screens/subscription/qr_scanner_page.dart` | camera 扫描 → 解析链接 → 调用导入流程 |
| 7 | 导航骨架 | `client/lib/main.dart` | GoRouter 配置：home → subscription input → node list → settings；底部导航栏 |

### Day 2 — 联调收尾

| 检查点 | 方式 | 负责 |
|--------|------|------|
| 订阅链接 curl 验证 | 输出格式正确，base64 可解码 | combat_bot |
| Clash YAML 格式 | `yq` 或 Clash 解析 | ubuntu_game_bot |
| 重置后旧链接失效 | 返回 404 | combat_bot |
| 超量返回限制 header | `X-UltraUsage-Limit: exceeded` | combat_bot |
| Portal 订阅页 + QR 码 | 前端打开验证，扫码可识别 | ui_bot |
| Flutter 扫码导入 | camera 扫 QR → 解析 → 显示节点列表 | char_bot |
| Flutter VPN 按钮模拟 | 点击 → loading → 状态切换无崩溃 | char_bot |
| Admin 操作正常 | Admin 打开验证 | ubuntu_game_bot |
| 注册→订阅 token 自动生成 | 全流程走通 | ubuntu_game_bot |

---

## Phase 1 — 支付集成（2天）

| 任务 | 说明 | Agent |
|------|------|-------|
| 选一个真实支付渠道 | Alipay 沙箱 / Stripe 测试模式 | combat_bot |
| 支付回调接口 | 异步通知 → 自动开通套餐 | combat_bot |
| Portal 支付页 | 扫码支付 / 跳转支付 | ui_bot |
| Flutter 续费入口 | 从节点列表跳转 Portal 支付页（WebView/外跳） | char_bot |
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
| 🟢 | Flutter 配置加密（flutter_string_encrypt） | char_bot |

---

## Phase 3 — Flutter 客户端完整版（MVP+ · ~7天）

> **范围**: 真 VPN 集成 + 全面功能

| 阶段 | 任务 | 说明 |
|------|------|------|
| 3.1 | 订阅导入 → 解析配置 → 节点列表 | Phase 0 已有，切真实 API |
| 3.2 | Android VpnService 通道 | tun 设备读写，iptables 规则 |
| 3.3 | 节点测速 | ICMP ping + TCP connect 延迟 |
| 3.4 | 连接/断开/切节点 UI 联动 | VPN 按钮实装，状态实时同步 |
| 3.5 | 真实流量统计 + 图表 | 读取系统流量 / Xray API |
| 3.6 | iOS NETunnelProvider | 需 Mac 编译环境（deferred） |

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
Phase 0 — 订阅系统        ████████░░  3天  (三路并行 → 1.5天)
  ├─ combat_bot: 订阅 API 端点
  ├─ ui_bot:     订阅页 + QR 码
  └─ char_bot:   Flutter 订阅导入 + 节点展示 + VPN 模拟

Phase 1 — 支付集成        ████░░░░░░  2天
Phase 2 — 安全加固        ██░░░░░░░░  1天
Phase 3 — Flutter 完整版  ██████████  ~7天 (MVP+)
Phase 4 — E2E 验收        ████░░░░░░  2天
Phase 5 — 生产部署        ██░░░░░░░░  1天
                      ──────────────
MVP (含 Phase 0-2)         ≈ 6天
Full (含 Phase 3)          ≈ 13天
```

---

## Agent 职责分派

| Agent | Phase | 核心路径 | 说明 |
|-------|-------|----------|------|
| **combat_bot** | 0, 1, 2 | `manager/`, `daemon/`, `xray-core/`, `deploy/` | Go 后端 |
| **ui_bot** | 0, 1 | `portal/`, `admin/` | Vue3 前端 |
| **char_bot** | 0, 1, 2, 3 | `client/` | Flutter（Phase 0-2 模拟层，Phase 3 完整 VPN） |
| **ubuntu_game_bot** (总指挥) | 0-5 | 全局 | Admin 面板 + 集成测试 + 调度 |

---

## 并行执行矩阵

```
Phase 0 ──┬── combat_bot (订阅API) ──┐
           ├── ui_bot (Portal订阅页) ──┼── Day 1 三路并行
           └── char_bot (Flutter订阅导入+模拟) ┘
                                           ↓ Day 2 联调
Phase 1 ──┬── combat_bot (支付) ───┐
           ├── ui_bot (支付页) ──────┼── 并行
           └── char_bot (续费入口) ──┘
                                           ↓
Phase 2 ──┬── combat_bot (安全加固) ──┐
           └── char_bot (配置加密) ────┘ 并行
                                           ↓
Phase 3 ── char_bot (Flutter 完整VPN) ── (后续迭代)
                                           ↓
Phase 4 ── ubuntu_game_bot (E2E 验收) + 全 agent 修 bug
                                           ↓
Phase 5 ── ubuntu_game_bot (生产部署)
```

---

## 任务分发格式

总指挥分发任务时使用以下 dispatch 命令：

```bash
hermes chat --profile ubuntu_game_<agent> -q "Phase X: [标题]

任务清单:
1. [精确描述，含文件路径/API路径]
2. ...
3. ...

约束: ...
完成后 git add + commit 并回复完成状态" --quiet
```

---

*最后更新: 2026-06-24 (v3 — 移除 Phase 3.3 Xray 内核嵌入 FFI，架构确认：服务端 Xray Docker + 客户端 VpnService 隧道)*
