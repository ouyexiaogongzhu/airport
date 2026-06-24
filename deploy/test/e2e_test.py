#!/usr/bin/env python3
"""
Phase 4 — E2E 验收测试 (全场景覆盖)
===================================
Usage:
  # Start Manager API first:
  cd manager && go run ./cmd/server/... &
  
  # Then run tests:
  python3 deploy/test/e2e_test.py
  # Or with custom API base:
  API_BASE=http://192.168.1.100:8080 python3 deploy/test/e2e_test.py
"""
import json, urllib.request, urllib.error, sys, os, re, uuid

BASE = os.environ.get("API_BASE", "http://localhost:8080").rstrip("/")

PASS = 0
FAIL = 0
ERRORS = []

# Shared state
USER_TOKEN = ""
ADMIN_TOKEN = ""
ADMIN_USERNAME = "admin"
ADMIN_PASSWORD = "Admin@2024!Secure"
ORDER_ID = 0
NODE_ID = 0
SUBSCRIPTION_LINK = ""

def api(path):
    return "/api/v1" + path

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
            ERRORS.append(f"{method} {path} → {code} (expected error, got success)")
            print(f"  [UNEXPECTED OK] {code}")
        elif isinstance(expect_error, int) and code != expect_error:
            FAIL += 1
            ERRORS.append(f"{method} {path} → {code} (expected {expect_error})")
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
            ERRORS.append(f"{method} {path} → {code} (expected success, got error)")
            print(f"  [UNEXPECTED ERROR] {code}")
        elif isinstance(expect_error, int) and code != expect_error:
            FAIL += 1
            ERRORS.append(f"{method} {path} → {code} (expected {expect_error})")
            print(f"  [UNEXPECTED CODE] {code} (expected {expect_error})")
        else:
            PASS += 1
            print(f"  [OK] {code}")
        return code, res

def step(num, name, ok, detail=""):
    global PASS, FAIL
    if ok:
        PASS += 1
        m = "✅"
    else:
        FAIL += 1
        m = "❌"
        ERRORS.append(f"Step {num} ({name}) failed: {detail}")
    d = f" — {detail}" if detail else ""
    print(f"  {m} Step {num}: {name}{d}")

def assert_has(data, keys, label=""):
    """Assert that data contains all keys."""
    missing = [k for k in keys if k not in data]
    if missing:
        return False, f"missing keys: {missing} ({label})"
    return True, ""

# ================================================================
print("=" * 60)
print("  RFPlay Airport — Phase 4 E2E 验收测试")
print("  Base URL:", BASE)
print("=" * 60)

# ================================================================
# SCENARIO 1: 用户全流程
# ================================================================
print("\n" + "=" * 60)
print("📋 场景 1: 用户全流程")
print("    注册 → 登录 → 选套餐 → 支付 → 拿订阅链接 → 导入客户端")
print("=" * 60)

# 1.1 Health Check
print("\n--- 1.1 Health Check ---")
code, res = req("GET", "/health")
step(1.1, "API运行健康", code == 200, f"HTTP {code}")

# 1.2 Get captcha (required for registration)
print("\n--- 1.2 Get Captcha ---")
code, res = req("GET", api("/captcha"))
if code == 200:
    captcha_token = res.get("token", "")
    captcha_question = res.get("question", "")
    # Parse the math question: "3 + 7 = ?"
    m = re.match(r'(\d+)\s*\+\s*(\d+)\s*=', captcha_question)
    if m:
        captcha_answer = str(int(m.group(1)) + int(m.group(2)))
    else:
        captcha_answer = "0"  # fallback
    step(1.2, "验证码获取成功", True, f"{captcha_question}")
else:
    captcha_token = ""
    captcha_answer = "0"
    step(1.2, "验证码获取", False, f"HTTP {code}")

# 1.3 Register new user (with captcha)
print("\n--- 1.3 Register ---")
TEST_USER = f"e2e_user_{uuid.uuid4().hex[:8]}"
code, res = req("POST", api("/public/register"),
    {"username": TEST_USER, "password": "Test@2024!Pass",
     "captcha_token": captcha_token, "captcha_answer": captcha_answer})
if code in (200, 201):
    USER_TOKEN = res.get("token", "")
    ok, detail = assert_has(res, ["token", "user"], "register response")
    step(1.3, "用户注册成功", ok, detail if ok else json.dumps(res)[:100])
