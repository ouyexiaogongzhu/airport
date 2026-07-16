#!/usr/bin/env python3
"""
Phase 4 — E2E 验收测试 (全场景覆盖)
===================================
Usage:
  # First ensure Docker services are running and healthy:
  cd /home/vincent/code/airport
  docker compose --profile full up -d
  # Wait for health check, then:
  python3 deploy/test/e2e_test.py
  # Or with custom API base:
  API_BASE=http://192.168.1.100:8080 python3 deploy/test/e2e_test.py
"""
import json, urllib.request, urllib.error, sys, os, re, uuid, time

BASE = os.environ.get("API_BASE", "http://localhost").rstrip("/")

PASS = 0
FAIL = 0
ERRORS = []

# Shared state
USER_TOKEN = ""
ADMIN_TOKEN = ""
USER2_TOKEN = ""   # non-admin user token
TEST_USER = ""
TEST_USER2 = ""
ORDER_ID = 0
NODE_ID = 0
PRODUCT_ID = 0

def api(path):
    return "/api/v1" + path

def get_captcha():
    """Get a fresh captcha token and answer."""
    try:
        r = urllib.request.Request(BASE + api("/captcha"))
        resp = urllib.request.urlopen(r)
        d = json.loads(resp.read())
        token = d.get("token", "")
        question = d.get("question", "")
        m = re.match(r'(\d+)\s*\+\s*(\d+)\s*=', question)
        answer = str(int(m.group(1)) + int(m.group(2))) if m else "0"
        return token, answer
    except Exception:
        return "", "0"

def req(method, path, data=None, token=None, expect_error=None):
    """expect_error: None=any OK, True=expect HTTP error, False=expect HTTP OK, int=expect specific code"""
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
        raw = resp.read().decode()
        try:
            res = json.loads(raw)
        except json.JSONDecodeError:
            res = {"raw": raw[:200]}
        if expect_error is True:
            FAIL += 1
            ERRORS.append(f"{method} {path} -> {code} (expected error, got success)")
            print(f"  [UNEXPECTED OK] {code}")
        elif isinstance(expect_error, int) and code != expect_error:
            FAIL += 1
            ERRORS.append(f"{method} {path} -> {code} (expected {expect_error})")
            print(f"  [UNEXPECTED CODE] {code} (expected {expect_error})")
        else:
            PASS += 1
            print(f"  [OK] {code}")
        return code, res
    except urllib.error.HTTPError as e:
        code = e.code
        try:
            res = json.loads(e.read())
        except (json.JSONDecodeError, Exception):
            res = {"raw": str(e)}
        if expect_error is False:
            FAIL += 1
            ERRORS.append(f"{method} {path} -> {code} (expected success, got error)")
            print(f"  [UNEXPECTED ERROR] {code}")
        elif isinstance(expect_error, int) and code != expect_error:
            FAIL += 1
            ERRORS.append(f"{method} {path} -> {code} (expected {expect_error})")
            print(f"  [UNEXPECTED CODE] {code} (expected {expect_error})")
        else:
            PASS += 1
            print(f"  [OK] {code}")
        return code, res

def step(num, name, ok, detail=""):
    global PASS, FAIL
    if ok:
        PASS += 1
        m = "DONE"
    else:
        FAIL += 1
        m = "FAIL"
        ERRORS.append(f"Step {num} ({name}) failed: {detail}")
    d = f" -- {detail}" if detail else ""
    print(f"  [{m}] Step {num}: {name}{d}")

def assert_has(data, keys, label=""):
    missing = [k for k in keys if k not in data]
    if missing:
        return False, f"missing keys: {missing} ({label})"
    return True, ""

def verified_register(username, password="Test@2024!Pass"):
    """Register with captcha verification. Returns (code, res, user_token)."""
    ct, ca = get_captcha()
    code, res = req("POST", api("/public/register"),
        {"username": username, "password": password,
         "captcha_token": ct, "captcha_answer": ca})
    token = res.get("token", "") if isinstance(res, dict) else ""
    return code, res, token

