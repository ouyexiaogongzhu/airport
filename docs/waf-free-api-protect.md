# Free-plan WAF：保护登录（dashboard / API）

> 适用：Cloudflare **Free** 区 `rfplay.uk`。  
> 目标：压登录爆破 / 空 UA 脚本，**不**误伤订阅拉取与一般 API。  
> Free 额度提醒：**1** 条 Rate Limiting 规则；**5** 条 Custom rules（本 runbook 最多用掉 1 RL + 1 optional custom）。

## 已通过 API 部署（2026-09-25）

下列 ruleset 已写入生产区（可在 Dashboard → Security → Security rules 核对）：

| Phase | Ruleset | 规则 |
| :--- | :--- | :--- |
| `http_ratelimit` | `RFPlay login rate limit` | POST login/register/admin-login，5/10s，`cf.colo.id`+`ip.src`，Block 10s |
| `http_request_firewall_custom` | `RFPlay custom WAF` | 同上路径且空 UA → Block |

Bot Fight Mode：保持 **Off**。Turnstile：应用层 **保持 ON**。

以下为手工复现步骤（若需重建）。

同站 proxy 之后，浏览器对 login/register 的请求落在 **xv / xva**（以及仍直连时的 **api**）。规则的 **Hostname 必须包含三者**，只配 `api.rfplay.uk` 不够。

| Host | 角色 |
| :--- | :--- |
| `api.rfplay.uk` | Worker 直连 / 旧路径 |
| `xv.rfplay.uk` | Portal（same-origin proxy → API） |
| `xva.rfplay.uk` | Admin（same-origin proxy → API） |

应用层 **Turnstile 保持开启**（login/register）；本文件只补边缘限速与可选 UA 规则。

---

## 1. Bot Fight Mode：对 api / xv / xva 关掉

Bot Fight Mode 会挑战 / 干扰 API 与 SPA 流量，与 Turnstile + cookie 会话不搭。

1. Cloudflare Dashboard → 选中区 **rfplay.uk**
2. **Security** → **Bots**（或 **Bot Fight Mode**）
3. 确认 **Bot Fight Mode = Off**（至少不要对生产 `api` / `xv` / `xva` 开着）
4. 若账号只有区级开关：整区 Off；不要依赖 BFM 挡登录

---

## 2. 唯一一条 Rate Limiting 规则（IP，Block）

Free 只有 **1** 条 RL，全部预算用在登录路径上。**不要**对整站或全部 `/api` 做 RL。

1. **Security** → **WAF** → **Rate limiting rules** → **Create rule**
2. **Rule name**：`login-register-rl`（任意清晰名称）
3. **If incoming requests match…**（按 Free UI 字段填；表达式示意）：

   - **Hostname** 在  
     `api.rfplay.uk` **或** `xv.rfplay.uk` **或** `xva.rfplay.uk`
   - **URI Path** 包含下列之一：  
     `/public/login` **或** `/public/register` **或** `/admin/auth/login`
   - **HTTP Method** = `POST`

   示意 expression（若 UI 提供 Edit expression）：

   ```text
   (http.host in {"api.rfplay.uk" "xv.rfplay.uk" "xva.rfplay.uk"}
    and http.request.method eq "POST"
    and (
      http.request.uri.path contains "/public/login"
      or http.request.uri.path contains "/public/register"
      or http.request.uri.path contains "/admin/auth/login"
    ))
   ```

4. **With the same characteristics** / counting：
   - Characteristics：**IP**
   - Requests：约 **5** / **10 seconds**（Free UI 若无精确「5/10s」，选最接近的阈值与窗口）
5. **Then take action**：
   - Action：**Block**
   - Duration / timeout：约 **10 seconds**（按 Free UI 可选项调整）
6. Deploy / Save

**明确不要做**：对 `/*`、全部 `/api`、订阅 `/clash` `/links` 等做 rate limit。

---

## 3.（可选）Custom rule：仅登录路径空 User-Agent → Block

占用 1/5 条 Free custom。**不要**全局「空 UA → Block」（会误伤健康检查 / 部分客户端）。

1. **Security** → **WAF** → **Custom rules** → **Create rule**
2. **Rule name**：`login-empty-ua`
3. Match：与上面 **相同 hosts + 相同三条 path + POST**，且  
   `http.user_agent eq ""`（或 UI：**User Agent** is empty）
4. Action：**Block**
5. Deploy

示意 expression：

```text
(http.host in {"api.rfplay.uk" "xv.rfplay.uk" "xva.rfplay.uk"}
 and http.request.method eq "POST"
 and http.user_agent eq ""
 and (
   http.request.uri.path contains "/public/login"
   or http.request.uri.path contains "/public/register"
   or http.request.uri.path contains "/admin/auth/login"
 ))
```

---

## 4. 核对清单

| 项 | 期望 |
| :--- | :--- |
| Bot Fight Mode | Off（至少不挡 api/xv/xva） |
| Rate limiting | **仅 1 条**；hosts = api+xv+xva；paths = login/register/admin login；POST；~5/10s Block ~10s |
| Custom（可选） | 仅上述登录路径 + 空 UA → Block；非全局空 UA |
| 未对全部 `/api` RL | 是 |
| Turnstile | login/register 仍开启（应用层，非本规则替代） |
| Same-origin proxy | RL/custom **已含 xv、xva** |

改完后用真实浏览器走一遍 portal / admin 登录；Security Events 里应只看到异常 IP 的命中，正常登录不被 Block。