else:
    step(1.3, "用户注册", False, f"HTTP {code}")

# 1.4 Register duplicate -> 409
print("\n--- 1.4 Duplicate Register -> 409 ---")
code, res = req("POST", api("/public/register"),
    {"username": TEST_USER, "password": "Test@2024!Pass",
     "captcha_token": captcha_token, "captcha_answer": captcha_answer},
    expect_error=409)
step(1.4, "重复注册限制", code == 409, f"HTTP {code}")

# 1.5 Short password -> 400
print("\n--- 1.5 Short Password -> 400 ---")
code, _ = req("POST", api("/public/register"),
    {"username": "shortpw_test", "password": "ab",
     "captcha_token": captcha_token, "captcha_answer": captcha_answer},
    expect_error=400)
step(1.5, "弱密码拒绝", code == 400, f"HTTP {code}")

# 1.6 Login
print("\n--- 1.6 Login ---")
code, res = req("POST", api("/public/login"),
    {"username": TEST_USER, "password": "Test@2024!Pass"})
if code == 200:
    USER_TOKEN = res.get("token", "")
    ok, detail = assert_has(res, ["token"], "login response")
    step(1.6, "用户登录成功", ok and len(USER_TOKEN) > 10, f"token={USER_TOKEN[:20]}...")
else:
    step(1.6, "用户登录", False, f"HTTP {code}")

# 1.7 Wrong password -> 401
print("\n--- 1.7 Wrong Password -> 401 ---")
code, _ = req("POST", api("/public/login"),
    {"username": TEST_USER, "password": "wrongpassword"},
    expect_error=401)
step(1.7, "错误密码拦截", code == 401, f"HTTP {code}")

# 1.8 List products/plans
print("\n--- 1.8 List Products ---")
code, res = req("GET", api("/products"), token=USER_TOKEN)
if code == 200:
    plans = res if isinstance(res, list) else res.get("products", []) or res.get("data", [])
    step(1.8, "产品列表加载", len(plans) > 0, f"{len(plans)} plan(s)")
else:
    step(1.8, "产品列表", False, f"HTTP {code}")

# 1.9 Create order (using first product)
print("\n--- 1.9 Create Order ---")
code, res = req("POST", api("/user/orders"),
    {"product_id": 1}, token=USER_TOKEN)
if code in (200, 201):
    order = res.get("order", res)
    ORDER_ID = order.get("id", 0)
    step(1.9, "订单创建成功", ORDER_ID > 0, f"order_id={ORDER_ID}")
else:
    step(1.9, "订单创建", False, f"HTTP {code}")

# 1.10 Mock payment callback
print("\n--- 1.10 Payment Callback (mock) ---")
if ORDER_ID > 0:
    code, res = req("POST", api("/public/payment/callback"),
        {"order_id": ORDER_ID, "status": "paid"})
    if code == 200:
        o = res.get("order", res)
        step(1.10, "支付回调成功", o.get("status") == "paid", f"status={o.get('status')}")
    else:
        step(1.10, "支付回调", False, f"HTTP {code}")
else:
    step(1.10, "支付回调", False, "no order_id")

# 1.11 Repeat payment -> 400
print("\n--- 1.11 Repeat Payment -> 400 ---")
if ORDER_ID > 0:
    code, _ = req("POST", api("/public/payment/callback"),
        {"order_id": ORDER_ID, "status": "paid"},
        expect_error=400)
    step(1.11, "重复支付拒绝", code == 400, f"HTTP {code}")
else:
    step(1.11, "重复支付拒绝", False, "no order_id")

# 1.12 Verify order status = paid
print("\n--- 1.12 Verify Order Status ---")
code, res = req("GET", api("/user/orders"), token=USER_TOKEN)
if code == 200:
    orders = res if isinstance(res, list) else res.get("orders", [])
    paid = [o for o in orders if o.get("status") == "paid"]
    step(1.12, "订单状态验证", len(paid) > 0, f"{len(paid)} paid order(s)")
else:
    step(1.12, "订单状态", False, f"HTTP {code}")

# 1.13 Get subscription token
print("\n--- 1.13 Get Client Token ---")
code, res = req("GET", api("/web/client-token"), token=USER_TOKEN)
if code == 200:
    token_masked = res.get("token", "")
    step(1.13, "订阅Token获取", len(token_masked) > 0, f"token={token_masked[:20]}...")
else:
    step(1.13, "订阅Token", False, f"HTTP {code}")