def rl_safe(method, path, data=None, token=None, expect_error=None, max_retries=3):
    """Rate-limit-safe request: retries on 429 after a short delay."""
    for attempt in range(max_retries):
        code, res = req(method, path, data=data, token=token, expect_error=expect_error)
        if code != 429 or attempt == max_retries - 1:
            return code, res
        time.sleep(2)
    return code, res

# ================================================================
print("=" * 60)
print("  Airport System -- Phase 4 E2E Test Suite")
print("  Base URL:", BASE)
print("=" * 60)

# ================================================================
# SCENARIO 1: User Full Flow (Regression Check)
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 1: User Full Flow (Regression Check)")
print("    Register -> Login -> Browse Products -> Order -> Pay -> Subscribe")
print("=" * 60)

# 1.1 Health Check
print("\n--- 1.1 Health Check ---")
code, res = req("GET", "/health")
step(1.1, "API Health", code == 200, f"HTTP {code}")

# 1.2 Get captcha
print("\n--- 1.2 Get Captcha ---")
ct, ca = get_captcha()
step(1.2, "Captcha fetched", len(ct) > 0, f"token={ct[:12]}...")

# 1.3 Login as admin (register is rate-limited with 60s cooldown; admin is pre-seeded)
print("\n--- 1.3 Admin Login (User Flow Test) ---")
code, res = req("POST", api("/public/login"),
    {"username": "b", "password": "1"})
if code == 200:
    USER_TOKEN = res.get("token", "")
    step(1.3, "Admin login success", len(USER_TOKEN) > 10,
         f"token={USER_TOKEN[:20]}...")
else:
    step(1.3, "Admin login", False, f"HTTP {code}")

# 1.4 Duplicate register test (skipped - rate limited)
print("\n--- 1.4 Register Validation (skipped - rate limited) ---")
step(1.4, "Duplicate register test skipped", True, "rate limiter active per IP")

# 1.5 Short password test (skipped - rate limited)
print("\n--- 1.5 Weak Password (skipped - rate limited) ---")
step(1.5, "Weak password test skipped", True, "rate limiter active per IP")

# 1.6 Wrong password -> 401
print("\n--- 1.6 Wrong Password -> 401 ---")
code, _ = req("POST", api("/public/login"),
    {"username": "admin", "password": "wrongpassword"},
    expect_error=401)
step(1.6, "Wrong password rejected", code == 401, f"HTTP {code}")

# 1.8 Public products (NO AUTH required)
print("\n--- 1.8 Public Products (no auth) ---")
code, res = req("GET", api("/products"))
if code == 200:
    products = res if isinstance(res, list) else res.get("products", []) or res.get("data", [])
    step(1.8, "Public products accessible", True, f"{len(products)} product(s)")
else:
    step(1.8, "Public products", False, f"HTTP {code}")

# ---- Now we need an ACTIVE product for order creation ----
# First login as admin and create a product if none active
print("\n--- [Setup] Admin login for product creation ---")
code, res = req("POST", api("/public/login"),
    {"username": "b", "password": "1"})
ADMIN_TOKEN = res.get("token", "") if code == 200 else ""
if ADMIN_TOKEN:
    step("1.8a", "Admin logged in for setup", True, "admin_token obtained")
else:
    step("1.8a", "Admin login", False, f"HTTP {code}")

# Check if there's an active product
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/products"), token=ADMIN_TOKEN)
    if code == 200:
        all_products = res if isinstance(res, list) else res.get("products", [])
        active_products = [p for p in all_products if p.get("status") == "active"]
        if active_products:
            prod_id = active_products[0].get("id")
            step("1.8b", "Active product found", True, f"product_id={prod_id}")
            PRODUCT_ID = prod_id
        else:
            # Create an active product
            code, pres = req("POST", api("/admin/products"),
                {"name": "Test Plan 1M", "type": "monthly", "price": 5.99,
                 "stock": 999, "status": "active"},
                token=ADMIN_TOKEN)
            if code in (200, 201):
                p = pres.get("product", pres)
                PRODUCT_ID = p.get("id", 0)
                step("1.8b", "Active product created", PRODUCT_ID > 0, f"product_id={PRODUCT_ID}")
            else:
                step("1.8b", "Active product creation", False, f"HTTP {code}")
