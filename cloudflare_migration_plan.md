# RFPlay 平台方案：Cloudflare Workers + Tunnel 节点

> **状态（2026-09-24）**：**v0.1.0** — Go manager 与 Flutter 客户端已退役，后端为 Workers（TS/Hono）+ D1 + KV。里程碑 A（A0–A3）已在测试节点 `w1` 验收：订阅与代理在 **v2rayNG / v2rayA / Clash Verge**（Android / Ubuntu / MacBook）通过。支付（里程碑 B）暂缓。
> **关联**：[README.md](README.md)；[airport_system_design.md](airport_system_design.md) 为早期设计，仅供参考。

---

## 1. 已确定的决策

| 项 | 决策 |
| :--- | :--- |
| 后端 | 单个 Worker `rfplay-api`（Hono 按 public/client/web/admin/node 分路由），共享 D1；不拆微服务 |
| 客户端 | 不自研 App。用户用通用客户端导入订阅 URL：Clash 系（mihomo 内核，`/clash`）为主，V2rayNG 等用 Base64（`/links/:token`） |
| 节点形态 | **所有节点走 Cloudflare：VLESS/VMess + WS 或 XHTTP，cloudflared Tunnel 回源**（`nodes.network`）。Xray 只监听 `127.0.0.1`，VPS 零公网端口、无需证书。Reality 已删除 |
| 协议 | 只保留 `vless`、`vmess`；Shadowsocks / Trojan 已下线 |
| 商品 | 每个商品单独配置时长、流量（每 30 天额度）、限速 |
| 支付 | 暂不做（里程碑 B）。先由管理员在后台手动开通 |
| 出口 IP | 不处理：节点出站直连（`freedom`），目标网站可见 VPS 出口 IP |
| 节点面板 | 不引入 Marzban / 3X-UI：自研 daemon + Worker 为唯一控制面，只借鉴其设计 |
| 会话 | HS256 JWT 放 httpOnly cookie（`session` / `admin_session` / `refresh`）+ CSRF 双提交；跨站 pages.dev 用 Bearer 兜底 |
| 限流 / 人机校验 | 不在 Worker 内做进程级限流；由 CF WAF 规则 + Turnstile 承担 |

---

## 2. 架构

```
                         Cloudflare
  ┌────────────────────────────────────────────────────────────┐
  │ Pages   www.rfplay.uk（portal） / admin.rfplay.uk（admin）   │
  │ Worker  api.rfplay.uk/api/v1/*（rfplay-api）                 │
  │   ├─ D1  rfplay：users / nodes / orders / products / traffic │
  │   ├─ KV  CACHE：订阅响应缓存 60s                               │
  │   ├─ R2  BACKUPS（已绑定，未使用）                              │
  │   └─ Cron 每小时：标记过期、按 30 天周期重置流量                    │
  │ Tunnel  node-xx.rfplay.uk → VPS 127.0.0.1:<xray-port>        │
  │         pay.rfplay.uk     → VPS BEpusdt（里程碑 B）            │
  └───────────────┬───────────────────────────┬────────────────┘
                  │ 订阅 URL                    │ /api/v1/node/:token/*（HMAC）
          用户的 Clash / V2rayNG              VPS：Xray + daemon（Go）+ cloudflared
                                              入站只留 SSH
```

**节点数据流**：daemon 定时拉 `GET /node/:token/config`，按版本号决定是否重载 Xray；从 Xray StatsService 读按用户流量，`POST /node/:token/traffic/report` 批量上报；上报成功后才扣除本地增量。

**支付数据流（里程碑 B）**：portal 下单 → Worker 调 BEpusdt 建单拿收银台 URL → 用户付款 → BEpusdt 回调 Worker 验签（MD5，WebCrypto 不支持，用自带 `md5.ts`）→ D1 batch 以「订单仍为 pending」为闸门开通，重复回调零副作用。PayPal 走 Orders v2 + 官方 verify-webhook-signature 接口，商品须以 USD 计价。

**Worker Secrets**：`JWT_SECRET`、`TURNSTILE_SECRET`；里程碑 B 另需 `BEPUSDT_API_URL`、`BEPUSDT_TOKEN`、`BEPUSDT_SECRET`、`PAYPAL_CLIENT_ID`、`PAYPAL_CLIENT_SECRET`、`PAYPAL_WEBHOOK_ID`。批量写入：`deploy/cloudflare/push-secrets.sh`。