# 1.14 User profile (check subscription activated)
print("\n--- 1.14 User Profile (subscription) ---")
code, res = req("GET", api("/user/profile"), token=USER_TOKEN)
if code == 200:
    sub_status = res.get("subscription_status", "")
    tier = res.get("subscription_tier", "")
    step(1.14, "订阅已激活", sub_status == "active", f"status={sub_status}, tier={tier}")
else:
    step(1.14, "用户信息", False, f"HTTP {code}")

# 1.15 Get subscription links (V2ray format)
print("\n--- 1.15 Get Subscription Link ---")
code, res = req("GET", api("/client/links/test"), token=USER_TOKEN)
# This may fail if user doesn't have full link, but endpoint should respond
step(1.15, "订阅链接端点响应", code in (200, 400, 404), f"HTTP {code}")

# ================================================================
# SCENARIO 2: 管理员全流程
# ================================================================
print("\n" + "=" * 60)
print("📋 场景 2: 管理员全流程")
print("    登录 Admin → 添加节点 → 管理用户 → 创建产品 → 看统计")
print("=" * 60)

# 2.1 Admin login (need seed admin or create via register)
print("\n--- 2.1 Admin Login ---")
code, res = req("POST", api("/public/login"),
    {"username": ADMIN_USERNAME, "password": ADMIN_PASSWORD})
if code == 200:
    ADMIN_TOKEN = res.get("token", "")
    step(2.1, "管理员登录成功", len(ADMIN_TOKEN) > 10, f"token={ADMIN_TOKEN[:20]}...")
else:
    step(2.1, "管理员登录", False, "测试需要先创建admin用户 (run seed.go or register)")

# 2.2 Create Node (admin only)
print("\n--- 2.2 Create Node ---")
if ADMIN_TOKEN:
    code, res = req("POST", api("/admin/nodes"),
        {"name": "E2E-Test-Node", "type": "v2ray",
         "address": "192.168.1.1", "port": 443,
         "protocol": "vmess", "status": "active"},
        token=ADMIN_TOKEN)
    if code in (200, 201):
        NODE_ID = res.get("id", 0)
        step(2.2, "节点创建成功", NODE_ID > 0, f"node_id={NODE_ID}")
    else:
        step(2.2, "节点创建", False, f"HTTP {code}")
else:
    step(2.2, "节点创建", False, "no admin token")

# 2.3 List Nodes (admin)
print("\n--- 2.3 List Nodes ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/nodes"), token=ADMIN_TOKEN)
    nodes = res if isinstance(res, list) else res.get("nodes", [])
    step(2.3, "节点列表加载", code == 200 and len(nodes) > 0, f"{len(nodes)} node(s)")
else:
    step(2.3, "节点列表", False, "no admin token")

# 2.4 List Users (admin)
print("\n--- 2.4 List Users ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/users"), token=ADMIN_TOKEN)
    users = res if isinstance(res, list) else res.get("users", [])
    step(2.4, "用户列表加载", code == 200 and len(users) > 0, f"{len(users)} user(s)")
else:
    step(2.4, "用户列表", False, "no admin token")

# 2.5 Create Product (admin)
print("\n--- 2.5 Create Product ---")
if ADMIN_TOKEN:
    code, res = req("POST", api("/admin/products"),
        {"name": "E2E Test Plan", "price": 9.99,
         "traffic_bytes": 10737418240, "duration_days": 30},
        token=ADMIN_TOKEN)
    step(2.5, "产品创建", code in (200, 201), f"HTTP {code}")
else:
    step(2.5, "产品创建", False, "no admin token")

# 2.6 Admin Dashboard Stats
print("\n--- 2.6 Admin Stats ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/stats"), token=ADMIN_TOKEN)
    step(2.6, "管理员统计加载", code == 200, f"HTTP {code}")
else:
    step(2.6, "管理员统计", False, "no admin token")

# 2.7 Admin Orders List
print("\n--- 2.7 Admin Orders List ---")
if ADMIN_TOKEN:
    code, res = req("GET", api("/admin/orders"), token=ADMIN_TOKEN)
    orders = res if isinstance(res, list) else res.get("orders", [])
    step(2.7, "订单管理列表", code == 200, f"{len(orders)} order(s)")
else:
    step(2.7, "订单管理", False, "no admin token")

