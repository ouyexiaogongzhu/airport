# Implementation Plan

> ⚠️ **此文档已归档。** 当前交付计划请参见 **[PLAN.md](PLAN.md)**（Phase 0-5）。

---

## 历史

本文档记录了 RFPlay Airport 项目初期的 Sprint 0-4 实施计划（2026-06 前），已被 [PLAN.md](PLAN.md) 中的 Phase 0-5 交付计划取代。

### 关键变更

| 旧结构 | 新结构 |
|--------|--------|
| Sprint 0: 环境搭建 | Phase 0: **订阅系统**（核心缺失） |
| Sprint 1: Manager Core | Phase 1: 支付集成 |
| Sprint 2: Portal+Admin 前端 | Phase 2: 安全加固 |
| Sprint 3: Flutter 客户端 | Phase 3: Flutter 客户端（**MVP+**） |
| Sprint 4: Daemon+Xray | Phase 4: E2E 验收测试 |
| — | Phase 5: 生产部署 |

### 变更原因

1. **MVP 范围收紧**: Flutter 客户端从首发移到 MVP+，用户先通过第三方客户端使用
2. **订阅系统优先级提升**: 机场核心功能（买套餐→拿链接→连接）未实现，列为 Phase 0
3. **安全/支付/部署自成 Phase**: 不再混在 Sprint 中，独立验收

详见 [PLAN.md](PLAN.md)。