**wrangler vars**：`MOCK_PAY_ENABLED="0"`、`PORTAL_URL`、`TURNSTILE_DISABLED="1"`（⚠️ 配好 `TURNSTILE_SECRET` 后必须删掉）。

---

## 3. 部署与运维

### 3.1 CI/CD

- **Worker**：push main → `.github/workflows/deploy-worker.yml` 执行 `wrangler d1 migrations apply rfplay --remote`，再 `wrangler deploy`。⚠️ 推到 main 即上线生产
- **Pages**：portal / admin 由 GitHub Actions `deploy-pages.yml` 构建后 `wrangler pages deploy` Direct Upload（不依赖 Pages Git 集成）
- **本地**：`npm run db:migrate`（本地 D1）、`wrangler dev`；测试 `npx vitest run`（D1 相关测试用 `node:sqlite` 跑真实迁移，见 `src/testing/d1.ts`）
- **回滚**：重新部署上一版 Worker；D1 可用 Time Travel 回到 30 天内任意时间点

### 3.2 证书

用户到 CF 边缘由 Universal SSL 覆盖；Worker 与 Pages 无源站；Tunnel 回源走本机 loopback 明文。**无任何证书需要申请或续期**。

### 3.3 新增节点（手动步骤，自动化见 §5.3 部署脚本）

1. 后台建节点（域名、本地端口、WS Path）→ `POST /admin/nodes/:id/token` 生成 token
2. VPS 上运行 `deploy-node-cf-ws.sh`（装 Xray + daemon + cloudflared）
3. Zero Trust 里给 Tunnel 加公共主机名 `node-xx.rfplay.uk → http://127.0.0.1:<port>`（自动建橙云 CNAME）

### 3.4 尚需人工完成

| # | 事项 | 阻塞什么 |
| :--- | :--- | :--- |
| 1 | Turnstile site key → Pages env；`wrangler secret put TURNSTILE_SECRET`；删掉 `TURNSTILE_DISABLED` | 注册/登录防刷 |
| 2 | 正式 `JWT_SECRET` | 正式会话 |
| 3 | VPS：Tunnel token + 公共主机名 | 节点回源 |
| 4 | Access 保护 `admin.rfplay.uk`；Email Routing | 运维 |
| 5 | 老用户数据（若 VPS 上有 `manager.db`）→ `deploy/cloudflare/dump-to-seed.sh` → `wrangler d1 import` | 老用户迁移（无则跳过） |
| 6 | git 凭据需要 `workflow` 权限才能推送改动 `.github/workflows/` 的提交 | 推送 |
| 7 | 里程碑 B：BEpusdt（VPS 上只开 TRON 单链）、PayPal 应用与 webhook | 收款 |

### 3.5 免费额度风险

| 资源 | 免费额度 | 风险与对策 |
| :--- | :--- | :--- |
| Workers | 10 万请求/天 | 订阅拉取为主；超限升级 $5/月 Workers Paid，无架构变更 |
| D1 | 500 万行读、**10 万行写**/天 | 流量上报是写入大户：上报间隔 × 节点数 × 用户数要控制；必要时先进 KV 聚合，再由 Cron 批量写入 |
| KV | **1 千写**/天 | 只缓存订阅响应（60s TTL），禁止按请求写 |

---

## 4. 未完成功能与已知 bug

> 2026-09-23 代码审查。P0 = 不修无法运营；P1 = 资损/投诉；P2 = 体验或运维。✅ = 已修复，— = 因决策作废。编号供 §5 引用。

### 4.1 P0：节点链路不通

| # | 问题 | 位置 |
| :--- | :--- | :--- |
| 1 | ✅ Worker 没有 daemon 调用的 `GET /api/v1/node/:token/config` 和 `POST /api/v1/node/:token/traffic/report`（A2：`routes/node.ts` + HMAC） | `index.ts`；daemon `sync.go:166,404` |
| 2 | ✅ 补路由须对齐格式：daemon 期望 `{node_id,name,protocol,config}`，批量上报 `{node_id,traffic:[...]}`（A2） | `sync.go:157,398` |
| 3 | ✅ 无按用户流量统计（A2：`email: u{id}` + StatsService，daemon 用 `statsquery` 读取） | `admin.ts` buildNodeXrayConfig；`sync.go:430` |
| 4 | — Reality `privateKey` 为空（Reality 已删除） | |
| 5 | ✅ 配置版本号只按用户 ID 集合计算，改节点端口/传输后 daemon 不重载（A2：版本号含传输配置） | `admin.ts` userSetVersion；`sync.go:228` |
| 6 | ✅ 服务端与客户端共用一个 `security` 字段，inbound 监听所有网卡（A2：固定 Tunnel 形态，只监听 127.0.0.1） | `xrayuri.ts`；`admin.ts` |
| 7 | ✅ 部署脚本写死 `node_id: 1`（A2：取自配置响应） | `deploy/node-*/deploy-*.sh` |
| 8 | ✅ Shadowsocks / Trojan 已从白名单、订阅、Clash 下线，存量节点由 `0002` 置为 inactive | |

