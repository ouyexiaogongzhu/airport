#!/usr/bin/env bash
# ============================================================================
# RFPlay Airport — Local CI Verification Script
# ============================================================================
# Run this BEFORE pushing to ensure all checks pass locally.
# It mirrors exactly what the GitHub CI workflow runs, plus a full E2E test.
#
# Usage:
#   ./deploy/ci-verify.sh              # Run all checks (normal mode)
#   ./deploy/ci-verify.sh --skip-e2e   # Skip Docker + E2E test
#   ./deploy/ci-verify.sh --skip-frontend # Skip Vue / Flutter checks
#   ./deploy/ci-verify.sh --quick      # Skip frontend AND e2e (Go only)
#
# Exit code: 0 if all steps pass, 1 if any step fails.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

TOTAL_STEPS=0
PASSED=0
FAILED=0
ERRORS=()

# ── Flags ──────────────────────────────────────────────────────────────────
SKIP_E2E=false
SKIP_FRONTEND=false
for arg in "$@"; do
    case "$arg" in
        --skip-e2e)     SKIP_E2E=true ;;
        --skip-frontend) SKIP_FRONTEND=true ;;
        --quick)        SKIP_FRONTEND=true; SKIP_E2E=true ;;
    esac
done

# ── Helpers ─────────────────────────────────────────────────────────────────
green()  { echo -e "\033[32m$1\033[0m"; }
red()    { echo -e "\033[31m$1\033[0m"; }

run_step() {
    local step_name="$1"
    local label="$2"
    local workdir="$3"
    shift 3
    TOTAL_STEPS=$((TOTAL_STEPS + 1))

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  Step $TOTAL_STEPS: $step_name"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  $label"
    echo ""

    set +e
    output=$(
        cd "$PROJECT_DIR/$workdir" 2>/dev/null
        "$@" 2>&1
    )
    exit_code=$?
    set -euo pipefail

    if [ $exit_code -eq 0 ]; then
        PASSED=$((PASSED + 1))
        green "  ✅ PASS: $step_name"
    else
        FAILED=$((FAILED + 1))
        ERRORS+=("$step_name")
        red "  ❌ FAIL: $step_name"
        echo ""
        echo "  Last output lines:"
        echo "$output" | tail -20 | sed 's/^/    /'
    fi
}

check_cmd() {
    if ! command -v "$1" &>/dev/null; then
        echo "  ⚠️  Required command '$1' not found. Skipping."
        return 1
    fi
    return 0
}

# ── Banner ──────────────────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║      RFPlay Airport — Local CI Verification          ║"
echo "╠══════════════════════════════════════════════════════╣"
echo "║  Project: $PROJECT_DIR"
echo "║  Date:    $(date '+%Y-%m-%d %H:%M:%S')"
echo "╚══════════════════════════════════════════════════════╝"

# ============================================================================
# STEP 1: Go Unit Tests (Manager)
# ============================================================================
if check_cmd "go"; then
    run_step "Go Unit Tests" \
        "cd manager && go test -v -count=1 -timeout 30s ./internal/..." \
        "manager" \
        go test -v -count=1 -timeout 30s ./internal/...
else
    echo "  ⚠️  Go not installed — skipping Go tests"
fi

# ============================================================================
# STEP 2: Go Build (Manager)
# ============================================================================
if check_cmd "go"; then
    run_step "Go Build" \
        "cd manager && go build -v ./cmd/server/" \
        "manager" \
        go build -v ./cmd/server/
else
    echo "  ⚠️  Go not installed — skipping Go build"
fi

# ============================================================================
# STEP 3: Flutter Analyze + Test (Client)
# ============================================================================
if [ "$SKIP_FRONTEND" = false ] && check_cmd "flutter"; then
    run_step "Flutter Analyze" \
        "cd client && flutter analyze" \
        "client" \
        flutter analyze

    run_step "Flutter Test" \
        "cd client && flutter test" \
        "client" \
        flutter test
elif [ "$SKIP_FRONTEND" = true ]; then
    echo "  ⚠️  Skipping Flutter checks (--skip-frontend)"
else
    echo "  ⚠️  Flutter not installed — skipping Flutter checks"
fi

# ============================================================================
# STEP 4: Portal (Vue 3) — TypeScript check + Build
# ============================================================================
if [ "$SKIP_FRONTEND" = false ] && [ -d "$PROJECT_DIR/portal/node_modules" ]; then
    if check_cmd "npx"; then
        run_step "Portal TypeScript Check" \
            "cd portal && npx vue-tsc --noEmit" \
            "portal" \
            npx vue-tsc --noEmit

        run_step "Portal Build" \
            "cd portal && npm run build" \
            "portal" \
            npm run build
    else
        echo "  ⚠️  npx not available — skipping portal checks"
    fi
