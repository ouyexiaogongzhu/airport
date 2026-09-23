# XHTTP over Cloudflare Tunnel — 架构设计

> **状态**：设计草案（2026-09-24），**未实现**。  
> **基线**：当前生产节点形态见 [cloudflare_migration_plan.md](../cloudflare_migration_plan.md) §1–3、§5.3：VLESS/VMess + **WS**，`cloudflared` Tunnel 回源，Xray 仅听 `127.0.0.1`，TLS 在 CF 边缘终结。  
> **目标读者**：实现 Worker / daemon / 部署 / 订阅格式的工程师与产品负责人。

---

## 1. Goals / Non-goals

### Goals

| # | 目标 |
| :--- | :--- |
| G1 | 在**不改变「隐藏入站 IP」模型**的前提下引入 Xray **XHTTP**（原 SplitHTTP）：客户端 → CF 橙云域名:443 → Tunnel → VPS 本机 Xray |
| G2 | **出口 IP 仍为 VPS**（`freedom` 直连），与现网一致；不引入 WARP / 出口代理 |
| G3 | 与现有控制面契约兼容：daemon 仍拉 `GET /api/v1/node/:token/config`、上报流量；配置版本号仍驱动重载 |
| G4 | 订阅面（Base64 URI、Clash/mihomo）能按节点正确下发 `network: xhttp`，主验收客户端仍为 Clash Verge（mihomo）+ v2rayNG |
| G5 | 可从 WS-only 节点**平滑迁移**（过渡期允许 WS / XHTTP 共存，按节点选择） |

### Non-goals

| # | 不做 |
| :--- | :--- |
| N1 | 不用 Reality / 不用节点自签 TLS；继续 **security=none** 于本机，TLS 只在 CF |
| N2 | 不改「单 Worker + D1 + daemon」控制面；不引入 3X-UI / Marzban |
| N3 | 不做客户端自研 App；sing-box 订阅仍可保持占位（现状 portal 已下线入口，见迁移方案 #33） |
| N4 | 不在本阶段做 Hysteria2 / gRPC 作为另一条产品线（迁移方案 #38） |
| N5 | 不保证所有第三方客户端对 XHTTP 的完整支持；以 mihomo + 较新 Xray 系客户端为主 |
| N6 | 不解决「出口隐藏」；产品话术继续区分「入站隐藏 / 出站不隐藏」 |

---

## 2. 为何从 WS 迁到 XHTTP

当前实现（硬编码）：

- 服务端：`workers/api/src/lib/nodeconfig.ts` → `streamSettings: { network: 'ws', security: 'none', wsSettings: { path } }`
- 客户端：`xrayuri.ts` / `subformats.ts` → 固定 `type=ws` / `network: ws` + `ws-opts`，端口 443 + tls，host/sni = 节点域名
- 部署：`deploy/node-cf-ws/deploy-node-cf-ws.sh`；默认 Xray **v26.3.27**（脚本已注明 WS 在 Xray 26 标为 deprecated）

| 维度 | 当前 WS | XHTTP（本方案） |
| :--- | :--- | :--- |
| CDN / 边缘语义 | WebSocket Upgrade；CF 支持，但与「普通 HTTPS 站点」指纹不同 | 纯 HTTP 请求语义（GET/POST 等），更贴近常规 CDN 流量 |
| 多路复用 | 依赖单条 WS；无 XMUX | 内置 **XMUX**（连接内多会话复用） |
| 填充 / 抗探测 | 基本无 | **x_padding**、可配 header/query 放置；可选 gRPC 伪装 header（stream-up） |
| ALPN / HTTP 版本 | 经 CF 后多为 H2 上的 WS | 原生按 HTTP/1.1 / H2 /（可选）H3 走 XHTTP framing；经 CF Tunnel 时需选对 **mode** |
| 维护前景 | Xray 26：**websocket deprecated** | 官方主推传输之一，替代旧 HTTP / 部分 gRPC 场景 |
| 与本仓库契合度 | 已验收（w1） | 迁移方案 #38 明确「尚未支持」；本设计补齐路径 |

**与 Cloudflare Tunnel 的关键取舍（实现约束，非口号）：**

- **packet-up**：分包上行，兼容性最好；经 CF CDN / Tunnel 时社区反馈最稳（含 stream-up 失败回退场景）。
- **stream-up**：流式上行，延迟/吞吐更好，但对中间盒缓冲敏感；CF 上常需 gRPC 伪装且 **H3 不可靠**；且较新 Xray 在 stream-up 侧更倾向 **h2c 回源**，而本仓库 Tunnel ingress 当前是 **`http://127.0.0.1:<port>`（HTTP/1.1 回源）**。
- **stream-one**：单请求承载上下行；非本阶段默认。

