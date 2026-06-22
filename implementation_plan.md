# Implementation Plan: Airport Proxy System (PoC & MVP)

---

## 1. Goal Description

* **Manager API** (`api.rfplay.uk`): Go Fiber `:443` + Origin PEM, CORS, webhooks, online verify.
* **User Portal** (`www.rfplay.uk`): Vue 3 CF Pages — cookie + CSRF auth, register, pay.
* **Admin Dashboard** (`admin.rfplay.uk`): Vue 3 CF Pages — cookie + CSRF, token 发放, ops.
* **Daemon + Xray-core**: online verify, sync traffic, CF-WS Nginx decoy.
* **Flutter**: Bearer JWT, Token 导入, VPN.

---

## 2. Decided Specifications

| Item | Decision |
| :--- | :--- |
| **User Portal** | CF Pages → `www.rfplay.uk` |
| **Admin** | CF Pages → `admin.rfplay.uk` |
| **Manager** | `api.rfplay.uk`; Go `:443` + Origin PEM; CF IP firewall |
| **Portal/Admin auth** | httpOnly `session`/`refresh` + CSRF；**no** localStorage JWT |
| **Flutter auth** | `X-Client: flutter` → JSON Bearer; secure storage |
| **CORS** | `AllowCredentials: true`; whitelist `www` + `admin` |
| **Node auth** | `POST /api/node/verify-token` per connection; sync 无 user_list |
| **CF-WS** | Nginx decoy site + WS reverse proxy to Xray localhost |
| **Payment** | BEpusdt + Payoneer webhooks on `api.rfplay.uk` |
| **Email** | Resend/Brevo transactional; CF Email Routing inbound |
| **Tokens** | `rf_` portal / `at_` admin; `at_` immutable; renew = new token |
| **`max_devices`** | `0` = unlimited |
| **Traffic analytics** | Primary: node sync → DB; CF: reconciliation only |

---

## 3. Repository Structure

```
airport-system/
├── manager/
├── portal/
├── admin/
├── daemon/
├── client/
└── xray-core/
```

---

## 4. Manager (Phase 3a)

* Remove `go:embed`
* CORS `AllowCredentials` + CSRF middleware
* Cookie session for `/api/web/*`; Bearer for `/api/client/*` + Flutter login
* `GET /api/auth/csrf`, `POST /api/auth/logout`, `POST /api/auth/refresh`
* Dual login: browser Set-Cookie vs `X-Client: flutter` JSON
* `POST /api/node/verify-token`; slim `/api/node/sync` (traffic + config only)
* `issued_tokens`: batch, immutable, `/renew` (revoke + new `at_`)
* Payment callbacks; Resend email on register
* Env: `PORTAL_ORIGIN`, `ADMIN_ORIGIN`, `TLS_CERT_FILE`, `RESEND_API_KEY`

---

## 5. Cloudflare Pages

### Portal (`portal/`)
* Build: `npm ci && npm run build` → `dist`
* Env: `VITE_API_BASE_URL=https://api.rfplay.uk`
* Auth: `withCredentials` + `X-CSRF-Token`; mount时 `GET /api/auth/csrf`
* Domain: `www.rfplay.uk`

### Admin (`admin/`)
* Same pattern; cookies: `admin_session`, `admin_csrf`
* Domain: `admin.rfplay.uk`

### API (`api.rfplay.uk`)
* CF proxied → Manager `:443`
* Full (Strict) + Origin CA

---

## 6. Execution Plan

| Phase | Deliverable |
| :--- | :--- |
| **1** | Clone Xray-core |
| **2** | Xray: verify via Daemon, rate limit, P2P audit, logs |
| **3a** | Manager API |
| **3b** | Portal Vue → `www` |
| **3c** | Admin Vue → `admin` |
| **4** | Daemon + CF-WS Nginx + REALITY nodes |
| **5** | Flutter client |
| **6** | E2E: pay → connect → multi-device verify |

---

## 7. Verification

| Test | Expected |
| :--- | :--- |
| `www` login | Set-Cookie; body `{ user }` only; no localStorage JWT |
| `www` POST order | CSRF required; cookie auth |
| `admin` login | `admin_session` cookie; CSRF on mutations |
| Flutter login | `X-Client: flutter` → Bearer in body |
| Node connect | verify-token to Manager; no local HMAC secret |
| Node compromised | No user_list leak from sync |
| `at_` renew | Old token revoked; new token required in Flutter |
| Webhook | BEpusdt callback on `api.rfplay.uk` |
| Wrong CORS origin | Blocked |
