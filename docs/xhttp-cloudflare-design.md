# XHTTP + TLS（Cloudflare Tunnel）

> **状态**：已实现。产品仅支持 **VLESS/VMess + XHTTP + TLS**（WS 已下线）。  
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
| 本机 | `listen=127.0.0.1`，`security=none`，path = `ws_path`（空则 `/rfhttp/`） |
| Tunnel | `hostname → http://127.0.0.1:<port>` |
| Mode | 服务端省略（auto）；订阅固定 **packet-up** |
| ALPN | 订阅写 **h2**（避免 H3） |
| Xray | ≥ `v26.3.27`（部署脚本 pin） |

`nodes.network`：仅 **`xhttp`**（admin 拒绝 `ws`；读配置时存量值一律 coerce）。path 列名仍为 `ws_path`。改 `ws_path` 后 version 变，gateway 下一周期重载（`SCHEMA=a4-xhttp-only-2`）。

**日志**：Xray `loglevel=error`，`error` → `/var/log/xray/error.log`，`access=none`。gateway 走 journald：`journalctl -u rfplay-gateway -p warning -f`。

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

代码：`nodeconfig.ts`、`xrayuri.ts`、`subformats.ts`；admin Transport = `xhttp` only；新建节点默认 `protocol=vless`、`network=xhttp`、path 空则 `/rfhttp/`。

## 运维

| 操作 | 做法 |
| :--- | :--- |
| 新建 / 切到 XHTTP | 后台默认已是 xhttp；path 建议 `/rfhttp/` |
| 探针 | `https://<host>/rfhttp/` → HTTP **400** + `x-padding` 即 Xray XHTTP 存活 |

## 客户端

| 客户端 | 说明 |
| :--- | :--- |
| Clash Verge / mihomo | 主推；用 `/clash` |
| v2rayNG | Base64；需支持 XHTTP 的 Xray 内核 |
| v2rayA | 需 **xray-core**；≥2.2.7.5 认 `xhttpMode`。OpenWrt 官方 **2.2.7.3** 仅 minimal，经 CF 不可靠 → 用 mihomo 或换机 |
| 测速 TIMEOUT | v2rayA HTTP 测速硬限 8s，冷启动易假阳；**直接连接**验证 |

不做：本机 TLS / Reality；stream-up 作默认；出口隐藏；WS。