elif [ "$SKIP_FRONTEND" = true ]; then
    echo "  ⚠️  Skipping Portal checks (--skip-frontend)"
else
    echo "  ⚠️  Portal node_modules not found — run 'cd portal && npm ci' first"
fi

# ============================================================================
# STEP 5: Admin (Vue 3) — TypeScript check + Build
# ============================================================================
if [ "$SKIP_FRONTEND" = false ] && [ -d "$PROJECT_DIR/admin/node_modules" ]; then
    if check_cmd "npx"; then
        run_step "Admin TypeScript Check" \
            "cd admin && npx vue-tsc --noEmit" \
            "admin" \
            npx vue-tsc --noEmit

        run_step "Admin Build" \
            "cd admin && npm run build" \
            "admin" \
            npm run build
    else
        echo "  ⚠️  npx not available — skipping admin checks"
    fi
elif [ "$SKIP_FRONTEND" = true ]; then
    echo "  ⚠️  Skipping Admin checks (--skip-frontend)"
else
    echo "  ⚠️  Admin node_modules not found — run 'cd admin && npm ci' first"
fi

# ============================================================================
# STEP 6: E2E Test (requires Docker)
# ============================================================================
if [ "$SKIP_E2E" = false ]; then
    if check_cmd "docker" && check_cmd "python3"; then
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  Step $((TOTAL_STEPS + 1)): E2E Test (Docker)"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "  Starting Docker services, then running e2e_test.py"
        echo ""

        # Start Docker services
        echo "  [setup] docker compose up -d manager nginx"
        set +e
        docker compose up -d manager nginx 2>&1
        dc_exit=$?
        set -euo pipefail

        if [ $dc_exit -ne 0 ]; then
            TOTAL_STEPS=$((TOTAL_STEPS + 1))
            ERRORS+=("E2E Test (Docker compose up failed)")
            FAILED=$((FAILED + 1))
            red "  ❌ FAIL: E2E Test"
        else
            # Wait for services to be healthy
            echo "  [wait] Waiting for services to be healthy..."
            healthy=false
            for i in $(seq 1 30); do
                if curl -sf http://localhost/health > /dev/null 2>&1; then
                    echo "  [ok] Services ready after ${i}s."
                    healthy=true
                    break
                fi
                sleep 1
            done

            if [ "$healthy" = false ]; then
                echo "  ⚠️  Services did not become healthy within 30s"
                docker compose logs manager --tail=20 2>&1 || true
            fi

            # Run the E2E test
            TOTAL_STEPS=$((TOTAL_STEPS + 1))
            echo ""
            echo "  Running: python3 deploy/test/e2e_test.py"
            echo ""

            set +e
            python3 "$PROJECT_DIR/deploy/test/e2e_test.py"
            e2e_exit=$?
            set -euo pipefail

            if [ $e2e_exit -eq 0 ]; then
                PASSED=$((PASSED + 1))
                green "  ✅ PASS: E2E Test"
            else
                FAILED=$((FAILED + 1))
                ERRORS+=("E2E Test")
                red "  ❌ FAIL: E2E Test"
            fi
        fi

        # Tear down Docker services
        echo ""
        echo "  [teardown] docker compose down"
        docker compose down 2>&1 || true
    else
        echo "  ⚠️  Docker or Python3 not available — skipping E2E test"
    fi
else
    echo "  ⚠️  Skipping E2E test (--skip-e2e)"
fi

# ============================================================================
# Summary
# ============================================================================
echo ""
echo "╔══════════════════════════════════════════════════════╗"
echo "║                    CI VERIFICATION                    ║"
echo "╠══════════════════════════════════════════════════════╣"
printf "║  %-48s ║\n" "Total: $TOTAL_STEPS steps"
if [ "$FAILED" -eq 0 ]; then
    printf "║  %-48s ║\n" "$(green "Passed: $PASSED/$TOTAL_STEPS")"
else
    printf "║  %-48s ║\n" "$(red "Passed: $PASSED/$TOTAL_STEPS  |  Failed: $FAILED")"
fi
echo "╚══════════════════════════════════════════════════════╝"

if [ "$FAILED" -gt 0 ]; then
    echo ""
    red "❌ FAILURES:"
    for e in "${ERRORS[@]}"; do
        echo "   - $e"
    done
    echo ""
    exit 1
fi

echo ""
green "✅ All checks passed! Ready to push."
exit 0