### 4.2 P0：到期、超额、封禁不停服

| # | 问题 |
| :--- | :--- |
| 9 | ✅ 过期用户可无限使用（A1：统一资格判定 + Cron 标记过期） |
| 10 | ✅ 流量上限不生效（A1） |
| 11 | ✅ 封禁不停服（订阅与节点侧 A1；登录态接口 A3 回库检查 `status`） |
| 12 | ✅ 改用户状态会重生 `client_token`（A0） |
| 13 | 流量按月重置 ✅（A1）；限速为已知限制：Xray 不支持按用户限速，`rate_limit_bps` 不下发（推迟） |

### 4.3 P1：支付与订单（里程碑 B）

| # | 问题 | 位置 |
| :--- | :--- | :--- |
| 14 | PayPal 下单无 `return_url/cancel_url`，无 capture 调用，订单永远不完成 | `payments.ts` |
| 15 | PayPal webhook 验签传重新序列化的字符串，很可能恒失败；缺 `custom_id` 时回退 PayPal 订单号，可能激活错订单 | `payments.ts`；`payment.ts` |
| 16 | BEpusdt 付款后跳转不存在的 `API域名/user/orders/:id`；`PayResult.vue` 无入口 | `web.ts` |
| 17 | 回调不核对金额/币种；先收到 failed 再来 paid 不会激活 | `payment.ts` |
| 18 | ✅ 商品时长/流量写死（A1：按商品字段开通） | |
| 19 | 下单即扣库存，pending 不过期、失败不回补，可被刷空；默认 provider 为已关闭的 `mock` | `web.ts` |
| 20 | 退款不收回时长和流量；UPDATE 无 `status='paid'` 条件，并发退款重复加库存 | `admin.ts` |
| 21 | ✅ portal 价格单位与币种符号（A0） | |
| 22 | 回调交易号被丢弃，orders 无 `transaction_id/paid_at`，无法对账 | `payment.ts`；schema |

### 4.4 P1：认证与会话

| # | 问题 | 位置 |
| :--- | :--- | :--- |
| 23 | ✅ `/auth/refresh` 要求 session 仍有效，refresh token 形同虚设；跨站 Bearer 24h 后掉线（A3：只看 refresh token，前端 401 自动续期） | `auth.ts` |
| 24 | ✅ admin 初始化调 `/auth/validate`，优先读 portal 的 `session` cookie（A3：新增 `/admin/auth/validate`） | `auth.ts`；`admin/src/stores/auth.ts` |
| 25 | ✅ JWT 不可吊销，`role` 取自 token（A3：`token_version` 吊销，role 以库为准；改密接口尚不存在） | `jwt.ts`；`admin.ts` guard |
| 26 | ✅ 清除 csrf cookie 缺 `Secure`（A0） | |
| 27 | ✅ 未配 Turnstile secret 时放行（A0，改为 fail closed） | |
| 28 | ✅ 改用户名不校验空值/重名（A0） | |

### 4.5 P2：daemon

| # | 问题 | 位置 |
| :--- | :--- | :--- |
| 29 | ✅ 上报前就更新流量快照，上报失败则增量丢失；拉配置失败时本轮不上报（A2） | `sync.go` |
| 30 | 部分 ✅ Xray 重启失败仍记为已应用、不重试；崩溃不拉起；daemon 退出留孤儿进程（A2 已修）。剩余：用户变更仍整进程重启 Xray，全节点连接会断一次 | `sync.go`；`main.go` |
| 31 | `sync_interval` 是 `time.Duration`，JSON 须写纳秒整数（`60000000000`），写 `"30s"` 解析失败 | `config.go` |
| 32 | 部分 ✅ 默认 `default-token`/`localhost:8080` 能通过校验；`:9090` HTTP API 无鉴权（A2：默认值与非回环 `listen_addr` 拒绝启动）。剩余：`/api/v1/traffic` 流量恒为 0 | `config.go`；`server.go` |

