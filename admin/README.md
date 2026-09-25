# RFPlay Admin (`xva.rfplay.uk`)

Vue 3 + Vite app deployed to Cloudflare Pages (`rfplay-admin`).

## Same-origin `/api` proxy

Production builds use `VITE_API_BASE_URL=/api/v1`. Pages Function
`functions/api/[[path]].ts` forwards `/api/*` to Worker `rfplay-api` via
Service Binding `API` (see `wrangler.toml`). Static assets stay outside
Functions (`public/_routes.json` includes only `/api/*`).

Subscription clipboard URLs still use the public API host
(`VITE_SUBSCRIPTION_BASE_URL=https://api.rfplay.uk`). Settings health
check calls `https://api.rfplay.uk/health` because `/health` is the
Worker root, not under `/api/*`.

### Local Pages Functions + binding

```bash
# terminal 1 — API Worker
cd workers/api && npx wrangler dev

# terminal 2 — Pages (after npm run build)
npx wrangler pages dev dist --service API=rfplay-api
```

Ordinary Vite dev (`npm run dev`) still proxies `/api/v1` to localhost via
`vite.config.ts` and does not need the Service Binding.
