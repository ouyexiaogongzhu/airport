# RFPlay Airport — 本地全链路开发计划

> **原则**: 全部本地开发，零外部服务依赖。
> **并行**: 无依赖的 Sprint 可同时推进，由 agent 组 ubuntu_game_*_bot 并行执行。

---

## Sprint 0 — 环境 & 工具链（已完成）

| 项目 | 状态 |
|------|------|
| Go 1.22+ 编译链 | ✅ 就绪 |
| Docker (可选) | ✅ 就绪 |
| 项目目录结构 | ✅ manager/ daemon/ xray-core/ deploy/ |
| 本地自签 TLS | ✅ 可用 |

**产出**: 编译通过、目录就绪、可本地启动 Manager + Daemon。

---

## Sprint 1 — Manager API（Go Fiber）

**路径**: `manager/`

| 模块 | 说明 | 并行依赖 |
|------|------|----------|
| 用户认证 | JWT 注册/登录/refresh | 无 |
| 节点管理 | CRUD Inbound/Outbound 配置 | 无 |
| 流量统计 | 本地 SQLite 记录用量 | 需认证模块 |
| 支付 mock | 本地模拟回调 | 需用户模块 |

**并行**: Sprint 1 ↔ Sprint 2 无依赖，可同时开发。

---

## Sprint 2 — 前端管理面板（本地 HTML/JS）

**路径**: `manager/web/`

| 模块 | 说明 | 并行依赖 |
|------|------|----------|
| 登录页 | JWT 登录 | 需 Sprint 1 API 定义 |
| 节点面板 | 节点列表/启停 | 需 Sprint 1 API 定义 |
| 用户管理 | 管理员操作 | 需 Sprint 1 API 定义 |

**并行**: 先定义 API 契约（OpenAPI YAML），前端 mock 数据独立开发。Sprint 2 ↔ Sprint 1 并行。

---

## Sprint 3 — Flutter 客户端（移动端）

**路径**: `client/`

| 模块 | 说明 | 并行依赖 |
|------|------|----------|
| 订阅拉取 | 从 Manager 拉配置 | 需 Sprint 1 API |
| 本地代理开关 | 调用 Xray-core 核心 | 需 Sprint 4 Daemon 接口 |
| 流量显示 | 图表/用量 | 需 Sprint 1 统计 API |

**并行**: 用 mock API 先行开发 UI，等 Sprint 1/4 稳定后联调。

---

## Sprint 4 — Xray-core + Daemon

**路径**: `xray-core/` `daemon/`

| 模块 | 说明 | 并行依赖 |
|------|------|----------|
| Xray-core fork | inbound verify 回调 | 无（独立二进制） |
| Rate limiter | 本地速率限制 | 无 |
| Daemon | verify-token 客户端、sync、Loki 上报 | 需 Sprint 1 API |
| CF-WS Nginx | 本地 WebSocket 配置 | 无 |

**并行**: Xray-core fork 可独立编译；Daemon 与 Sprint 1 定义 API 契约后并行。

---

## Sprint 5 — 全链路验证

| 步骤 | 说明 |
|------|------|
| 1. 启动 Manager | `go run .` |
| 2. 注册测试用户 | 通过 API / 前端 |
| 3. 创建节点 | Manager 写 SQLite |
| 4. 启动 Daemon | Daemon 拉配置 |
| 5. 启动 Xray-core | 加载配置、verify 回调 |
| 6. Flutter 连接 | 订阅拉取 → 代理连通 |
| 7. 验证流量统计 | Loki / SQLite 确认 |

---

## 并行执行矩阵

```
Sprint 0 ────────────────────────────────────────────── (已完成)
                Sprint 1 ──── Manager API ──────┐
                                                 ├── 并行
                Sprint 2 ──── 前端面板 ───────────┘
                Sprint 3 ──── Flutter ─────────── (mock 先行，晚 1 Sprint)
                Sprint 4 ──── Xray+Daemon ──────┐
                                                 ├── 与 Sprint 1 并行
                Sprint 5 ──── 全链路 ─────────────┘ (所有 Sprint 完成后)
```

## Agent 职责分派

| Agent | Sprint | 核心文件 |
|-------|--------|----------|
| ubuntu_game_combat_bot | Sprint 1, 4 | manager/, daemon/, xray-core/, deploy/ |
| ubuntu_game_ui_bot | Sprint 2 | manager/web/ |
| ubuntu_game_char_bot | Sprint 3 | client/ |

---

*最后更新: 2026-06-22*
