// 客戶端訂閱數據面 — 對齊 manager/internal/handler/subscription.go + middleware/auth.go JWTProtected
// 契約：錯誤字串/狀態碼/JSON 鍵序/響應頭逐字對齊 Go。
import { Hono } from 'hono';
import type { Env } from '../index';
import { verifyJwt, type Claims } from '../lib/jwt';
import { buildFormat, goJSON, type FormatKind } from '../lib/subformats';
import { encodeNodeToURI, type NodeRow, type UserCreds } from '../lib/xrayuri';

type UserRow = {
  id: number;
  subscription_status: string | null;
  subscription_tier: string | null;
  traffic_limit_bytes: number | null;
  traffic_used_bytes: number | null;
  expire_time: number | null;
  vless_uuid: string | null;
  ss_password: string | null;
  trojan_password: string | null;
};

type CachedBody = { ct: string; body: string };

const KV_TTL = 60;

function subsHeaders(user: UserRow): Record<string, string> {
  const used = user.traffic_used_bytes ?? 0;
  const limit = user.traffic_limit_bytes ?? 0;
  const expire = user.expire_time ?? 0;
  let remaining = limit - used;
  if (remaining < 0) remaining = 0;
  return {
    // Go: fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d", used, limit, expire)
    'Subscription-Userinfo': `upload=0; download=${used}; total=${limit}; expire=${expire}`,
    'X-UltraUsage-Remaining': String(remaining),
    'X-UltraUsage-Total': String(limit),
    'X-UltraUsage-Expiry': String(expire),
  };
}

function creds(user: UserRow): UserCreds {
  return {
    id: user.id,
    vless_uuid: user.vless_uuid,
    ss_password: user.ss_password,
    trojan_password: user.trojan_password,
  };
}

async function getActiveNodes(env: Env): Promise<NodeRow[]> {
  const { results } = await env.DB.prepare(
    "SELECT name, address, port, protocol, reality_public_key, reality_short_id FROM nodes WHERE status = 'active' ORDER BY id",
  ).all<NodeRow>();
  return results ?? [];
}

// JWTProtected 移植：401 JSON 與 Go 逐字一致
async function requireJwt(c: { env: Env; req: { header: (k: string) => string | undefined } }): Promise<Claims | Response> {
  const secret = c.env.JWT_SECRET;
  if (!secret) {
    return new Response(goJSON({ error: 'server configuration error' }), {
      status: 500,
      headers: { 'Content-Type': 'application/json' },
    });
  }
  const authHeader = c.req.header('Authorization');
  let tokenStr: string;
  if (!authHeader) {
    // Portal（純 cookie 會話）兜底：同一 JWT/密鑰，僅傳輸通道不同（Go 設計缺陷修補）
    const m = (c.req.header('Cookie') ?? '').match(/(?:^|;\s*)session=([^;]+)/);
    tokenStr = m ? m[1] : '';
    if (!tokenStr) {
      return Response.json({ error: 'missing authorization header' }, { status: 401 });
    }
  } else {
    const idx = authHeader.indexOf(' '); // Go strings.SplitN(header, " ", 2)
    const scheme = idx === -1 ? authHeader : authHeader.slice(0, idx);
    tokenStr = idx === -1 ? '' : authHeader.slice(idx + 1);
    if (idx === -1 || scheme.toLowerCase() !== 'bearer') {
      return Response.json({ error: 'invalid authorization header format' }, { status: 401 });
    }
  }
  const claims = await verifyJwt(tokenStr, secret);
  if (!claims) {
    return Response.json({ error: 'invalid or expired token' }, { status: 401 });
  }
  return claims;
}

function isResponse(x: unknown): x is Response {
  return x instanceof Response;
}

