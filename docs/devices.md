# Device limits (subscription-side slots)

## Chosen model

A **device** is a fingerprint bound when a client successfully pulls the subscription URL
(`/api/v1/client/links/:token` and `/clash` / `/singbox` variants).

| Candidate | Used? | Why |
| :--- | :--- | :--- |
| Subscription pull + fingerprint | **Yes** | Enforceable in Workers/D1 today |
| Portal JWT session | No | Login sessions ≠ VPN clients; TTL unchanged |
| Live Xray connection | No | CF Tunnel hides real client IPs; would need daemon/online registry |

## Fingerprint resolution (priority)

1. Header `X-Device-Id` or `X-Device-Fingerprint` (8–64 `[A-Za-z0-9_-]`)
2. Query `device_id` or `dfp`
3. Fallback: `SHA256("ua:" + User-Agent)[:32]` — **weak**: identical clients share one slot

## Limit

- Constant: `DEFAULT_MAX_DEVICES = 5` in `workers/api/src/lib/devices.ts` (easy to change).
- Per-user: `users.max_devices` (`0` = unlimited).
- Products: optional `products.max_devices`; when set, copied onto the user on grant/pay.

## Enforcement

- New fingerprint at/over limit → `403` `{ "error": "DEVICE_LIMIT_EXCEEDED", ... }`.
- Known fingerprint → update `last_seen`, serve subscription.
- `GET /api/v1/client/devices` / `DELETE /api/v1/client/devices/:id` for list/revoke.
- Regenerating `client_token` clears that user’s device rows (URL changed; re-import required).

## Limitation

This caps **subscription importers**, not simultaneous Xray TCP sessions. Two phones using the
same Clash User-Agent without an explicit device id share one slot. True online-connection
limiting needs daemon-side presence tracking (out of scope for this version).
