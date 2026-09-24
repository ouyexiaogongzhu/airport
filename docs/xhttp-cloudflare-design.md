# XHTTP + TLS（Cloudflare Tunnel）

> **状态**：已实现。生产节点（Amsterdam / New York）均为 `network=xhttp`。  
> 总览见 [cloudflare_migration_plan.md](../cloudflare_migration_plan.md)。

## 模型

```
客户端 → 域名:443 TLS（CF 终结，SNI=节点域名）
       → Tunnel → http://127.0.0.1:<nodes.port>
       → Xray（security=none, network=xhttp）
       → freedom（出口 IP = VPS）
```

| 项 | 取值 |
| :--- | :--- |
| 客户端 | `address`:443，`security=tls`，host/sni = 域名 |
| 本机 | `listen=127.0.0.1`，`security=none`，path = `ws_path` |
| Tunnel | `hostname → http://127.0.0.1:<port>`（与 WS 相同，无需改 ingress） |
| Mode | 服务端省略（auto）；订阅固定 **packet-up** |
| ALPN | 订阅写 **h2**（避免 H3） |
| Xray | ≥ `v26.3.27`（部署脚本 pin） |

`nodes.network`：`ws` \| `xhttp`（代码仍支持 WS；path 列名仍为 `ws_path`）。改 `network`/`ws_path` 后 version 变，daemon 下一周期重载。

## 订阅

**VLESS URI**（另发 `xhttpMode` 给 v2rayA ≥2.2.7.5）：

```
type=xhttp&mode=packet-up&xhttpMode=packet-up&alpn=h2&path=/rfhttp/&security=tls&sni=<host>&host=<host>
```

**Clash / mihomo**：

```yaml
network: xhttp
alpn: [h2]
xhttp-opts:
  path: "/rfhttp/"
  host: <host>
  mode: packet-up
```

代码：`nodeconfig.ts`（`SCHEMA=a3-xhttp-1`）、`xrayuri.ts`、`subformats.ts`；admin Transport = `ws`\|`xhttp`。

## 运维

| 操作 | 做法 |
| :--- | :--- |
| 切到 XHTTP | D1/后台：`network=xhttp`，path 建议 `/rfhttp/`；同 hostname 可切，旧 WS 客户端立即失效 |
| 探针 | `https://<host>/rfhttp/` → HTTP **400** + `x-padding` 即 Xray XHTTP 存活 |
| 回退 WS | `network=ws` + 原 path；客户端刷新订阅 |

## 客户端

| 客户端 | 说明 |
| :--- | :--- |
| Clash Verge / mihomo | 主推；用 `/clash` |
| v2rayNG | Base64；需支持 XHTTP 的 Xray 内核 |
| v2rayA | 需 **xray-core**；≥2.2.7.5 认 `xhttpMode`。OpenWrt 官方 **2.2.7.3** 仅 minimal，经 CF 不可靠 → 用 mihomo 或换机 |
| 测速 TIMEOUT | v2rayA HTTP 测速硬限 8s，冷启动易假阳；**直接连接**验证 |

不做：本机 TLS / Reality；stream-up 作默认；出口隐藏。