**本设计默认产品策略**：经 CF 的节点 **服务端 mode 兼容 auto（同时收 packet-up / stream-up）**；**订阅下发客户端 mode 固定 `packet-up`**（或产品拍板后改为 `auto`），优先连通性。后续可对单节点开放 `xhttp_mode` 覆写。

---

## 3. 提议流量路径

与现网拓扑相同，仅将「WS Upgrade」换成「XHTTP HTTP 请求」：

```
客户端（Clash / v2rayNG）
  │  TLS 1.3，SNI = 节点域名（如 w1.rfplay.uk）
  │  目标：节点域名:443
  │  ALPN：建议订阅写 alpn=h2（避免 H3 歧义；见 §7）
  ▼
Cloudflare 边缘（Universal SSL 终结 TLS）
  │  橙云 CNAME → <tunnel-id>.cfargotunnel.com
  ▼
cloudflared（VPS）
  │  Public Hostname：hostname → http://127.0.0.1:<nodes.port>
  │  （与 deploy-node-cf-ws.sh 现逻辑相同：HTTP 明文回源）
  ▼
Xray inbound
  listen: 127.0.0.1
  port:   nodes.port          // 本机端口，不出现在订阅里
  protocol: vless | vmess
  streamSettings:
    network: xhttp
    security: none            // TLS 已在边缘结束
    xhttpSettings:
      path: <nodes.ws_path 或后续 xhttp_path>
      // mode 省略 = auto（服务端双模式）
  ▼
outbound freedom → 互联网（出口 IP = VPS）
```

| 项 | 取值（对齐现网） |
| :--- | :--- |
| 客户端地址 | `nodes.address`（FQDN） |
| 客户端端口 | **443**（`CLIENT_PORT` in `xrayuri.ts`） |
| Host / SNI | 节点域名（与 `nodeTransport().host` 一致） |
| Path | 现 `ws_path`（默认 `/`）；迁移期可复用同列，或改名为语义中立的 `http_path`（见 §9） |
| VPS 公网代理端口 | **无**；防火墙仍建议仅 SSH |
| StatsService | 仍 `127.0.0.1:10085`（冲突则 10086），`_meta.api_port`；routing 仍 block 私网 |

**TLS**：继续零证书运维（迁移方案 §3.2）。Xray **不要**在本机开 `security: tls`。

**cloudflared ingress**：无需为 XHTTP 改 service URL 形态；仍是 `http://127.0.0.1:<port>`。若未来强依赖 stream-up + h2c，才需评估 Tunnel 是否支持 h2c 回源或改用本地反代——**不在 v1 范围**。

---

## 4. Xray inbound 配置草图

由 `buildNodeXrayConfig` 生成（daemon 原样写入并 `xray run -test`）。与现配置差异仅在 `streamSettings`；用户、stats、routing、`_meta` 不变。

```jsonc
{
  "inbounds": [
    {
      "tag": "in-vless",           // 或 in-vmess
      "listen": "127.0.0.1",
      "port": 28001,               // = nodes.port
      "protocol": "vless",
      "settings": {
        "clients": [
          { "id": "<uuid>", "email": "u1", "level": 0 }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "xhttp",
        "security": "none",
        "xhttpSettings": {
          "path": "/vcheck/",      // = nodeWsPath(node)
          "host": ""               // 本机无 TLS；Host 由 CF/客户端带，服务端通常可空
          // "mode": "auto"        // 省略即可；服务端同时接受 packet-up / stream-up
          // 可选后续调优（默认不写，避免订阅与服务端 drift）：
          // "scMaxEachPostBytes": "1000000",
          // "xPaddingBytes": "100-1000"
        }
      }
    },
    {
      "tag": "api",
      "listen": "127.0.0.1",
      "port": 10085,
      "protocol": "dokodemo-door",
      "settings": { "address": "127.0.0.1" }
    }
  ]
  // api / stats / policy / outbounds / routing / _meta 与现 a2-1 相同
}
```

**版本指纹**：`configVersion` 的 `parts` 必须纳入 `network`（及 mode，若入库），否则改传输后 daemon 不重载（历史 bug #5）。建议：

- 递增 `SCHEMA`（例如 `a2-1` → `a3-xhttp-1`）
- `parts` 增加 `network`、可选 `xhttp_mode`

**daemon**：无需理解 xhttp；仍整份 JSON 落盘 + 重启 Xray。前提是节点上的 Xray ≥ 支持 XHTTP 的版本（部署脚本当前 pin `v26.3.27` 已满足；WS 节点同版本即可双栈切换）。