### 4.6 P2：未实现的功能

| # | 功能 | 现状 |
| :--- | :--- | :--- |
| 33 | sing-box 订阅 | 仅输出节点名 + 协议名；portal 已下线入口 |
| 34 | 订阅二维码接口 | 返回 501；portal 前端自行生成，暂不需要 |
| 35 | D1 → R2 备份 | 未实现 |
| 36 | ✅ 后台改用户到期/流量/订阅状态、商品币种（A1） | |
| 37 | 设备管理页 | `AccountDevices.vue` 为占位 |
| 38 | Hysteria2 / gRPC | 不支持；XHTTP+Tunnel 已实现（`nodes.network=xhttp`），见 [docs/xhttp-cloudflare-design.md](docs/xhttp-cloudflare-design.md) |
| 39 | 杂项 | `PORTAL_URL` ✅；`online_nodes` 把 active 算在线；营收按下单时间；`Pay.vue` 超时提示不显示；Dashboard 空状态不显示 |

---

## 5. 修复计划

### 5.1 里程碑 A — 跑通（无支付）

**目标**：管理员在后台建节点、给用户开通商品 → 用户导入订阅 → 通过 Tunnel 节点正常上网 → 流量被计入 → 到期、超额或封禁后一个同步周期内被踢下线。

| 阶段 | 内容 | 状态 |
| :--- | :--- | :--- |
| A0 小修 | #12 #21 #26 #27 #28 #39 #8 | ✅ |
| A1 服务资格 + 后台开通 | 见 5.2 | ✅ |
| A2 节点链路（Tunnel） | 见 5.3 | ✅（w1 VPS 已实测） |
| A3 认证与会话 | 见 5.4 | ✅ |
| 验收 | 见 5.5 | ✅ v0.1.0（客户端多端已测；部分管控项见清单） |

### 5.2 A1 服务资格与后台开通（✅ 2026-09-23）

- 迁移改为 `wrangler d1 migrations apply`。生产库首次执行会重跑 `0001`（全部 `IF NOT EXISTS`，安全），然后执行 `0002`
- `0002`：products 加 `duration_days`、`traffic_bytes`、`speed_limit_bps`、`description`；users 加 `token_version`（A3 用）；补齐缺失的 `vless_uuid`；ss/trojan 节点置为 inactive
- `lib/entitlement.ts`：`serviceBlock()` 与等价 SQL `SERVICEABLE_SQL`，订阅链接、`/client/subscription`、节点配置用户列表共用；拒绝原因 `ACCOUNT_DISABLED` / `SUBSCRIPTION_PENDING` / `SUBSCRIPTION_EXPIRED` / `TRAFFIC_EXCEEDED`
- `activationStatement()`：按商品开通或顺延（到期 = max(now, 旧到期) + 时长），后台开通与支付回调共用
- Cron（每小时）：标记过期用户；按 `traffic_period_start` 对齐 30 天周期重置已用流量
- 后台：`POST /admin/users/:id/grant {product_id}`；`PUT /admin/users/:id` 可改订阅状态、到期、流量上限/已用、限速；商品增改支持新字段与币种（USD/CNY）

### 5.3 A2 节点链路（Tunnel 方案）（✅ 2026-09-23；w1 实测 ✅ 2026-09-24）

**完成情况**