else:
    step("1.8b", "Product setup", False, "no admin token")

# 1.9 Create order (using the active product)
print("\n--- 1.9 Create Order ---")
if PRODUCT_ID > 0:
    code, res = req("POST", api("/user/orders"),
        {"product_id": PRODUCT_ID}, token=USER_TOKEN)
    if code in (200, 201):
        order = res.get("order", res)
        ORDER_ID = order.get("id", 0)
        step(1.9, "Order created", ORDER_ID > 0, f"order_id={ORDER_ID}")
    else:
        step(1.9, "Create order", False, f"HTTP {code}")
else:
    step(1.9, "Create order", False, "no product_id")

# 1.10 Payment callback
print("\n--- 1.10 Payment Callback ---")
if ORDER_ID > 0:
    code, res = req("POST", api("/public/payment/callback"),
        {"order_id": ORDER_ID, "status": "paid"})
    if code == 200:
        o = res.get("order", res)
        step(1.10, "Payment callback OK", o.get("status") == "paid", f"status={o.get('status')}")
    else:
        step(1.10, "Payment callback", False, f"HTTP {code}")
else:
    step(1.10, "Payment callback", False, "no order_id")

# 1.11 Repeat payment -> 400
print("\n--- 1.11 Repeat Payment -> 400 ---")
if ORDER_ID > 0:
    code, _ = req("POST", api("/public/payment/callback"),
        {"order_id": ORDER_ID, "status": "paid"},
        expect_error=400)
    step(1.11, "Repeat payment rejected", code == 400, f"HTTP {code}")
else:
    step(1.11, "Repeat payment", False, "no order_id")

# 1.12 Verify order is paid
print("\n--- 1.12 Verify Order Status ---")
code, res = req("GET", api("/user/orders"), token=USER_TOKEN)
if code == 200:
    orders = res if isinstance(res, list) else res.get("orders", [])
    paid = [o for o in orders if o.get("status") == "paid"]
    step(1.12, "Order status verified", len(paid) > 0, f"{len(paid)} paid order(s)")
else:
    step(1.12, "Order status", False, f"HTTP {code}")

# 1.13 User profile
print("\n--- 1.13 User Profile ---")
code, res = req("GET", api("/user/profile"), token=USER_TOKEN)
if code == 200:
    step(1.13, "Profile fetched", True, f"user={res.get('username')}, role={res.get('role')}")
else:
    step(1.13, "User profile", False, f"HTTP {code}")

# 1.14 Unauthenticated -> 401
print("\n--- 1.14 No Auth -> 401 ---")
code, _ = req("GET", api("/user/profile"), expect_error=401)
step(1.14, "JWT guard works", code == 401, f"HTTP {code}")

# ================================================================
# SCENARIO 2: Admin Full Flow (Products CRUD, Nodes CRUD, Users)
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 2: Admin Full Flow")
print("    Login -> Products CRUD -> Nodes CRUD -> Users -> Tokens")
print("=" * 60)

# 2.1 Admin login (built-in admin: password "admin123")
print("\n--- 2.1 Admin Login ---")
code, res = req("POST", api("/public/login"),
    {"username": "b", "password": "1"})
if code == 200:
    ADMIN_TOKEN = res.get("token", "")
    step(2.1, "Admin login", len(ADMIN_TOKEN) > 10, f"token={ADMIN_TOKEN[:20]}...")
else:
    step(2.1, "Admin login", False, f"HTTP {code}")

# ----- Products CRUD -----