---

## 5. 订阅 URI / Clash / sing-box 相对 WS 的字段变化

数据源：`client.ts` 现 `SELECT name, address, protocol, ws_path FROM nodes WHERE status = 'active'` —— 需扩展选出 `network`（或新列），否则无法分支。

### 5.1 共用约定（两种 transport）

| 字段 | WS（现） | XHTTP（新） |
| :--- | :--- | :--- |
| server / add | 节点域名 | 同 |
| port | 443 | 同 |
| tls / security | tls | 同 |
| sni / servername | 域名 | 同 |
| path | `ws_path` | 同 path 列 |
| host | 域名 | 同 |
| fp / client-fingerprint | chrome（vless URI） | 建议保留 |
| alpn | 未写（客户端默认） | **建议显式 `h2`**（Clash `alpn: [h2]`；URI `alpn=h2`） |

### 5.2 VLESS 分享链接（`encodeVless`）

现参数：`encryption, fp, host, path, security, sni, type=ws`

XHTTP 示例：

```
vless://<uuid>@w1.rfplay.uk:443?alpn=h2&encryption=none&fp=chrome&host=w1.rfplay.uk&mode=packet-up&path=%2Fvcheck%2F&security=tls&sni=w1.rfplay.uk&type=xhttp#w1
```

相对 WS 的变更：`type=xhttp`；增加 `mode`（及可选 `alpn`）；其余键序规则仍按 `xrayuri.ts` 字母序。

### 5.3 VMess 分享 JSON（`encodeVmess`）

| 键 | WS | XHTTP |
| :--- | :--- | :--- |
| `net` | `ws` | `xhttp`（或客户端文档要求的等价值；以实现时 v2rayNG 实测为准） |
| `path` / `host` / `tls` / `sni` / `port` | 同现 | 同 |
| `type` | `none` | 通常仍 `none`；mode 若客户端不认 JSON 扩展，可仅靠 path+net |

> 验收以 **v2rayNG 实际导入**为准；若某版不认 `net=xhttp`，产品决策：该节点仅 Clash 可用，或暂不下发 Base64 中的 XHTTP 节点。

### 5.4 Clash / mihomo（`buildClash`）

现：

```yaml
network: ws
ws-opts:
  path: "/vcheck/"
  headers:
    Host: w1.rfplay.uk
```

改为：

```yaml
network: xhttp
alpn:
  - h2
client-fingerprint: chrome
xhttp-opts:
  path: "/vcheck/"
  host: w1.rfplay.uk
  mode: packet-up
```

（`tls: true` / `servername` / `uuid` 不变。）

### 5.5 sing-box（`buildSingbox`）

现状仅输出 `tag` + `protocol` 名，**无真实 outbound**。本设计不强制补齐；若做，应对齐 sing-box 的 `vless` + `transport.type = "xhttp"`（字段名以当时 sing-box 文档为准），与 Clash 同步。

---

## 6. 从 WS-only 迁移

### 6.1 推荐模型：按节点 transport 标志（非全局一刀切）

复用 D1 已有列 **`nodes.network`**（schema 默认 `'ws'`；现 INSERT 写死 `'ws'`，admin 已停用读写）：

| `nodes.network` | 服务端 streamSettings | 订阅 |
| :--- | :--- | :--- |
| `ws`（默认，现网） | `network: ws` + `wsSettings.path` | 现逻辑 |
| `xhttp` | `network: xhttp` + `xhttpSettings.path` | §5 |

**不要**在同一 inbound 上同时开 WS+XHTTP（Xray 单 streamSettings）。「双栈」指 **集群内部分节点 WS、部分 XHTTP**，或 **同一 VPS 两个本地端口 / 两条 Tunnel hostname**（运维成本高，仅作灰度备选）。

### 6.2 过渡步骤（建议）

1. **代码**：`nodeconfig` / `xrayuri` / `subformats` / admin `parseTransport` 认 `network ∈ {ws,xhttp}`；默认仍 `ws`，零行为变化。
2. **测试节点**：新建或将 staging 节点 `network` 改为 `xhttp`，daemon 下一同步周期重载；用 Clash Verge + v2rayNG 回归。
3. **客户端公告**：要求 mihomo / Xray 内核版本支持 XHTTP；旧客户端拉到 XHTTP 节点会失败 → 灰度期保留至少一台 WS 节点在订阅里。
4. **批量切换**：后台按节点改 `network`；`SCHEMA` 或 version parts 变化 → daemon 自动重载。无需重装 cloudflared（ingress 不变）。
5. **WS 退役**：全部节点 `xhttp` 且观察窗口通过后，可将默认新建改为 `xhttp`；部署脚本改名（如 `deploy-node-cf-xhttp.sh`）或共用脚本加 `--transport`；文档更新迁移方案 §1「节点形态」行。