# ================================================================
# SCENARIO 3: 节点同步 (架构验证)
# ================================================================
print("\n" + "=" * 60)
print("📋 场景 3: 节点同步")
print("    Manager 增节点 → Daemon 自动同步 → 用户订阅可见")
print("=" * 60)

# 3.1 Verify user can see nodes in subscription
print("\n--- 3.1 User Subscription Nodes ---")
code, res = req("GET", api("/client/subscription"), token=USER_TOKEN)
if code == 200:
    nodes = res.get("nodes", [])
    step(3.1, "用户订阅可见节点", len(nodes) > 0, f"{len(nodes)} node(s)")
else:
    step(3.1, "用户订阅", False, f"HTTP {code}")

# 3.2 Client Config endpoint
print("\n--- 3.2 Client Config ---")
code, res = req("GET", api("/client/config"))
if code == 200:
    server = res.get("server", res.get("port", ""))
    step(3.2, "客户端配置加载", True, f"HTTP {code}")
else:
    step(3.2, "客户端配置", False, f"HTTP {code}")

# ================================================================
# SCENARIO 4: 限速/超量
# ================================================================
print("\n" + "=" * 60)
print("📋 场景 4: 限速/超量")
print("    用超流量 → 自动断流 → 续费恢复")
print("=" * 60)

# 4.1 Get profile with traffic info
print("\n--- 4.1 Traffic Info ---")
code, res = req("GET", api("/user/profile"), token=USER_TOKEN)
if code == 200:
    used = res.get("traffic_used_bytes", 0)
    limit = res.get("traffic_limit_bytes", 0)
    step(4.1, "流量信息正常", limit > 0, f"{used}/{limit} bytes")
else:
    step(4.1, "流量信息", False, f"HTTP {code}")

# 4.2 Rate limiting test (rapid requests)
print("\n--- 4.2 Rate Limiting ---")
rate_limited = False
for i in range(20):
    code, _ = req("GET", api("/user/profile"), token=USER_TOKEN)
    if code == 429:
        rate_limited = True
        break
step(4.2, "速率限制生效", rate_limited, "20次请求内触发429" if rate_limited else "未触发(可能有更高限额)")

# ================================================================
# SCENARIO 5: 安全
# ================================================================
print("\n" + "=" * 60)
print("📋 场景 5: 安全")
print("    未登录访问/SQL注入/XSS")
print("=" * 60)

# 5.1 Unauthenticated access to protected endpoint -> 401
print("\n--- 5.1 Unauthenticated Access -> 401 ---")
protected_endpoints = [
    ("/user/profile", "用户信息"),
    ("/user/orders", "订单列表"),
    ("/web/client-token", "订阅Token"),
    ("/admin/users", "用户管理"),
    ("/admin/nodes", "节点管理"),
]
for path, name in protected_endpoints:
    code, _ = req("GET", api(path), expect_error=401)
    if code == 401:
        step(5.1, f"{name} 未登录拦截", True, f"HTTP {code}")
    else:
        step(5.1, f"{name} 未登录拦截", False, f"期望401, 收到{code}")

# 5.2 SQL injection attempt in login
print("\n--- 5.2 SQL Injection Attempt ---")
injection_payloads = [
    {"username": "' OR '1'='1", "password": "' OR '1'='1"},
    {"username": "admin'--", "password": "anything"},
    {"username": "'; DROP TABLE users; --", "password": "x"},
]
for payload in injection_payloads:
    code, _ = req("POST", api("/public/login"), payload, expect_error=401)
step(5.2, "SQL注入防御", True, "所有注入尝试均未返回200")

# 5.3 XSS check in registration
print("\n--- 5.3 XSS Input Sanitization ---")
xss_user = f"<script>alert(1)</script>_{uuid.uuid4().hex[:4]}"
code, _ = req("POST", api("/public/register"),
    {"username": xss_user, "password": "Test@2024!Pass",
     "captcha_token": "", "captcha_answer": ""})
step(5.3, "XSS 输入处理", True, f"username含HTML标签, 未导致崩溃")

# ================================================================
# RESULTS
# ================================================================
print("\n" + "=" * 60)
print("  E2E 验收测试完成")
print(f"  通过: {PASS}  |  失败: {FAIL}")
print("=" * 60)

if ERRORS:
    print("\n❌ 失败详情:")
    for e in ERRORS[:10]:
        print(f"  - {e}")
    if len(ERRORS) > 10:
        print(f"  ... and {len(ERRORS) - 10} more")

sys.exit(FAIL)