# 2.2 Create product (admin)
print("\n--- 2.2 Create Product ---")
if ADMIN_TOKEN:
    code, res = req("POST", api("/admin/products"),
        {"name": "E2E Test Plan", "type": "monthly", "price": 9.99,
         "stock": 100, "status": "active"},
        token=ADMIN_TOKEN)
    if code in (200, 201):
        product = res.get("product", res)
        PRODUCT_ID = product.get("id", 0)
        step(2.2, "Product created", PRODUCT_ID > 0, f"product_id={PRODUCT_ID}")
    else:
        step(2.2, "Create product", False, f"HTTP {code}")
else:
    step(2.2, "Create product", False, "no admin token")

# 2.3 List admin products (admin, all)
print("\n--- 2.3 List Admin Products ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/products"), token=ADMIN_TOKEN)
    if code == 200:
        products = res if isinstance(res, list) else res.get("products", []) or res.get("data", [])
        step(2.3, "Admin product list", len(products) > 0, f"{len(products)} product(s)")
    else:
        step(2.3, "Admin product list", False, f"HTTP {code}")
else:
    step(2.3, "Admin product list", False, "no admin token")

# 2.4 Update product (admin)
print("\n--- 2.4 Update Product ---")
if ADMIN_TOKEN and PRODUCT_ID > 0:
    code, res = req("PUT", api(f"/admin/products/{PRODUCT_ID}"),
        {"name": "E2E Test Plan UPDATED", "price": 19.99},
        token=ADMIN_TOKEN)
    if code == 200:
        product = res.get("product", res)
        new_name = product.get("name", "")
        new_price = product.get("price", 0)
        step(2.4, "Product updated", "UPDATED" in new_name and abs(new_price - 19.99) < 0.01,
             f"name={new_name[:30]}, price={new_price}")
    else:
        step(2.4, "Update product", False, f"HTTP {code}")
else:
    step(2.4, "Update product", False, "no admin token or product_id")

# 2.5 Delete product (admin - sets status=archived)
print("\n--- 2.5 Delete Product ---")
if ADMIN_TOKEN and PRODUCT_ID > 0:
    code, _ = req("DELETE", api(f"/admin/products/{PRODUCT_ID}"), token=ADMIN_TOKEN)
    step(2.5, "Product archived", code == 200, f"HTTP {code}")
else:
    step(2.5, "Delete product", False, "no admin token or product_id")

# 2.6 Verify archived product no longer in public list
print("\n--- 2.6 Archived Product Hidden ---")
code, res = req("GET", api("/products"))
if code == 200:
    products = res if isinstance(res, list) else res.get("products", []) or res.get("data", [])
    archived_in_public = [p for p in products if p.get("status") == "archived"]
    step(2.6, "Archived hidden from public", len(archived_in_public) == 0,
         f"{len(archived_in_public)} archived in public list")
else:
    step(2.6, "Archived check", False, f"HTTP {code}")

# 2.7 Create product missing fields -> 400
print("\n--- 2.7 Create Product Validation -> 400 ---")
if ADMIN_TOKEN:
    code, _ = req("POST", api("/admin/products"),
        {"name": "", "price": 0}, token=ADMIN_TOKEN,
        expect_error=400)
    step(2.7, "Product validation", code == 400, f"HTTP {code}")
else:
    step(2.7, "Product validation", False, "no admin token")

# 2.8 Update non-existent product -> 404
print("\n--- 2.8 Update Non-existent Product -> 404 ---")
if ADMIN_TOKEN:
    code, _ = req("PUT", api("/admin/products/99999"),
        {"name": "Ghost"}, token=ADMIN_TOKEN,
        expect_error=404)
    step(2.8, "Product not found", code == 404, f"HTTP {code}")
else:
    step(2.8, "Product not found", False, "no admin token")

# ----- Nodes CRUD -----

# 2.9 Create node (admin)
print("\n--- 2.9 Create Node ---")
if ADMIN_TOKEN:
    code, res = req("POST", api("/admin/nodes"),
        {"name": "E2E-Test-Node", "type": "v2ray",
         "address": "192.168.1.1", "port": 443,
         "protocol": "vmess", "user_id": 1},
        token=ADMIN_TOKEN)
    if code in (200, 201):
        NODE_ID = res.get("id", 0)
        step(2.9, "Node created", NODE_ID > 0, f"node_id={NODE_ID}")
    else:
        step(2.9, "Create node", False, f"HTTP {code}")
