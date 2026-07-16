# RFPlay Airport — 开发经验总结

> 全流程开发（后端 Go/Manager API、前端 Vue/Admin+Portal、Flutter 客户端、Docker/Nginx 部署、E2E 测试）踩坑记录与最佳实践。

---

## 架构设计

- **Nginx 统一入口 > 多端口暴露** — 所有服务（Portal/Admin/API/Xray）走同一端口 443，Vite dev server 通过 proxy `/api/v1` 解决 CORS，生产环境零配置切换
- **API 版本化** — 路径前缀 `/api/v1/`，Go Fiber 路由按角色分组（`public`/`web`/`user`/`admin`/`client`），权限层层隔离，职责清晰
- **Xray 不绑定 nginx 启动** — nginx 用 resolver 变量引用 xray 容器（`set $xray_ws "xray:443";`），xray 挂掉不影响 nginx 正常启动和 SPA 服务

---

## 开发流程

- **Multi-agent 并行派发** — ui_bot/combat_bot/char_bot 各负责一个子任务，总指挥负责集成+测试，开发效率远高于单线串行
- **先规划再动手** — 硬性规则。产出 PLAN.md 作为唯一执行依据，中间不问直接干
- **账号合并的链式影响** — 合并测试账号（`admin`/`fluttertest`/`ittest`/`demo`/`e2etest` → `a/1`, `b/1`）导致 E2E 测试、数据库外键引用全部断裂。**教训：合并前先 grep 所有硬编码引用**

---

## 前端（Vue 3 Portal / Admin）

- **Vite proxy > 硬编码 baseURL** — `.env` 污染 axios baseURL 是常见坑。统一走 Vite proxy（`/api/v1` → `localhost:80`），CORS 和路径问题一次解决
- **401 拦截器必须排除 `/public/`** — 否则登录页抛错后死循环重定向到登录页，用户永远进不去
- **Shared layout 避免导航闪烁** — Admin 侧边栏用嵌套路由 + `<router-view />` 替代每个页面独立 inline sidebar，否则路由切换时侧边栏折叠/闪烁
- **续订逻辑前端只需提示** — 后端 `activateSubscription` 已支持续订（自动延长 expire_time），前端只需展示 banner 让用户知情

---

## Flutter 客户端

- **Desktop 用 `dart-define` 传 API 地址** — 不能依赖相对路径。编译命令：`flutter build linux --dart-define=API_BASE_URL=http://localhost:80/api/v1`
- **Platform 条件编译** — VPN 在 Linux Desktop 只能模拟（拷贝订阅链接到剪贴板），Android 才调原生 `VpnService`。用 `Platform.isAndroid` 做条件分支，否则 `MissingPluginException`
- **16:9 宽高比** — `LayoutBuilder` + `AspectRatio(9/16)` 居中黑边。注意：16=高, 9=宽，所以是 `9/16` 不是 `16/9`

---

## Nginx + SSL

- **Cloudflare Origin CA 证书** — 直接从 VPS scp 拉到本地 `deploy/ssl/`，有效期 15 年（至 2041），无需 Let's Encrypt 续期。Docker 通过 `-v ./deploy/ssl:/etc/nginx/ssl:ro` 挂载
- **Docker 动态配置注入** — `docker cp` + `nginx -s reload` 实现配置热替换，不用重建镜像。适合开发阶段频繁调 nginx config
- **`listen 443 ssl http2;` 已废弃** — 新版 nginx（1.27+）要写成两行：
  ```nginx
  listen 443 ssl;
  http2 on;
  ```
- **HTTP → HTTPS 健康检查** — 纯 HTTP server block 需要保留 `/health` 反代，不能全部 301。否则容器 healthcheck 返回 301 被认为是失败

---

## E2E 测试

- **测试数据必须动态发现** — 硬编码 `user_id=2`、`node_id=1` 在账号/节点变更后全部 404。从 API 响应中提取目标 ID：
  ```python
  users = req("GET", "/admin/users")
  target_id = [u for u in users if u["role"] == "user"][0]["id"]
  ```
- **HTTPS 测试需 patch SSL context** — Python `urllib` 默认验证证书。Cloudflare Origin 证书 CN 不匹配 `localhost`，需：
  ```python
  ctx = ssl.create_default_context()
  ctx.check_hostname = False
  ctx.verify_mode = ssl.CERT_NONE
  ```
- **API_BASE 推荐用 HTTPS** — 测试经过完整反代链路（Nginx + TLS），不只是后端直连，覆盖了生产环境真实路径
- **sshpass/expect 缺失替代** — Python `pexpect` 无需 sudo 即可自动处理 SSH 密码登录 + 文件传输

---

## 运维

- **healthcheck 也要 HTTPS** — Nginx 将 HTTP 301 到 HTTPS 后，容器 healthcheck 必须从 `curl -sf http://localhost/health` 改为 `curl -skf https://localhost/health`
- **Docker healthcheck 最佳实践**：同样需要 `start_period` + 合理 `interval`，避免服务启动中的误判

---

## 常见 Bug 模式

| 症状 | 根因 | 修复 |
|------|------|------|
| Portal/Admin 白页 | `AdminLayout.vue` 用了 `<slot />` 而非 `<router-view />` | 替换为 `<router-view />` |
| Flutter VPN "已断开" 无法变 | `disconnect()` 不是 async，或缺少 `Platform.isAndroid` 守卫 | 加 `async`/`await` 和平台判断 |
| Admin 登录 401 循环 | axios 拦截器没有排除 `/public/` 路径 | 在拦截器中加 `if (config.url.includes('/public/')) return config` |
| Flutter 登录 "No host specified" | 未传 API_BASE_URL 编译参数 | `--dart-define=API_BASE_URL=...` |
| 镜像:443 端口 TLS 握手失败 | Nginx 容器有 SSL block 但无证书文件 | 挂载证书 + 检查 `ssl_certificate` 路径 |