- Worker：`routes/node.ts`（`GET /node/:token/config`、`POST /node/:token/traffic/report`）+ `lib/nodehmac.ts`（与 daemon `signRequest` 同算法，时间戳容差 ±300s，签名覆盖 body）。上报经 `json_each` 展开，一个 batch 固定 2–3 条语句，与用户数无关（D1 单次调用有查询数上限）；同一用户多条合并，不存在的用户不记录，节点计数与心跳一并更新。节点非 active 时下发空用户列表（daemon 应用后断开所有连接），而不是 403（403 会让 daemon 保留旧配置继续服务）
- `lib/nodeconfig.ts`：`buildNodeXrayConfig` 从 `admin.ts` 移出，后台预览与节点接口共用；用户列表用 `SERVICEABLE_SQL`，缺 `vless_uuid` 的用户跳过（不再内存补随机 UUID）。StatsService 走 `127.0.0.1:10085`（与节点端口冲突时 10086，写入 `_meta.api_port`）。路由屏蔽私网/回环目标，否则用户可经代理连本机 StatsService 重置流量计数；为此去掉了 `inboundTag → direct` 规则，让域名目标经 `IPIfNonMatch` 解析后再匹配 IP 规则。版本号 = FNV-1a（配置结构版本、协议、端口、path、api 端口、每个用户 `id:uuid`）截成 53 位，JSON 往返不丢精度
- #13 已知限制：Xray 没有按用户限速（policy 只有超时与统计开关），`speed_limit_bps` / `rate_limit_bps` 不下发到节点，推迟处理
- 订阅：`xrayuri.ts`、`subformats.ts` 按 `nodes.network`（`ws`|`xhttp`）下发 + tls + 443、host/sni = 节点域名；XHTTP 订阅固定 `mode=packet-up` + `xhttpMode=packet-up`（v2rayA）、`alpn=h2`。`nodes.port` 只作本机端口。后台收 `ws_path` + `network`（默认 `ws`，security 仍写死 `none`）；节点页含 Transport 下拉与「Token」按钮。`/admin/nodes/:id/config` 预览不再记心跳
- daemon：`xray api statsquery -reset` 读流量，读出的增量进 `pending`，上报 200 后才扣除；拉配置失败也上报；应用新配置前先 `xray run -test`，失败保留旧进程；重启前先收一次流量；重启失败不记为已应用（下轮重试）；崩溃后指数退避自动拉起；`Stop()` 与退出时结束 Xray（`main.go` 不再 `log.Fatalf` 跳过清理）；`node_id` 取自配置响应（配置里可省略）；默认 token/地址、非回环 `listen_addr` 拒绝启动
- 部署脚本：安装 cloudflared 并 `cloudflared service install <tunnel token>`；给 `--cf-api-token` 时经 API 写 Tunnel ingress（`hostname → http://127.0.0.1:<port>`）与橙云 CNAME，否则打印手动步骤；Xray 改由 daemon 独占管理（停用 `xray.service` 与旧 `rfplay-xray.service`，避免两个 Xray 抢端口）；结尾检查节点端口、9090、10085/10086 只监听回环地址，否则报错退出。原固定的 Xray `v25.3.8` 不存在（404），改为已验证的 `v26.3.27`（Xray 26 已把 WS 与 VMess 标为 deprecated，升级前需确认）
- 已删除 `deploy/node-reality/`
- 验证：`node.routes.test.ts`（真实 SQLite，19 例：签名正确/错误/过期/篡改 body、配置只含可服务用户、版本号变化、上报记账）；`subformats.test.ts` 更新。daemon 用本机缓存的 Go 1.26.5 工具链 `go vet` + `go test -race` 通过；并用 Xray 26.3.27 实测：生成的 vless/vmess 配置 `-test` 通过，经 WS 代理正常上网，`statsquery` 读到 `u{id}` 流量，经代理访问 `127.0.0.1:10085`/`localhost` 被拦；daemon 对假 manager 实测上报失败重发、`kill -9` 后 2s 拉起、SIGTERM 后 Xray 退出
- w1 实测（2026-09-24）：`deploy-node-cf-ws.sh` + Tunnel；Xray 仅 `127.0.0.1:28001`；daemon 拉配置/上报流量；客户端经 `w1.rfplay.uk:443` 连通，出口 IP 为 VPS（入站隐藏、出站不隐藏，见 §1）

**节点模型（唯一形态）**

| 项 | 取值 |
| :--- | :--- |
| Xray inbound | `listen: 127.0.0.1`，端口 = `port`，`network` = `nodes.network`（`ws`\|`xhttp`），`security=none`，path = `ws_path` |
| 客户端链接 | 地址 = 节点域名（`address`），端口固定 443，`security=tls`，host/sni = 节点域名；XHTTP 另带 `mode=packet-up`、`xhttpMode=packet-up`、`alpn=h2` |
| 回源 | cloudflared：`node-xx.rfplay.uk → http://127.0.0.1:<port>`（WS 与 XHTTP 相同） |
| DNS | Tunnel 自动创建的 CNAME（橙云），源站 IP 不出现在任何 DNS 记录里 |
| VPS 防火墙 | 入站只留 SSH（建议 SSH 也限制来源或改用 Cloudflare Access） |

`nodes.network` 已启用（`ws` 默认 / `xhttp`）。`security`、`server_name`、`reality_*` 列仍停用（D1 删列需重建表，暂不删）。