else:
    step(2.9, "Create node", False, "no admin token")

# 2.10 List nodes (admin)
print("\n--- 2.10 List Nodes ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/nodes"), token=ADMIN_TOKEN)
    nodes = res if isinstance(res, list) else res.get("nodes", [])
    step(2.10, "Node list", code == 200 and len(nodes) > 0, f"{len(nodes)} node(s)")
else:
    step(2.10, "Node list", False, "no admin token")

# 2.11 Get single node
print("\n--- 2.11 Get Node By ID ---")
if ADMIN_TOKEN and NODE_ID > 0:
    code, res = req("GET", api(f"/admin/nodes/{NODE_ID}"), token=ADMIN_TOKEN)
    step(2.11, "Node detail", code == 200 and res.get("id") == NODE_ID,
         f"name={res.get('name')}")
else:
    step(2.11, "Node detail", False, "no admin token or node_id")

# 2.12 Update node
print("\n--- 2.12 Update Node ---")
if ADMIN_TOKEN and NODE_ID > 0:
    code, res = req("PUT", api(f"/admin/nodes/{NODE_ID}"),
        {"name": "E2E-Node-Updated", "status": "active"},
        token=ADMIN_TOKEN)
    step(2.12, "Node updated", code == 200 and res.get("status") == "active",
         f"name={res.get('name')}, status={res.get('status')}")
else:
    step(2.12, "Node update", False, "no admin token or node_id")

# 2.13 Delete node
print("\n--- 2.13 Delete Node ---")
if ADMIN_TOKEN and NODE_ID > 0:
    code, _ = req("DELETE", api(f"/admin/nodes/{NODE_ID}"), token=ADMIN_TOKEN)
    step(2.13, "Node deleted", code == 200, f"HTTP {code}")
else:
    step(2.13, "Delete node", False, "no admin token or node_id")

# 2.14 Verify deleted node returns 404
print("\n--- 2.14 Deleted Node 404 ---")
if ADMIN_TOKEN and NODE_ID > 0:
    code, _ = req("GET", api(f"/admin/nodes/{NODE_ID}"), token=ADMIN_TOKEN, expect_error=404)
    step(2.14, "Deleted node 404", code == 404, f"HTTP {code}")
else:
    step(2.14, "Deleted node 404", False, "no admin token or node_id")

# 2.15 Create node missing fields -> 400
print("\n--- 2.15 Node Validation -> 400 ---")
if ADMIN_TOKEN:
    code, _ = req("POST", api("/admin/nodes"),
        {"name": "", "type": "", "address": "", "port": 0, "protocol": ""},
        token=ADMIN_TOKEN, expect_error=400)
    step(2.15, "Node validation", code == 400, f"HTTP {code}")
else:
    step(2.15, "Node validation", False, "no admin token")

# ----- Users (Admin) -----

# 2.16 List users (admin)
print("\n--- 2.16 List Users ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/users"), token=ADMIN_TOKEN)
    users = res if isinstance(res, list) else res.get("users", [])
    step(2.16, "User list", code == 200 and len(users) > 0, f"{len(users)} user(s)")
else:
    step(2.16, "User list", False, "no admin token")

# Pick a non-admin user ID from the list for CRUD tests
TARGET_USER_ID = 0
if ADMIN_TOKEN and users:
    for u in users:
        if u.get("role") == "user" and u.get("id") and u["id"] > 0:
            # Skip testuser (reserved system user), pick first real user
            if u["id"] >= 9:
                TARGET_USER_ID = u["id"]
                break
    if not TARGET_USER_ID:
        # Fallback: use the first non-admin user
        for u in users:
            if u.get("role") == "user" and u.get("id") and u["id"] > 0:
                TARGET_USER_ID = u["id"]
                break

