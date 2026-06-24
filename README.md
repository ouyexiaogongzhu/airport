# RFPlay Airport System

A modern, secure proxy service platform ("Airport") designed for **rfplay.uk**, featuring a Go Fiber API backend, dual Vue 3 frontends (User Portal + Admin Dashboard), a Flutter mobile client, and pull-based node daemon protocol with Xray-core integration.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Cloudflare Pages                         │
│  ┌──────────────┐  ┌──────────────┐                        │
│  │  User Portal  │  │    Admin     │  Vue 3 + Vite          │
│  │  www.rfplay.uk│  │ admin.rfplay │  httpOnly cookie + CSRF│
│  └──────┬───────┘  └──────┬───────┘                        │
└─────────┼─────────────────┼────────────────────────────────┘
          │                 │
          │   https://api.rfplay.uk (CF Proxy)
          ▼                 ▼
┌─────────────────────────────────────────────────────────────┐
│                     VPS / Docker Host                        │
│  ┌──────────────┐                                           │
│  │   Nginx      │  Reverse proxy, SPA serving, WS upgrade   │
│  │   :80/:443   │  Health: /health → manager:8080           │
│  └──────┬───────┘                                           │
│         │                                                    │
│  ┌──────▼───────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │   Manager    │  │   Daemon     │  │   Xray-core      │  │
│  │  Go/Fiber API│  │  Node agent  │  │  (modified fork)  │  │
│  │  :8080       │  │  verify/sync │  │  online verify   │  │
│  │  SQLite/Post │  │  + Loki logs │  │  WS/REALITY      │  │
│  └──────┬───────┘  └──────────────┘  └──────────────────┘  │
│         │                                                    │
│  ┌──────▼───────┐                                           │
│  │   Flutter    │  VPN Client (Android/iOS)                  │
│  │  (MVP+)      │  JWT Bearer + Token import (rf_/at_)      │
│  └──────────────┘                                           │
└─────────────────────────────────────────────────────────────┘
```

### Core Components

| Component | Language | Role | Port |
| :--- | :--- | :--- | :--- |
| **Manager** | Go / Fiber | API backend — auth, orders, nodes, traffic, subscriptions | `:8080` |
| **Nginx** | C | Reverse proxy, SPA serving, WebSocket upgrade | `:80` / `:443` |
| **Daemon** | Go | Node-side agent — verify tokens, sync config, Loki logs | `:9090` |
| **Xray-core** | Go (fork) | Modified XTLS/Xray-core with online verify-per-connection | `:443` / `:8443` / `:1099` |
| **Portal** | Vue 3 + Vite | User-facing website (www.rfplay.uk) | CF Pages |
| **Admin** | Vue 3 + Vite | Admin dashboard (admin.rfplay.uk) | CF Pages |
| **Flutter** | Dart | Mobile VPN client (Android/iOS) | App stores |

### Data Flow (Connection)

```
User → 3rd-party client (Shadowrocket/V2rayNG/Clash)
  → Node (Xray-core with X-Real-IP + xray_token header)
  → POST /api/v1/node/verify-token → Manager validates user + subscription
  → Allow/Deny → Traffic accounting
```

---

## Quick Start

### Prerequisites

- Docker & Docker Compose v2
- Go 1.25+ (for local development)
- Node.js 22+ (for frontend)
- Flutter 3.29+ (for mobile client)

### Start All Services (Docker)

```bash
# Clone and enter the project
git clone <repo-url> airport-system
cd airport-system

# Copy environment (edit as needed)
cp .env.example .env

# Start core services (Manager API + Nginx)
docker compose up -d manager nginx

# Verify health
curl http://localhost/health
# {"status":"ok","version":"0.0.1"}

# Start all services (including daemon + xray)
docker compose --profile full up -d
```

### Development (without Docker)

```bash
# Start Manager API
PORT=8080 DATA_DIR=./data JWT_SECRET=dev-secret \
  go run ./manager/cmd/server/

# Start Portal (separate terminal)
cd portal && npm ci && npm run dev