### 5.4 A3 认证与会话（✅ 2026-09-23）

- #25 / #11：`lib/session.ts` 统一校验。JWT 带 `tv`（缺省视为 0，旧 token 平滑过渡）；portal/web、admin、`/client/subscription` 每次请求回库比对 `token_version` 与 `status`，`role`/`username` 以库为准（降权下一次请求即生效）。退出（portal 与 admin）、后台把用户改为 `suspended`/`banned` 时版本号加一，该用户所有设备的 access/refresh 全部失效；解封不会恢复旧 token。退出只接受当前版本的 token 触发加一，被盗旧 token 不能反复踢人
- refresh token 带 `typ: 'refresh'`，不能当 access 用；access 也不能拿来续期。⚠️ 上线前签发的 refresh cookie 没有 `typ`，用户需重新登录一次
- #23：`POST /auth/refresh` 与新增的 `POST /admin/auth/refresh` 只看 refresh token（cookie 或 body `refresh_token`），不要求 access 有效。登录/注册响应新增 `refresh_token`；portal 与 admin 的 axios 拦截器在 401 时单飞续期并重放原请求，失败才跳回首页
- #24：新增 `GET /admin/auth/validate`（只读 `admin_session` + Bearer 兜底，非 admin 403）；admin 前端初始化改调它
- 未做：仓库里没有改密接口，目前无处可挂"改密加一"；以后加改密时调用 `bumpTokenVersion`
- 测试：`src/routes/session.routes.test.ts`（真实 SQLite，17 例）

### 5.5 里程碑 A 验收（v0.1.0 / 2026-09-24）

测试节点：`w1`（`w1.rfplay.uk` → Tunnel → `127.0.0.1:28001`）。

- [x] 用 `deploy-node-cf-ws.sh` 部署 Tunnel 节点；Xray 只监听回环，公网不可达代理端口
- [x] 后台建节点，给测试用户开通商品
- [x] **Clash Verge**（`/clash`）、**v2rayNG**（Base64）、**v2rayA**（Base64）均可导入并连通；已在 **Android / Ubuntu / MacBook** 验证
- [x] 产生流量后，D1 用户已用流量随 daemon 上报更新
- [x] 用户 `suspended` 时订阅返回 `ACCOUNT_DISABLED`（403）
- [ ] 过期 / 超额后一个同步周期内现有连接被断开（订阅 403 已覆盖；在线踢断依赖 daemon 下一轮空用户配置）
- [ ] 封禁再解封后，原 `client_token` 仍可用（未故意轮换 token 时）
- [ ] 管理员被降权后，下一次请求即失去后台权限
- [x] 节点域名解析为 Cloudflare；入站隐藏源站。**出口 IP = VPS 公网 IP**（预期，非缺陷）

### 5.6 里程碑 B — 支付（暂缓）

里程碑 A 验收通过后再排期。首发只接 BEpusdt，PayPal 放在最后。

- 迁移：orders 加 `transaction_id`、`paid_at`
- #17、#22：核对金额与币种；failed 之后到达的 paid 回调仍然激活；记录交易号与付款时间
- #16：BEpusdt 付款后跳回 portal 的 `PayResult` 页
- #19：pending 订单 30 分钟未付由 Cron 释放库存；失败、取消、建单失败都回补；默认 provider 不再是 `mock`
- #20：退款收回时长与流量；UPDATE 加 `status='paid'` 条件
- #14、#15（PayPal）：`return_url`/`cancel_url`、capture 接口、webhook 验签传原始事件对象、缺 `custom_id` 时拒绝

### 5.7 里程碑 C — 功能补全（按需）

- #33：sing-box 订阅，完成后恢复 portal 引导页的 Sing-box 标签
- #35：备份。先依靠 D1 Time Travel，再加每周 CI 任务执行 `wrangler d1 export` 上传 R2
- #37、#39：设备管理页；统计口径；`Pay.vue` 超时提示；Dashboard 空状态
- #8：如有需要，再补齐 Shadowsocks / Trojan 的服务端配置与真实密码
- #38：Hysteria2 等新协议；XHTTP（CF 隐藏入站）已实现，设计见 [docs/xhttp-cloudflare-design.md](docs/xhttp-cloudflare-design.md)
- 运维增强：节点拨测失败自动置 inactive + 告警；额度 >80% 告警；Telegram bot（查流量、续费、到期提醒）