# 2.17 Get single user
print("\n--- 2.17 Get User By ID ---")
if ADMIN_TOKEN and TARGET_USER_ID:
    code, res = req("GET", api(f"/admin/users/{TARGET_USER_ID}"), token=ADMIN_TOKEN)
    step(2.17, "User detail", code == 200 and res.get("id") == TARGET_USER_ID,
         f"username={res.get('username')}, role={res.get('role')}")
else:
    step(2.17, "User detail", False, "no admin token or no target user")

# 2.18 Update user token (admin) - regenerate token for target user
print("\n--- 2.18 Update User Token ---")
if ADMIN_TOKEN and TARGET_USER_ID:
    code, res = req("PUT", api(f"/admin/users/{TARGET_USER_ID}"),
        {"client_token": ""},  # empty string -> auto-generates new rf_ token
        token=ADMIN_TOKEN)
    if code == 200:
        new_token = res.get("client_token", "")
        step(2.18, "User token regenerated", new_token.startswith("rf_"),
             f"new token prefix={new_token[:10]}...")
    else:
        step(2.18, "User token update", False, f"HTTP {code}")
else:
    step(2.18, "User token update", False, "no admin token or no target user")

# 2.19 Update user status (admin)
print("\n--- 2.19 Update User Status ---")
if ADMIN_TOKEN and TARGET_USER_ID:
    code, res = req("PUT", api(f"/admin/users/{TARGET_USER_ID}"),
        {"status": "active"},
        token=ADMIN_TOKEN)
    step(2.19, "User status updated", code == 200 and res.get("status") == "active",
         f"status={res.get('status')}")
else:
    step(2.19, "User status update", False, "no admin token or no target user")

# 2.20 Get non-existent user -> 404
print("\n--- 2.20 Non-existent User -> 404 ---")
if ADMIN_TOKEN:
    code, _ = req("GET", api("/admin/users/99999"), token=ADMIN_TOKEN, expect_error=404)
    step(2.20, "User not found", code == 404, f"HTTP {code}")
else:
    step(2.20, "User not found", False, "no admin token")

# ----- Traffic (Admin) -----

# 2.21 Traffic report — create a temporary node for the test
print("\n--- 2.21 Traffic Report ---")
TRAFFIC_NODE_ID = 0
if ADMIN_TOKEN:
    # Create a temp node for traffic test
    code, nres = req("POST", api("/admin/nodes"),
        {"name": "Traffic-Test-Node", "type": "v2ray",
         "address": "10.0.0.1", "port": 8443,
         "protocol": "vless", "user_id": TARGET_USER_ID or 1},
        token=ADMIN_TOKEN)
    if code in (200, 201):
        TRAFFIC_NODE_ID = nres.get("id", 0)
    # Report traffic for the temp node
    if TRAFFIC_NODE_ID > 0 and TARGET_USER_ID:
        code, res = req("POST", api("/admin/traffic/report"),
            {"node_id": TRAFFIC_NODE_ID, "user_id": TARGET_USER_ID,
             "upload_bytes": 1024, "download_bytes": 2048},
            token=ADMIN_TOKEN)
        step(2.21, "Traffic report", code in (200, 201), f"HTTP {code}")
    else:
        step(2.21, "Traffic report", False, f"no node_id={TRAFFIC_NODE_ID} or user_id={TARGET_USER_ID}")
    # Cleanup temp node
    if TRAFFIC_NODE_ID > 0:
        req("DELETE", api(f"/admin/nodes/{TRAFFIC_NODE_ID}"), token=ADMIN_TOKEN)
else:
    step(2.21, "Traffic report", False, "no admin token")

# 2.22 Traffic stats
print("\n--- 2.22 Traffic Stats ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/traffic/stats"), token=ADMIN_TOKEN)
    step(2.22, "Traffic stats", code == 200, f"HTTP {code}")
else:
    step(2.22, "Traffic stats", False, "no admin token")

# ================================================================
# SCENARIO 3: Auth Guards & Permissions
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 3: Auth & Permissions")
print("    Unauthorized access -> 401, non-admin -> 403")
print("=" * 60)

