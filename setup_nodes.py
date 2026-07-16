import json, subprocess, sys, base64

BASE = "http://localhost:80"

def api(method, path, data=None, token=None):
    cmd = ["curl", "-sf", "-X", method, f"{BASE}{path}",
           "-H", "Content-Type: application/json"]
    if token:
        cmd += ["-H", "Authorization: Bearer " + token]
    if data:
        cmd += ["-d", json.dumps(data)]
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
    out = r.stdout.strip()
    if not out:
        return out
    try:
        return json.loads(out)
    except:
        return out

# Admin login
print("=== Admin Login ===")
adm = api("POST", "/api/v1/public/login", {"username":"admin","password":"admin123"})
adm_token = adm["token"]
print(f"Admin token ok")

# List existing nodes
print("\n=== List Nodes ===")
nodes = api("GET", "/api/v1/admin/nodes", token=adm_token)
print(f"Existing nodes: {json.dumps(nodes, indent=2)[:200]}")

# Create a node pointing to this machine
print("\n=== Create Node ===")
node_data = {
    "name": "Local Server",
    "address": "192.168.100.235",
    "port": 443,
    "protocol": "vmess",
    "transport": "ws",
    "path": "/xray/ws",
    "host": "rfplay.example.com",
    "tls": False,
    "speed_limit_mbps": 100,
    "traffic_ratio": 1.0,
    "sort_order": 0,
    "remark": "Local development server"
}
node = api("POST", "/api/v1/admin/nodes", node_data, token=adm_token)
print(f"Created node: {json.dumps(node, indent=2)[:300]}")

# Create another node with VLESS+WS
print("\n=== Create Node 2 (VLESS) ===")
node2_data = {
    "name": "Local VLESS",
    "address": "192.168.100.235",
    "port": 8443,
    "protocol": "vless",
    "transport": "ws",
    "path": "/xray/vless-ws",
    "host": "",
    "tls": False,
    "speed_limit_mbps": 100,
    "traffic_ratio": 1.0,
    "sort_order": 1,
    "remark": "Local VLESS+WS node"
}
node2 = api("POST", "/api/v1/admin/nodes", node2_data, token=adm_token)
print(f"Created node 2: {json.dumps(node2, indent=2)[:300]}")

# Login fluttertest and check subscription again
print("\n=== fluttertest Subscription ===")
ft = api("POST", "/api/v1/public/login", {"username":"fluttertest","password":"test123"})
ft_token = ft["token"]
ft_client = ft["user"]["client_token"]
sub = api("GET", "/api/v1/client/subscription", token=ft_token)
print(f"Nodes in subscription: {len(sub.get('nodes',[]))}")
if sub.get('nodes'):
    for n in sub['nodes']:
        print(f"  - {n}")

# Get V2Ray links
print("\n=== V2Ray Links ===")
links_b64 = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(f"Base64 ({len(links_b64)} bytes)")
if links_b64:
    decoded = base64.b64decode(links_b64).decode()
    print(f"Decoded:")
    for line in decoded.strip().split('\n'):
        print(f"  {line[:100]}...")

# Clash
print(f"\n=== Clash Config ===")
clash = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}/clash"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(clash[:800])

# Sing-box
print(f"\n=== Sing-box Config ===")
singbox = subprocess.run(
    ["curl", "-sf", f"{BASE}/api/v1/client/links/{ft_client}/singbox"],
    capture_output=True, text=True, timeout=10).stdout.strip()
print(singbox[:800])