### 6.3 同一 path 切换

从 `ws` → `xhttp` 可保留同一 `ws_path` 与同一 Tunnel hostname：**切换瞬间旧 WS 客户端断开**，新订阅拉取后连 XHTTP。无需改 DNS。

### 6.4 不推荐

- 全局 feature flag 一次切全部节点（无法灰度客户端兼容性）
- 为「双协议」在订阅里对同一物理节点吐两条（易误导流量统计与用户选择）；若要做，应建成两个 `nodes` 行（不同 port/path/hostname）

---

## 7. Risks

| 风险 | 说明 | 缓解 |
| :--- | :--- | :--- |
| **CF 请求缓冲** | stream-up 依赖流式上行；CDN/代理缓冲会导致卡住或失败 | v1 订阅强制 **packet-up**；服务端 auto |
| **HTTP/2 vs HTTP/3** | CF 可能协商 H3；XHTTP stream-up 在 CF H3 上不可靠；packet-up 相对稳 | 订阅 **alpn=h2**；文档注明勿强制 h3 |
| **Tunnel 回源 HTTP/1.1** | 新版 Xray stream-up 优化偏向 h2c；与现 `http://127.0.0.1` ingress 组合可能仅 packet-up 可用 | 验收以 packet-up 为准；stream-up 列为实验 |
| **Path 指纹** | 固定冷门 path（如 `/vcheck/`）仍可被匹配；XHTTP 比 WS 更「像网站」但仍非隐身 | path 可配置；可选后续 padding / 更像静态资源的 path；不宣称抗审查 |
| **Xray 版本** | XHTTP 需足够新的 core；过旧二进制 `-test`/运行失败 | 部署脚本继续 pin ≥ `v26.3.27`；切换 transport 前检查 `xray version` |
| **WS deprecation（Xray 26）** | 继续用 WS 有未来移除风险 | 本设计的动机之一；过渡期保留 WS 代码路径至退役日 |
| **客户端碎片** | 部分旧 v2rayN / 非 mihomo Clash 不认 xhttp | 订阅过滤或双节点；portal 文案提示内核版本 |
| **VMess + XHTTP** | 协议仍可用，但生态与安全偏好偏向 VLESS | 实现两者；新品默认 VLESS（现已支持双协议） |
| **配置 drift** | 服务端 auto、客户端 packet-up 不一致通常仍可连；若服务端强制 stream-up 则会挂 | admin 校验：禁止「仅 stream-up」除非文档实验开关打开 |
| **版本号遗漏** | 只改 DB `network` 未进 `configVersion` → daemon 不重载 | 测例强制：改 network 后 version 变化（对齐现 `node.routes.test.ts`） |

---

## 8. 实现清单（映射到本仓库）

按依赖顺序；**本文件仅为设计，下列均为后续 PR 工作项**。

### 8.1 D1 / 数据模型

| 项 | 位置 | 工作 |
| :--- | :--- | :--- |
| 启用 `network` 列语义 | `migrations/0001_schema.sql` 已有列；新迁移可选 | 存量保持 `ws`；新建默认仍 `ws` 或产品改为 `xhttp` |
| Path 列 | `ws_path` | v1 **复用**（XHTTP 也写此列）；或迁移改名为 `http_path`（需改所有 SELECT）——见 §9 |
| 可选 `xhttp_mode` | 新列或 JSON 扩展 | v1 可不建列，代码常量 `packet-up` |

### 8.2 Worker：节点配置

| 项 | 文件 | 工作 |
| :--- | :--- | :--- |
| 按 network 生成 streamSettings | `workers/api/src/lib/nodeconfig.ts` | `ws` → 现逻辑；`xhttp` → `xhttpSettings`；`SCHEMA` 递增；version parts 含 network |
| NodeConfigRow | 同上 | 增加 `network` |
| daemon 拉配置 SELECT | `workers/api/src/routes/node.ts` | `SELECT` 增加 `network` |
| 后台预览 | `admin.ts` `GET /admin/nodes/:id/config` | 同上 |
| 测例 | `node.routes.test.ts` | XHTTP inbound 断言；改 network → version 变 |

### 8.3 Worker：订阅

