#!/usr/bin/env python3
"""Phase 2 — API 集成测试 (17 steps)"""
import json, urllib.request, urllib.error, sys, os

BASE = os.environ.get("API_BASE", "http://localhost:8080")
def api(path): return "/api/v1" + path
PASS = 0
FAIL = 0
TOKEN = ""
ADMIN_TOKEN = ""

def req(method, path, data=None, token=None, expect_http_error=False):
    global PASS, FAIL
    url = BASE + path
    body = json.dumps(data).encode() if data else None
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        resp = urllib.request.urlopen(r)
        code = resp.status
        res = json.loads(resp.read())
        if expect_http_error:
            FAIL += 1
            print("  [UNEXPECTED OK]", method, path, "->", code)
        else:
            PASS += 1
            print("  [OK]", method, path, "->", code)
        return code, res
    except urllib.error.HTTPError as e:
        code = e.code
        try:
            res = json.loads(e.read())
        except:
            res = {"raw": str(e)}
        if expect_http_error:
            PASS += 1
            print("  [OK (expected error)]", method, path, "->", code)
        else:
            FAIL += 1
            print("  [FAIL]", method, path, "->", code)
            if code >= 500:
                print("     Body:", json.dumps(res, indent=2)[:200])
        return code, res

print("==== Phase 2: API 集成测试 ====")

# 1. Health
print("--- 1. Health ---")
code, _ = req("GET", "/health")
assert code == 200, "Manager not running"

# 2. Register
print("--- 2. Register ---")
code, res = req("POST", api("/public/register"),
    {"username": "ittest", "password": "test123456"})
if code == 409:
    PASS += 1
    FAIL -= 1
    print("  (already exists, OK)")

# 3. Duplicate -> 409 expected
print("--- 3. Duplicate Register -> 409 ---")
code, _ = req("POST", api("/public/register"),
    {"username": "ittest", "password": "test123456"},
    expect_http_error=True)

# 4. Short password -> 400 expected
print("--- 4. Short Password -> 400 ---")
code, _ = req("POST", api("/public/register"),
    {"username": "shortpw", "password": "ab"},
    expect_http_error=True)

# 5. Login
print("--- 5. Login ---")
code, res = req("POST", api("/public/login"),
    {"username": "ittest", "password": "test123456"})
if code == 200:
    TOKEN = res.get("token", "")
    print("     Token:", TOKEN[:30], "...")

# 6. Wrong password -> 401 expected
print("--- 6. Wrong Password -> 401 ---")
req("POST", api("/public/login"),
    {"username": "ittest", "password": "wrongpass"},
    expect_http_error=True)

# 7. Admin login
print("--- 7. Admin Login ---")
code, res = req("POST", api("/public/login"),
    {"username": "admin", "password": "admin123"})
if code == 200:
    ADMIN_TOKEN = res.get("token", "")
    print("     Admin Token:", ADMIN_TOKEN[:30], "...")

# 8. Create Node
print("--- 8. Create Node ---")
if ADMIN_TOKEN:
    req("POST", api("/admin/nodes"),
        {"name": "TestNode", "type": "v2ray",
         "address": "127.0.0.1", "port": 443,
         "protocol": "vmess", "status": "active"},
        token=ADMIN_TOKEN)
else:
    print("  skip (no admin token)")

# 9. List Nodes
print("--- 9. List Nodes ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/nodes"), token=ADMIN_TOKEN)
    if code == 200: print("     Found", len(res), "node(s)")
else:
    print("  skip")

# 10. Create Order
print("--- 10. Create Order ---")
code, res = req("POST", api("/user/orders"),
    {"product_id": 1}, token=TOKEN)
order_id = 0
if code in (200, 201):
    order = res.get("order", res)
    order_id = order.get("id", 0)
print("     Order ID:", order_id)

# 11. Payment Callback
print("--- 11. Payment Callback ---")
if order_id:
    code, res = req("POST", api("/public/payment/callback"),
        {"order_id": order_id, "status": "paid"})
    if code == 200:
        o = res.get("order", res)
        print("     status:", o.get("status"))

# 12. Verify Order
print("--- 12. Verify Order ---")
code, res = req("GET", api("/user/orders"), token=TOKEN)
if code == 200 and isinstance(res, list):
    paid = [o for o in res if o.get("status") == "paid"]
    print("     ", len(paid), "paid order(s)")
    if not paid: FAIL += 1

# 13. Repeat Payment -> 400 expected
print("--- 13. Repeat Payment -> 400 ---")
if order_id:
    req("POST", api("/public/payment/callback"),
        {"order_id": order_id, "status": "paid"},
        expect_http_error=True)

# 14. Traffic Report
print("--- 14. Traffic Report ---")
if ADMIN_TOKEN:
    req("POST", api("/admin/traffic/report"),
        {"node_id": 1, "user_id": 1,
         "upload_bytes": 1024, "download_bytes": 2048},
        token=ADMIN_TOKEN)

# 15. Traffic Stats
print("--- 15. Traffic Stats ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/traffic/stats"), token=ADMIN_TOKEN)
    if code == 200: print("     Stats OK")

# 16. User Profile
print("--- 16. User Profile ---")
code, res = req("GET", api("/user/profile"), token=TOKEN)
if code == 200:
    print("     User:", res.get("username"), "role:", res.get("role"))

# 17. JWT Guard -> 401 expected
print("--- 17. JWT Guard -> 401 ---")
req("GET", api("/user/profile"), expect_http_error=True)

print()
print("==== Phase 2 Result:", PASS, "passed,", FAIL, "failed ====")
sys.exit(FAIL)
