# Google OAuth（Portal 登录）

Portal（`xv.rfplay.uk`）通过 Worker 走 Google OAuth 2.0 / OIDC **authorization code** 流程。  
**不是** Cloudflare Access OTP。

## 流程

1. 用户在 Login / Register 点 **Continue with Google**
2. 浏览器跳到 `GET https://api.rfplay.uk/api/v1/public/oauth/google/start`
3. Worker 写 `oauth_google_state` cookie，302 到 Google
4. Google 回调 `GET https://api.rfplay.uk/api/v1/public/oauth/google/callback`
5. Worker 用 `GOOGLE_CLIENT_ID` + `GOOGLE_CLIENT_SECRET` 换 token，读 userinfo（需 `email_verified`）
6. 按 `google_sub` find-or-create（已有同 email 则绑定 `google_sub`），签发与密码登录相同的 `session` / `refresh` / `csrf` cookies（`Domain=rfplay.uk`）
7. 302 到 `PORTAL_URL/dashboard`（默认 `https://xv.rfplay.uk/dashboard`）

密码登录仍走 Turnstile；OAuth 回调**不**校验 Turnstile。

## API

| Method | Path | 说明 |
|--------|------|------|
| GET | `/api/v1/public/oauth/google/start` | 开始 OAuth |
| GET | `/api/v1/public/oauth/google/callback` | Google 回跳 |
| POST | `/api/v1/public/login` | `username` 可为 **邮箱或用户名** |
| POST | `/api/v1/public/register` | 可选/推荐字段 `email`（唯一，大小写不敏感） |

迁移：`workers/api/migrations/0006_google_oauth.sql`（`users.google_sub` UNIQUE）。

## Google Cloud Console（需手动）

1. [Google Cloud Console](https://console.cloud.google.com/) → APIs & Services → **Credentials**
2. 创建 **OAuth client ID**，类型选 **Web application**
3. **Authorized JavaScript origins**（可选，本实现以 redirect 为主）:
   - `https://xv.rfplay.uk`
   - `http://localhost:5173`（本地）
4. **Authorized redirect URIs**（必填）:
   - `https://api.rfplay.uk/api/v1/public/oauth/google/callback`
   - 本地：`http://127.0.0.1:8787/api/v1/public/oauth/google/callback`（若用 `wrangler dev`）
5. 复制 **Client ID** / **Client Secret**

OAuth consent screen：选 External（或 Internal），至少加 scopes：`openid`、`email`、`profile`。测试阶段把测试账号加到 Test users。

## Worker Secrets / Vars

```bash
# secrets（从仓库根 .env）
cd workers/api
../../deploy/cloudflare/push-secrets.sh ../../.env
# 或单独：
printf '%s' 'YOUR_CLIENT_ID'     | npx wrangler secret put GOOGLE_CLIENT_ID
printf '%s' 'YOUR_CLIENT_SECRET' | npx wrangler secret put GOOGLE_CLIENT_SECRET

# 迁移
npx wrangler d1 migrations apply rfplay --remote

# PORTAL_URL 已在 wrangler.jsonc vars（OAuth 成功后回跳）
# 如需覆写回调 URL：
# npx wrangler secret put GOOGLE_REDIRECT_URI
# 值：https://api.rfplay.uk/api/v1/public/oauth/google/callback
```

根目录 `.env.example` 已加：

```
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

## Portal 环境变量

生产 Pages（`deploy-pages.yml`）注入：

```
VITE_GOOGLE_CLIENT_ID=<与 Worker 相同的 Client ID>
VITE_API_BASE_URL=/api/v1
VITE_SUBSCRIPTION_BASE_URL=https://api.rfplay.uk
```

普通 API 走同站 `/api/v1`（Pages Function → Worker）。**OAuth start / callback 仍绝对**在 `https://api.rfplay.uk/api/v1/public/oauth/google/...`：`portal/src/utils/googleAuth.ts` 在 `VITE_API_BASE_URL` 为相对路径时强制使用公网 API 主机（已注册的 redirect URI）。本地若把 `VITE_API_BASE_URL` 设成绝对地址（例如 `http://127.0.0.1:8787/api/v1`），OAuth 跟随该主机。

`VITE_GOOGLE_CLIENT_ID` 仅控制是否显示 Google 按钮；真正换码在 Worker。

## 邮箱登录

- 登录框：**Email or username** → 后端先按 username，再按 email（`COLLATE NOCASE`）查找
- 注册：必填 email + username + password；email 存小写并 UNIQUE
