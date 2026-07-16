#!/bin/bash
set -e

python3 <<'PYEOF'
import json, subprocess, sys

BASE = "http://localhost:80"

def curl(method, path, data=None, token=None):
    cmd = ["curl", "-sf", "-X", method, f"{BASE}{path}",
           "-H", "Content-Type: application/json"]
    if token:
        cmd += ["-H", f"Authorization: Bearer {token}"]
    if data:
        cmd += ["-d", json.dumps(data)]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
    if r.returncode != 0:
        print(f"FAIL [{method} {path}]: {r.stderr}")
        sys.exit(1)
    return json.loads(r.stdout)

print("=== Login fluttertest ===")
ft = curl("POST", "/api/v1/public/login", {"username":"fluttertest","password":"test123"})
ft_token = ft["token"]
ft_client = ft["user"]["client_token"]
print(f"Client: {ft_client}")

print("\n=== Create Order ===")
order = curl("POST", "/api/v1/user/orders", {"product_id":2}, token=ft_token)
oid = order.get("order",{}).get("id") or order.get("id")
print(f"Order ID: {oid}")

print("\n=== Payment Callback ===")
pay = curl("POST", "/api/v1/public/payment/callback",
           {"order_id": oid, "amount": 5.99, "provider": "mock"})
print(json.dumps(pay, indent=2)[:200])

print("\n=== Verify Subscription ===")
sub = curl("GET", "/api/v1/client/subscription", token=ft_token)
print(json.dumps(sub, indent=2)[:300])

print("\n=== V2Ray Links ===")
links_b64 = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(f"Base64: {links_b64}")
import base64
try:
    decoded = base64.b64decode(links_b64).decode()
    print(f"Decoded:\n{decoded}")
except:
    print("Decode failed")

print("\n=== Clash Config ===")
clash = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}/clash"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(clash[:500])

print("\n=== Sing-box Config ===")
singbox = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}/singbox"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(singbox[:500])
PYEOF