export function clientRoutes() {
  const r = new Hono<{ Bindings: Env }>();

  // GET /client/config — GetClientConfig
  r.get('/config', (c) => {
    const env = c.env as Env & { PORTAL_URL?: string };
    const portalURL = env.PORTAL_URL || 'http://localhost:5173';
    return c.body(goJSON({ portal_url: portalURL, renewal_path: '/plans' }), 200, {
      'Content-Type': 'application/json',
    });
  });

  // GET /client/links/:token{,/clash,/singbox} — handleLinksRequest
  r.get('/links/:token', (c) => handleLinks(c, c.req.param('token'), 'v2ray'));
  r.get('/links/:token/clash', (c) => handleLinks(c, c.req.param('token'), 'clash'));
  r.get('/links/:token/singbox', (c) => handleLinks(c, c.req.param('token'), 'singbox'));

  // QR 暫緩（§8-P1.5）
  r.get('/links/:token/qrcode', (c) => c.json({ error: 'NOT_IMPLEMENTED' }, 501));

  // GET /client/subscription — GetSubscription（Bearer JWT）
  r.get('/subscription', async (c) => {
    const auth = await requireJwt(c);
    if (isResponse(auth)) return auth;
    const userID = auth.user_id;

    const user = await c.env.DB.prepare('SELECT * FROM users WHERE id = ? LIMIT 1')
      .bind(userID)
      .first<UserRow>();
    if (!user) {
      return c.json({ error: 'user not found' }, 404);
    }

    const status = user.subscription_status ?? '';
    if (status !== 'active') {
      if (status === 'pending') {
        return c.json({ error: 'SUBSCRIPTION_PENDING' }, 403);
      }
      return c.json({ error: 'SUBSCRIPTION_EXPIRED' }, 403);
    }

    const nodes = await getActiveNodes(c.env);
    const u = creds(user);
    const nodeURIs: string[] = [];
    for (const node of nodes) {
      const uri = encodeNodeToURI(node, u);
      if (uri !== '') nodeURIs.push(uri);
    }

    const used = user.traffic_used_bytes ?? 0;
    const limit = user.traffic_limit_bytes ?? 0;
    let trafficRemaining = limit - used;
    if (trafficRemaining < 0) trafficRemaining = 0;

    // Go struct 欄位序：user{id,tier,traffic_remaining_bytes,expire_time}, nodes, routing, subscription_version
    return c.body(
      goJSON({
        user: {
          id: user.id,
          tier: user.subscription_tier ?? '',
          traffic_remaining_bytes: trafficRemaining,
          expire_time: user.expire_time ?? 0,
        },
        nodes: nodeURIs,
        routing: {
          geoip_url: 'https://api.rfplay.uk/assets/geoip.dat',
          geosite_url: 'https://api.rfplay.uk/assets/geosite.dat',
          geoip_etag: '',
          geosite_etag: '',
        },
        subscription_version: 1,
      }),
      200,
      { 'Content-Type': 'application/json' },
    );
  });

  return r;
}

// handleLinksRequest：鑑權/狀態檢查永不走緩存（每請求讀 D1 用戶行）；
// 僅在狀態通過後，格式化 body 以 KV 緩存 60s（key=token+format），KV 異常靜默穿透直讀 D1。
async function handleLinks(c: { env: Env; req: { param: (k: string) => string }; executionCtx: { waitUntil: (p: Promise<unknown>) => void } }, token: string, format: FormatKind): Promise<Response> {
  if (token === '') {
    return Response.json({ error: 'INVALID_TOKEN' }, { status: 401 });
  }

  // ponytail: Go 的進程內 10s/IP 限流已刪（方案 §5，多 isolate 下本就失效），由 CF WAF 規則承接
  const user = await c.env.DB.prepare('SELECT * FROM users WHERE client_token = ? LIMIT 1')
    .bind(token)
    .first<UserRow>();
  if (!user) {
    return Response.json({ error: 'INVALID_TOKEN' }, { status: 401 });
  }

  const status = user.subscription_status ?? '';
  if (status === 'expired') {
    return Response.json({ error: 'SUBSCRIPTION_EXPIRED' }, { status: 403 });
  }
  if (status === 'pending') {
    return Response.json({ error: 'SUBSCRIPTION_PENDING' }, { status: 403 });
  }

  const headers = subsHeaders(user);

  let cached: CachedBody | null = null;
  try {
    cached = await c.env.CACHE.get<CachedBody>(`sub:${token}:${format}`, 'json');
  } catch {
    // KV 失敗靜默穿透
  }
  if (cached) {
    return new Response(cached.body, { status: 200, headers: { ...headers, 'Content-Type': cached.ct } });
  }

  const nodes = await getActiveNodes(c.env);
  if (nodes.length === 0) {
    return new Response(null, { status: 204 });
  }

  const out = buildFormat(format, creds(user), nodes);
  if (!out) {
    return new Response(null, { status: 204 }); // v2ray：全部節點協議不可編碼
  }

  try {
    c.executionCtx.waitUntil(
      c.env.CACHE.put(`sub:${token}:${format}`, JSON.stringify(out), { expirationTtl: KV_TTL }).catch(() => {}),
    );
  } catch {
    // 無 executionCtx（測試環境）時放棄緩存
  }
  return new Response(out.body, { status: 200, headers: { ...headers, 'Content-Type': out.ct } });
}