# Start Admin (separate terminal)
cd admin && npm ci && npm run dev
```

### Build Docker Images

```bash
./deploy/docker/manage.sh build
./deploy/docker/manage.sh start
./deploy/docker/manage.sh status
./deploy/docker/manage.sh test
```

---

## Development Workflow

### Parallel Agent System

This project uses a multi-agent development workflow. Each component can be developed independently by separate agents:

```
Main orchestrator (ubuntu_game_bot)
├── backend_bot → Manager API (Go)
├── ui_bot      → Portal (Vue 3)
├── admin_bot   → Admin Dashboard (Vue 3)
├── flutter_bot → Flutter Client
├── daemon_bot  → Node Daemon
└── xray_bot    → Xray-core fork
```

### Repository Layout

```
airport-system/
├── manager/              # Go Fiber API backend
│   ├── cmd/server/       # Entry point
│   ├── internal/
│   │   ├── handler/      # HTTP handlers
│   │   ├── middleware/    # JWT, CSRF, rate-limit
│   │   ├── model/        # Data models
│   │   └── store/        # Data access (SQLite)
│   └── go.mod
├── portal/               # Vue 3 User Portal
│   ├── src/
│   │   ├── components/
│   │   ├── views/
│   │   └── test/
│   └── package.json
├── admin/                # Vue 3 Admin Dashboard
│   ├── src/
│   │   ├── components/
│   │   ├── views/
│   │   └── test/
│   └── package.json
├── client/               # Flutter mobile client
│   ├── lib/
│   ├── test/
│   └── pubspec.yaml
├── daemon/               # Node daemon (Go)
├── xray-core/            # XTLS/Xray-core fork
├── deploy/
│   ├── ci-verify.sh      # Local CI verification (run before push)
│   ├── test/
│   │   ├── e2e_test.py   # Full E2E test suite
│   │   ├── api_test.py   # API integration tests
│   │   └── api_test.sh   # Shell-based API tests
│   ├── docker/           # Dockerfiles, nginx configs, scripts
│   ├── certs/            # Dev SSL certificates
│   └── nginx/            # Production nginx configs
├── docker-compose.yml    # Production Docker Compose
├── Dockerfile.manager    # Manager Dockerfile
├── Dockerfile.daemon     # Daemon Dockerfile
├── Dockerfile.xray       # Xray-core Dockerfile
├── PLAN.md               # Current delivery plan (Phases 0-5)
├── airport_system_design.md  # Full architecture & API spec
└── .env                  # Environment variables
```

### Branch Strategy

| Branch | Purpose |
| :--- | :--- |
| `main` | Production — CI runs on push/PR |
| `feat/*` | Feature branches |
| `fix/*` | Bug fixes |
| `chore/*` | Maintenance, CI, docs |

---

## Running Tests

### 1. Local CI Verification (recommended before push)

```bash
# Run ALL checks (Go tests, Vue build, Flutter, E2E)
./deploy/ci-verify.sh

# Skip Docker-dependent E2E test
./deploy/ci-verify.sh --skip-e2e

# Skip frontend (Vue + Flutter) checks
./deploy/ci-verify.sh --skip-frontend

# Quick mode — Go tests only
./deploy/ci-verify.sh --quick
```

### 2. Go Unit Tests (Manager)

```bash
cd manager
go test -v -count=1 -timeout 30s ./...
```

Test files:
- `manager/internal/handler/auth_test.go`
- `manager/internal/handler/product_test.go`
- `manager/internal/handler/node_test.go`
- `manager/internal/handler/traffic_test.go`
- `manager/internal/handler/payment_test.go`
- `manager/internal/middleware/auth_test.go`

### 3. Vue 3 Frontend Tests (Portal + Admin)

```bash
# Portal
cd portal
npm ci                     # Install deps (first time)
npx vue-tsc --noEmit       # TypeScript check
npm run build              # Production build

# Admin
cd admin
npm ci                     # Install deps (first time)
npx vue-tsc --noEmit       # TypeScript check
npm run build              # Production build
```

### 4. Flutter Tests (Client)

```bash
cd client
flutter pub get            # Install deps (first time)
flutter analyze            # Static analysis
flutter test               # Unit/widget tests
```

### 5. E2E API Tests (requires Docker)

```bash
# Start services
docker compose up -d manager nginx
sleep 5                    # Wait for health check

# Full E2E test (6 scenarios, ~20+ checks)
python3 deploy/test/e2e_test.py

# API integration test (17 steps)
python3 deploy/test/api_test.py

# Cleanup
docker compose down
```

### What the E2E Test Covers

| Scenario | Steps |
| :--- | :--- |
| 1. User Full Flow | Health → Captcha → Login → Browse → Order → Pay → Subscribe → Profile |
| 2. Admin CRUD | Login → Products CRUD → Nodes CRUD → Users → Tokens → Traffic |
| 3. Auth Guards | Unauthorized → 401, Non-admin → 403, Public endpoints |
| 4. Token Login | Get client_token → Token login → Invalid token rejection |
| 5. Nginx Proxy | Health, captcha, products via reverse proxy |
| 6. Edge Cases | Empty login → 400, No-auth order → 401, Rate-limited paths |

---

## Environment Variables Reference

### Manager API

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | HTTP listen port |
| `DATA_DIR` | `./data` | SQLite database & data directory |
| `JWT_SECRET` | `dev-secret` | JWT signing secret (change in production!) |
| `CORS_ORIGINS` | `https://rfplay.uk,https://admin.rfplay.uk` | Allowed CORS origins |

### Daemon (Node Agent)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `DAEMON_MANAGER_URL` | `http://manager:8080` | Manager API base URL |
| `DAEMON_LISTEN_ADDR` | `:9090` | Daemon HTTP listen address |
| `DAEMON_NODE_ID` | `1` | Node ID for this daemon instance |
| `DAEMON_MANAGER_TOKEN` | (empty) | Auth token for manager API calls |

### Frontend (Portal / Admin)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `VITE_API_BASE_URL` | `/api/v1` | API base path (set to `https://api.rfplay.uk` on CF Pages) |

---

## API Quick Reference

All API endpoints are proxied through Nginx at `https://api.rfplay.uk`.

### Health & Public

| Method | Path | Auth | Description |
| :--- | :--- | :--- | :--- |
| GET | `/health` | No | Health check → `{"status":"ok"}` |
| GET | `/api/v1/captcha` | No | Get captcha challenge |
| POST | `/api/v1/public/register` | No | Register user (with captcha) |
| POST | `/api/v1/public/login` | No | Login → JWT token |
| POST | `/api/v1/public/token-login` | No | Login via client_token (for Flutter) |
| POST | `/api/v1/public/payment/callback` | No | Payment webhook callback |

### User Endpoints (JWT required)

| Method | Path | Description |
| :--- | :--- | :--- |
| GET | `/api/v1/products` | List active products (public) |
| GET | `/api/v1/user/profile` | Get user profile |
| PUT | `/api/v1/user/profile` | Update user profile |
| POST | `/api/v1/user/orders` | Create order |
| GET | `/api/v1/user/orders` | List user orders |
| GET | `/api/v1/subscription/:token` | Get subscription config (V2Ray base64) |
| GET | `/api/v1/subscription/:token/clash` | Get subscription config (Clash format) |
| GET | `/api/v1/web/client-token` | Get client token for Flutter |

### Admin Endpoints (JWT + admin role required)

| Method | Path | Description |
| :--- | :--- | :--- |
| GET | `/api/v1/admin/products` | List all products |
| POST | `/api/v1/admin/products` | Create product |
| PUT | `/api/v1/admin/products/:id` | Update product |
| DELETE | `/api/v1/admin/products/:id` | Archive product |
| GET | `/api/v1/admin/nodes` | List all nodes |
| POST | `/api/v1/admin/nodes` | Create node |
| GET | `/api/v1/admin/nodes/:id` | Get node detail |
| PUT | `/api/v1/admin/nodes/:id` | Update node |
| DELETE | `/api/v1/admin/nodes/:id` | Delete node |
| GET | `/api/v1/admin/users` | List users |
| GET | `/api/v1/admin/users/:id` | Get user detail |
| PUT | `/api/v1/admin/users/:id` | Update user / regenerate token |
| POST | `/api/v1/admin/traffic/report` | Report traffic usage |
| GET | `/api/v1/admin/traffic/stats` | Get traffic statistics |

### Node Endpoints (Node-to-Manager)

| Method | Path | Auth | Description |
| :--- | :--- | :--- | :--- |
| POST | `/api/v1/node/verify-token` | xray_token | Verify user token at connection time |
| GET | `/api/v1/node/sync` | Node token | Sync node configuration |

### Response Format

```json
{
  "data": { ... },
  "error": "optional error message"
}
```

Error codes: `400` (validation), `401` (unauthorized), `403` (forbidden), `404` (not found), `409` (conflict), `429` (rate limited), `500` (server error).

---

## Deployment

### Production Docker Compose

```bash
# Start with full profile (includes daemon + xray)
docker compose --profile full up -d

# SSL certificate setup (first time)
docker compose run --rm --profile ssl-setup certbot

# View logs
docker compose logs -f manager
docker compose logs -f nginx
```

### Cloudflare Pages (Portal + Admin)

Both frontends deploy to Cloudflare Pages. Environment variable `VITE_API_BASE_URL=https://api.rfplay.uk` must be set in the CF Pages dashboard.

### DNS Records

| Record | Type | Target |
| :--- | :--- | :--- |
| `www` | CNAME | CF Pages (portal) |
| `admin` | CNAME | CF Pages (admin) |
| `api` | A / CNAME | Manager IP (proxied) |
| `node-*` | A | Node IPs (proxied, CF-WS) |

---

## Key Security Decisions

| Area | Decision |
| :--- | :--- |
| **Portal/Admin login** | httpOnly cookie + CSRF; **no** localStorage JWT |
| **Flutter login** | JWT Bearer + `X-Client: flutter` + secure storage |
| **Token import** | `rf_` (portal user) / `at_` (admin-issued, no registration) |
| **`at_` rules** | Immutable; renew = invalidate + issue new; `max_devices=0` = unlimited |
| **Node auth** | `POST /api/v1/node/verify-token` on connect; no user_list in sync |
| **CF-WS nodes** | Nginx masquerade + WS proxy; Origin PEM; CF IP firewall |
| **REALITY nodes** | Direct connection; same online verify flow |
| **Payments** | BEpusdt + Payoneer; webhook → `api.rfplay.uk` |
| **Email** | Resend/Brevo outbound; CF Email Routing inbound → Gmail |

---

## Docs

| Document | Description |
| :--- | :--- |
| [PLAN.md](PLAN.md) | Current delivery plan (Phase 0-5, MVP priority) — **must read** |
| [airport_system_design.md](airport_system_design.md) | Full architecture, database schema, API contracts |
| [implementation_plan.md](implementation_plan.md) | (Archived) Old sprint plan |
| [task.md](task.md) | (Archived) Old task checklist |
| [deploy/ci-verify.sh](deploy/ci-verify.sh) | Local CI verification script |

---

## License

Proprietary — RFPlay Airport System. All rights reserved.
