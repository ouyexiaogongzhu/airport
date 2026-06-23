#!/usr/bin/env bash
# RFPlay Airport — Docker Entrypoint & SSL Setup
# ================================================
# Usage:
#   ./deploy/docker/ssl-setup.sh     # Get Let's Encrypt certs
#   ./deploy/docker/ssl-renew.sh     # Renew certificates
#
# Prerequisites:
#   - Domain DNS points to this server
#   - Port 80/443 open on firewall
#   - Docker Compose running

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_DIR"

case "${1:-help}" in
  ssl-setup)
    echo "=== RFPlay SSL Setup ==="
    echo "Getting Let's Encrypt certificates for rfplay.uk..."
    docker compose run --rm --profile ssl-setup certbot
    echo "Reloading nginx to pick up new certificates..."
    docker compose exec nginx nginx -s reload
    echo ""
    echo "✅ SSL setup complete!"
    echo "Configure auto-renewal with:"
    echo "  0 3 * * * cd $PROJECT_DIR && docker compose run --rm --profile ssl-setup certbot && docker compose exec nginx nginx -s reload"
    ;;

  ssl-renew)
    echo "=== RFPlay SSL Renewal ==="
    docker compose run --rm --profile ssl-setup certbot renew
    docker compose exec nginx nginx -s reload
    echo "✅ SSL renewal complete!"
    ;;

  build)
    echo "=== Building all Docker images ==="
    docker compose build --parallel
    echo "✅ Build complete!"
    ;;

  start)
    echo "=== Starting RFPlay services ==="
    docker compose up -d
    echo "✅ Services started!"
    echo "  Portal:  https://rfplay.uk"
    echo "  Admin:   https://rfplay.uk/admin"
    echo "  API:     https://api.rfplay.uk"
    echo "  Health:  https://rfplay.uk/health"
    ;;

  stop)
    echo "=== Stopping RFPlay services ==="
    docker compose down
    echo "✅ Services stopped!"
    ;;

  logs)
    shift || true
    docker compose logs -f "$@"
    ;;

  status)
    echo "=== RFPlay Service Status ==="
    docker compose ps
    echo ""
    echo "=== Recent logs ==="
    docker compose logs --tail=10
    ;;

  test)
    echo "=== RFPlay Integration Test ==="
    HOST="${2:-http://localhost:80}"
    echo "Testing $HOST..."
    
    # Health check
    echo -n "Health: "
    curl -sf "$HOST/health" && echo " ✅" || echo " ❌"
    
    # Login
    echo -n "Login: "
    TOKEN=$(curl -sf -X POST "$HOST/api/v1/public/login" \
      -H 'Content-Type: application/json' \
      -d '{"username":"ittest","password":"test123456"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
    if [ -n "$TOKEN" ]; then echo " ✅ (token acquired)"; else echo " ❌"; fi
    
    # Profile
    echo -n "Profile: "
    curl -sf "$HOST/api/v1/user/profile" -H "Authorization: Bearer $TOKEN" > /dev/null && echo " ✅" || echo " ❌"
    
    # Admin
    echo -n "Admin login: "
    ADMIN_TOKEN=$(curl -sf -X POST "$HOST/api/v1/public/login" \
      -H 'Content-Type: application/json' \
      -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin).get('token',''))" 2>/dev/null)
    if [ -n "$ADMIN_TOKEN" ]; then echo " ✅"; else echo " ❌"; fi
    
    echo ""
    echo "=== Portal SPA ==="
    echo -n "Portal index: "
    curl -sf "$HOST/" | grep -q "RFPlay" && echo " ✅" || echo " ❌"
    
    echo -n "Admin index: "
    curl -sf "$HOST/admin/" | grep -q "html" && echo " ✅" || echo " ❌"
    ;;

  *)
    echo "RFPlay Airport — Docker Deployment Manager"
    echo ""
    echo "Usage: ./deploy/docker/manage.sh <command> [args]"
    echo ""
    echo "Commands:"
    echo "  build         Build all Docker images"
    echo "  start         Start all services (docker compose up -d)"
    echo "  stop          Stop all services"
    echo "  restart       Restart all services"
    echo "  logs [svc]    Tail logs (optional: service name)"
    echo "  status        Show service status"
    echo "  ssl-setup     Get Let's Encrypt certificates (first time)"
    echo "  ssl-renew     Renew Let's Encrypt certificates"
    echo "  test [url]    Run integration tests (default: http://localhost:80)"
    echo ""
    ;;
esac