| 项 | 文件 | 工作 |
| :--- | :--- | :--- |
| URI | `workers/api/src/lib/xrayuri.ts` | `NodeRow.network`；分支 `type=xhttp` + `mode` + `alpn` |
| Clash | `workers/api/src/lib/subformats.ts` | `network: xhttp` + `xhttp-opts`；保留 ws 分支 |
| 节点查询 | `workers/api/src/routes/client.ts` | SELECT 增加 `network` |
| 测例 | `xrayuri` / `subformats.test.ts` | 双 transport 快照 |

### 8.4 Admin API / UI

| 项 | 文件 | 工作 |
| :--- | :--- | :--- |
| `parseTransport` | `admin.ts` | 接受 `network: 'ws' \| 'xhttp'`；校验 path；INSERT/UPDATE 写入 `network`（不再写死 `'ws'`） |
| `nodeJson` | `admin.ts` | 输出 `network` |
| 节点表单 | `admin/src/views/Nodes.vue` | Transport 下拉；文案「WS Path」→「Path（WS/XHTTP）」 |
| 测例 | `Nodes.spec.ts`、`node.routes.test.ts` admin 段 | |

### 8.5 部署

| 项 | 文件 | 工作 |
| :--- | :--- | :--- |
| 脚本说明 | `deploy/node-cf-ws/deploy-node-cf-ws.sh` | 注释改为「WS 或 XHTTP，由后台 network 决定」；Tunnel 步骤不变 |
| 可选 | 新脚本或 `--transport` 仅文档提示 | 无需改 ingress JSON |
| Xray 版本 | 同上 `XRAY_VERSION` | 保持 ≥ 支持 XHTTP；切换前勿降级 |

### 8.6 Daemon

| 项 | 文件 | 工作 |
| :--- | :--- | :--- |
| 同步逻辑 | `daemon/internal/sync/sync.go` | **通常无代码变更**（透传 config） |
| 回归 | `go test` + 用生成的 xhttp JSON 跑 `xray run -test` | 与 A2 验收同级 |

### 8.7 文档

| 项 | 工作 |
| :--- | :--- |
| `cloudflare_migration_plan.md` §1 节点形态、§5.3 表 | 实现后改为「WS 或 XHTTP」 |
| README / 运维手册 | 客户端内核要求、alpn=h2、packet-up |

---

## 9. 开放决策（产品 / 负责人）

请拍板后再开实现 PR：

1. **默认 transport**：新建节点默认继续 `ws`，还是直接 `xhttp`？
2. **客户端 mode**：固定 `packet-up`，还是下发 `auto`（把兼容性交给客户端）？
3. **是否暴露 admin「XHTTP mode」高级项**（含实验性 stream-up），还是 v1 完全隐藏？
4. **Path 列命名**：继续 `ws_path`，还是迁移为中性名（`path` / `http_path`）？
5. **VMess + XHTTP**：订阅是否照常下发，还是 XHTTP 节点仅允许 VLESS？
6. **不兼容客户端策略**：订阅里混合 WS/XHTTP 节点 vs 分订阅 profile vs 过滤旧 UA（我们无 UA，实际只能混合节点）？
7. **WS 退役时间表**：XHTTP 全量后是否删除 `ws` 代码路径与文档，还是长期双支持？
8. **ALPN**：强制订阅写 `h2`，还是留空让客户端默认（可能踩 H3）？
9. **验收矩阵**：除 Clash Verge / v2rayNG 外，是否必须 v2rayA、iOS 某客户端通过才算里程碑完成？

---

## 附录 A — 与现网对照速查

| 层 | 现网（WS） | 本设计（XHTTP） |
| :--- | :--- | :--- |
| CF DNS / 橙云 | 有 | 不变 |
| Tunnel hostname → `http://127.0.0.1:port` | 有 | 不变 |
| Xray listen | `127.0.0.1` | 不变 |
| `streamSettings.network` | `ws` | `xhttp` |
| 客户端 port | 443 | 443 |
| 出口 IP | VPS | VPS |
| daemon HMAC / traffic | 有 | 不变 |
| `nodes.port` 含义 | 本机端口 | 本机端口 |

## 附录 B — 参考

- 现网决策与 Tunnel 模型：[cloudflare_migration_plan.md](../cloudflare_migration_plan.md) §1–3、§5.3  
- 配置生成：`workers/api/src/lib/nodeconfig.ts`  
- URI / Clash：`workers/api/src/lib/xrayuri.ts`、`subformats.ts`  
- 部署：`deploy/node-cf-ws/deploy-node-cf-ws.sh`  
- Xray transport 文档：Project X `xhttpSettings`；社区 CF 实践：packet-up 优先，stream-up 慎用  
- mihomo：`network: xhttp` + `xhttp-opts`
