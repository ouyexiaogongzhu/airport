# RFPlay Airport — Health Monitoring Dashboard

## Overview

A standalone HTML page that monitors the health of all RFPlay Airport services
and displays real-time status with auto-refresh.

**Monitored services:**
| Service | Endpoint | Type |
|---------|----------|------|
| Manager API | `http://localhost:8080/health` | HTTP JSON |
| Nginx Proxy | `http://localhost:80/health` | HTTP JSON (proxied to manager) |
| Xray VMess/VLESS | `http://localhost:8443` | TCP port check |
| Xray SOCKS5 | `http://localhost:1099` | TCP port check |
| Node Daemon | `http://localhost:9090/health` | HTTP JSON |

## Usage

### Direct file (browser)

```bash
open deploy/health/health.html
```

Or open the file in any modern browser (Chrome, Firefox, Edge).
> **Note:** CORS restrictions may block cross-origin requests when opened as
> `file://`. For full functionality, serve via a local HTTP server.

### Serve via Python (recommended)

```bash
# From project root
cd deploy/health
python3 -m http.server 8081
```

Then open `http://localhost:8081/health.html`

### Serve via the existing nginx

Add this location to `deploy/docker/nginx.conf.dev` (or nginx.conf for production):

```nginx
location /health/dashboard {
    alias /path/to/deploy/health;
    index health.html;
}
```

Or copy the file to the portal's static directory.

## Features

- **Real-time status**: All services checked every 30 seconds
- **Color-coded indicators**:
  - 🟢 **Green** = Healthy (responding normally)
  - 🟡 **Yellow** = Degraded (was up, now unreachable)
  - 🔴 **Red** = Unhealthy (consistently unreachable)
- **Summary cards**: Total, healthy, degraded, and unhealthy counts
- **Uptime tracking**: Shows how long each service has been healthy
- **Latency display**: Response time for HTTP checks
- **Countdown timer**: Shows seconds until next check
- **Last-check timestamp**: When the most recent check completed
- **Responsive**: Works on desktop and mobile

## Service Status Logic

### HTTP checks (Manager, Nginx, Daemon)
- **Healthy**: HTTP 200 + valid JSON response
- **Degraded**: Previously healthy, now failing
- **Unhealthy**: Consistently failing (no prior success)

### TCP checks (Xray ports)
- **Healthy**: Port is open and accepting connections
- **Degraded/Unhealthy**: Port is closed or connection refused

## Customization

To add or modify services, edit the `SERVICES` array at the top of
`health.html`:

```javascript
{ id: 'my-service', name: 'My Service', endpoint: '/health', url: 'http://localhost:3000', tag: 'Custom', port: 3000 }
```

For TCP-only checks (non-HTTP):
```javascript
{ id: 'my-tcp', name: 'My TCP', endpoint: '', url: 'http://localhost:1234', tag: 'TCP', port: 1234, check: 'tcp' }
```

## Troubleshooting

| Symptom | Likely Cause | Solution |
|---------|-------------|----------|
| All services show red | Services not running | `docker compose ps` to check |
| Cross-origin errors | CORS when opened as `file://` | Serve via HTTP (see above) |
| Xray shows red | Xray profile not enabled | Use `docker compose --profile full up -d` |
| No update | Browser tab throttling | Make sure tab is active |