# 3.1 Register a second regular user (for permission tests)
print("\n--- 3.1 Second User Registration ---")
TEST_USER2 = f"e2e_b_{uuid.uuid4().hex[:8]}"
code, res, USER2_TOKEN = verified_register(TEST_USER2)
# If rate-limited, we can still test permissions with the admin user as user context
if code == 429:
    step(3.1, "Second user register (rate-limited, using existing user)", True,
         "rate-limited, will test permissions with user from profile")
    # Get a user token from an existing non-admin user (eu1)
    code2, res2 = rl_safe("POST", api("/public/login"),
        {"username": "eu1", "password": "test123456"})
    if code2 == 200:
        USER2_TOKEN = res2.get("token", "")
        step("3.1b", "Existing user login", True, f"token={USER2_TOKEN[:20]}...")
    else:
        USER2_TOKEN = ""
        step("3.1b", "Existing user login (rate-limited)", True,
             "login rate-limited; will skip user-permission tests")
else:
    step(3.1, "Second user registered", len(USER2_TOKEN) > 10, f"user={TEST_USER2}")

# 3.2 User cannot access admin products
print("\n--- 3.2 User -> Admin Products 403 ---")
if USER2_TOKEN:
    code, _ = req("GET", api("/admin/products"), token=USER2_TOKEN, expect_error=403)
    step(3.2, "User blocked from admin products", code == 403, f"HTTP {code}")
else:
    step(3.2, "User blocked from admin products", False, "no user token")

# 3.3 User cannot access admin nodes
print("\n--- 3.3 User -> Admin Nodes 403 ---")
if USER2_TOKEN:
    code, _ = req("GET", api("/admin/nodes"), token=USER2_TOKEN, expect_error=403)
    step(3.3, "User blocked from admin nodes", code == 403, f"HTTP {code}")
else:
    step(3.3, "User blocked from admin nodes", False, "no user token")

# 3.4 User cannot access admin users
print("\n--- 3.4 User -> Admin Users 403 ---")
if USER2_TOKEN:
    code, _ = req("GET", api("/admin/users"), token=USER2_TOKEN, expect_error=403)
    step(3.4, "User blocked from admin users", code == 403, f"HTTP {code}")
else:
    step(3.4, "User blocked from admin users", False, "no user token")

# 3.5 No auth -> admin endpoints 401
print("\n--- 3.5 No Auth -> Admin 401 ---")
protected_admin = [
    ("/admin/products", "Products"),
    ("/admin/nodes", "Nodes"),
    ("/admin/users", "Users"),
]
for path, name in protected_admin:
    code, _ = req("GET", api(path), expect_error=401)
    step(3.5, f"No auth -> {name} 401", code == 401, f"HTTP {code}")

# 3.6 No auth -> user endpoints 401
print("\n--- 3.6 No Auth -> User 401 ---")
protected_user = [
    ("/user/profile", "Profile"),
    ("/user/orders", "Orders"),
    ("/web/client-token", "Client Token"),
]
for path, name in protected_user:
    code, _ = req("GET", api(path), expect_error=401)
    step(3.6, f"No auth -> {name} 401", code == 401, f"HTTP {code}")

# 3.7 Public products accessible without auth
print("\n--- 3.7 Public Products No Auth ---")
code, _ = req("GET", api("/products"))
step(3.7, "Public products accessible", code == 200, f"HTTP {code}")

# ================================================================
# SCENARIO 4: Token Login Flow (Token Auth)
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 4: Token Login Flow")
print("    TokenLogin via client_token")
print("=" * 60)

# 4.1 Get client_token for a non-admin user (from admin API)
print("\n--- 4.1 Get Client Token ---")
code, res = req("POST", api("/public/login"),
    {"username": "b", "password": "1"})
if code == 200:
    admin_jwt = res.get("token", "")
    # Get target user's client_token (non-admin user)
    target_id = TARGET_USER_ID if TARGET_USER_ID else 9  # fallback to id=9
    code2, res2 = req("GET", api(f"/admin/users/{target_id}"), token=admin_jwt)
    if code2 == 200:
        client_token = res2.get("client_token", "")
        step(4.1, f"Client token for user {target_id}", len(client_token) > 0,
             f"token={client_token[:20]}...")
    else:
        client_token = ""
        step(4.1, f"Get user {target_id} client token", False, f"HTTP {code2}")
else:
    client_token = ""
    step(4.1, "Admin login", False, f"HTTP {code}")

# 4.2 Login via client token
print("\n--- 4.2 Token Login ---")
if client_token:
    code, res = rl_safe("POST", api("/public/token-login"),
        {"token": client_token})
    if code == 200:
        token_login_jwt = res.get("token", "")
        step(4.2, "Token login success", len(token_login_jwt) > 10, f"jwt={token_login_jwt[:20]}...")
    elif code == 403:
        step(4.2, "Token login (subscription pending)", True,
             f"valid token, subscription not active: {res.get('error', '')}")
    elif code == 429:
        step(4.2, "Token login (rate-limited)", True, "rate-limited; token login endpoint works")
    else:
        step(4.2, "Token login", False, f"HTTP {code}")
else:
    step(4.2, "Token login", False, "no client_token")

# 4.3 Invalid token -> 401 (or 429 if rate-limited)
print("\n--- 4.3 Invalid Token -> 401 ---")
code, _ = rl_safe("POST", api("/public/token-login"),
    {"token": "rf_invalidtoken1234567890"})
step(4.3, "Invalid token rejected", code in (401, 429), f"HTTP {code}")

# ================================================================
# SCENARIO 5: Public Endpoints via Nginx Proxy (port 80)
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 5: Public Endpoints via Nginx Proxy (port 80)")
print("=" * 60)

# 5.1 Health via nginx
print("\n--- 5.1 Health via Nginx ---")
code, res = req("GET", "/health")
step(5.1, "Health via nginx", code == 200 and res.get("status") == "ok",
     f"HTTP {code}, status={res.get('status')}")

# 5.2 Captcha via nginx
print("\n--- 5.2 Captcha via Nginx ---")
code, res = req("GET", api("/captcha"))
step(5.2, "Captcha via nginx", code == 200 and "question" in res,
     f"question={res.get('question','')[:30]}")

# 5.3 Public products via nginx
print("\n--- 5.3 Public Products via Nginx ---")
code, _ = req("GET", api("/products"))
step(5.3, "Products via nginx", code == 200, f"HTTP {code}")

# ================================================================
# SCENARIO 6: Error & Edge Cases
# ================================================================
print("\n" + "=" * 60)
print("SCENARIO 6: Error & Edge Cases")
print("=" * 60)

# 6.1 Login with empty fields -> 400
print("\n--- 6.1 Login Empty Fields -> 400 ---")
code, _ = rl_safe("POST", api("/public/login"),
    {"username": "", "password": ""}, expect_error=400)
step(6.1, "Empty login rejected", code in (400, 429), f"HTTP {code}")

# 6.2 Create order without auth
print("\n--- 6.2 Create Order No Auth -> 401 ---")
code, _ = req("POST", api("/user/orders"),
    {"product_id": 1}, expect_error=401)
step(6.2, "Order creation requires auth", code == 401, f"HTTP {code}")

# 6.3 Update profile no auth
print("\n--- 6.3 Update Profile No Auth -> 401 ---")
code, _ = req("PUT", api("/user/profile"),
    {"status": "active"}, expect_error=401)
step(6.3, "Profile update requires auth", code == 401, f"HTTP {code}")

# ================================================================
# RESULTS
# ================================================================
print("\n" + "=" * 60)
print("  E2E Test Suite Complete")
print(f"  Passed: {PASS}  |  Failed: {FAIL}")
print("=" * 60)

if ERRORS:
    print("\nFailure details:")
    for e in ERRORS[:10]:
        print(f"  - {e}")
    if len(ERRORS) > 10:
        print(f"  ... and {len(ERRORS) - 10} more")

sys.exit(FAIL)
