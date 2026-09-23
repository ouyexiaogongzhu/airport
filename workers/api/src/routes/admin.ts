// /admin/* + 公開 /products — 逐字移植 manager/internal/handler/{user,payment,stats,node,
// node_config,traffic,product}.go 的 admin 面。掛載點：/api/v1。
// 全部 WebAuth("admin_session") → AdminOnly；非 GET 再加 WebCSRF("admin_csrf")（對齊 main.go）。

import { Hono } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import { authenticateAccessCandidates } from '../lib/session';
import { constantTimeEqual, randomHex } from '../lib/csrf';
import { activationStatement, type ProductPlan } from '../lib/entitlement';
import { clearUserDevices } from '../lib/devices';
import { buildNodeXrayConfig, type NodeConfigRow } from '../lib/nodeconfig';
import type { Env } from '../index';

type AppEnv = { Bindings: Env; Variables: { userId: number; username: string; role: string } };

// parsePagination — 對齊 Go user.go parsePagination（page≥1、per_page 1..100，預設 20）
function parsePagination(c: { req: { query: (k: string) => string | undefined } }): { offset: number; limit: number } {
  let page = parseInt(c.req.query('page') ?? '', 10);
  let perPage = parseInt(c.req.query('per_page') ?? '', 10);
  if (!Number.isInteger(page)) page = 1;
  if (!Number.isInteger(perPage)) perPage = 20;
  if (page < 1) page = 1;
  if (perPage < 1 || perPage > 100) perPage = 20;
  return { offset: (page - 1) * perPage, limit: perPage };
}

async function db_count(db: D1Database, sql: string, ...binds: unknown[]): Promise<number> {
  const row = await db
    .prepare(sql)
    .bind(...binds)
    .first<{ n: number }>();
  return row?.n ?? 0;
}

// adminUserJson — 對齊 Go model.User 的 JSON 形狀：password_hash / ss_password /
// trojan_password 是 json:"-"，其餘（含 vless_uuid、client_token）全部輸出。
function adminUserJson(u: Record<string, unknown>): Record<string, unknown> {
  return {
    id: u.id,
    username: u.username,
    role: u.role,
    balance: u.balance,
    status: u.status,
    client_token: u.client_token,
    subscription_status: u.subscription_status,
    subscription_tier: u.subscription_tier,
    traffic_limit_bytes: u.traffic_limit_bytes,
    traffic_used_bytes: u.traffic_used_bytes,
    expire_time: u.expire_time,
    rate_limit_bps: u.rate_limit_bps,
    traffic_period_start: u.traffic_period_start,
    max_devices: u.max_devices,
    vless_uuid: u.vless_uuid,
    created_at: u.created_at,
    updated_at: u.updated_at,
  };
}

const USER_COLS =
  'id, username, role, balance, status, client_token, subscription_status, subscription_tier, ' +
  'traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start, ' +
  'max_devices, vless_uuid, created_at, updated_at';

// nodeJson — token 不輸出。節點固定為 Tunnel 形態：address = 節點域名，port = 本機 Xray 端口；
// network/security/server_name/reality_* 列已停用，不再輸出
function nodeJson(n: Record<string, unknown>): Record<string, unknown> {
  return {
    id: n.id,
    name: n.name,
    type: n.type,
    address: n.address,
    port: n.port,
    protocol: n.protocol,
    status: n.status,
    traffic_up: n.traffic_up,
    traffic_down: n.traffic_down,
    user_id: n.user_id,
    ws_path: n.ws_path,
    last_heartbeat: n.last_heartbeat,
    created_at: n.created_at,
    updated_at: n.updated_at,
  };
}

const VALID_PROTOCOLS = new Set(['vmess', 'vless']);
const PROTOCOL_ERROR = 'protocol must be one of: vmess, vless';

// 傳輸層只剩 ws_path 可配；只收到時返回，'' 存為 NULL（即 "/"）
function parseTransport(body: Record<string, unknown>): { fields: Record<string, unknown> } | { error: string } {
  const fields: Record<string, unknown> = {};
  const v = body.ws_path;
  if (v !== undefined) {
    if (v !== null && typeof v !== 'string') return { error: 'ws_path must be a string' };
    const s = typeof v === 'string' ? v.trim() : '';
    if (s !== '' && !s.startsWith('/')) return { error: 'ws_path must start with /' };
    fields.ws_path = s === '' ? null : s;
  }
  return { fields };
}

const NODE_COLS =
  'id, name, type, address, port, protocol, status, traffic_up, traffic_down, user_id, ws_path, ' +
  'last_heartbeat, created_at, updated_at';

function isNonNegInt(v: unknown): v is number {
  return typeof v === 'number' && Number.isSafeInteger(v) && v >= 0;
}

const PRODUCT_COLS =
  'id, name, type, price, stock, status, currency, duration_days, traffic_bytes, speed_limit_bps, max_devices, description, created_at, updated_at';

// 商品權益欄位：只校驗收到的鍵
function parseProductPlan(req: Record<string, unknown>): { fields: Record<string, unknown> } | { error: string } {
  const fields: Record<string, unknown> = {};
  if (req.duration_days !== undefined) {
    if (!isNonNegInt(req.duration_days) || req.duration_days === 0) return { error: 'duration_days must be a positive integer' };
    fields.duration_days = req.duration_days;
  }
  for (const key of ['traffic_bytes', 'speed_limit_bps', 'max_devices'] as const) {
    if (req[key] === undefined) continue;
    if (!isNonNegInt(req[key])) return { error: `${key} must be a non-negative integer` };
    fields[key] = req[key];
  }
  if (req.currency !== undefined) {
    if (req.currency !== 'USD' && req.currency !== 'CNY') return { error: 'currency must be one of: USD, CNY' };
    fields.currency = req.currency;
  }
  if (req.description !== undefined) {
    if (req.description !== null && typeof req.description !== 'string') return { error: 'description must be a string' };
    fields.description = req.description || null;
  }
  return { fields };
}

export function adminRoutes() {
  const app = new Hono<AppEnv>();

  // middleware.WebAuth("admin_session")：對齊 webauth.go，cookie 缺失/驗簽失敗 → 401 SESSION_EXPIRED
  const adminAuth = createMiddleware<AppEnv>(async (c, next) => {
    const secret = c.env.JWT_SECRET;
    // cookie 優先；失效時再試 Bearer（勿用 cookie||bearer：過期 Domain cookie 會擋住有效 Bearer）
    const bearer = c.req.header('Authorization')?.replace(/^Bearer /i, '');
    const r = await authenticateAccessCandidates(c.env.DB, secret, [
      secret ? getCookie(c, 'admin_session') : undefined,
      bearer,
    ]);
    if (!('user' in r)) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    c.set('userId', r.user.id);
    c.set('username', r.user.username);
    c.set('role', r.user.role);
    await next();
  });

  // middleware.AdminOnly：role !== "admin" → 403
  const adminOnly = createMiddleware<AppEnv>(async (c, next) => {
    if (c.get('role') !== 'admin') return c.json({ error: 'admin access required' }, 403);
    await next();
  });

  // middleware.WebCSRF("admin_csrf")：非安全方法要求 X-CSRF-Token === cookie
  const adminCsrf = createMiddleware<AppEnv>(async (c, next) => {
    if (c.req.method === 'GET' || c.req.method === 'HEAD' || c.req.method === 'OPTIONS') {
      await next();
      return;
    }
    // Bearer 認證不依賴 cookie，天然免疫 CSRF → 跳過雙提交
    if (c.req.header('Authorization')) {
      await next();
      return;
    }
    const header = c.req.header('X-CSRF-Token');
    const cookie = getCookie(c, 'admin_csrf');
    if (!header || !cookie || !constantTimeEqual(header, cookie)) {
      return c.json({ error: 'CSRF_INVALID' }, 403);
    }
    await next();
  });

  const guard = [adminAuth, adminOnly] as const;

  // ── Users（user.go ListUsers/GetUser/UpdateUser）───────────────────────────

  // ListUsers：{"data","total","page","per_page"}
  app.get('/admin/users', ...guard, async (c) => {
    const { offset, limit } = parsePagination(c);
    const total = await db_count(c.env.DB, 'SELECT COUNT(*) AS n FROM users');
    const rs = await c.env.DB.prepare(`SELECT ${USER_COLS} FROM users ORDER BY id LIMIT ? OFFSET ?`)
      .bind(limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ data: rs.results.map(adminUserJson), total, page: offset / limit + 1, per_page: limit });
  });

  // GetUser
  app.get('/admin/users/:id', ...guard, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid user id' }, 400);
    const user = await c.env.DB.prepare(`SELECT ${USER_COLS} FROM users WHERE id = ?`)
      .bind(id)
      .first<Record<string, unknown>>();
    if (!user) return c.json({ error: 'user not found' }, 404);
    return c.json(adminUserJson(user));
  });

  // UpdateUser：status 白名單；client_token 只在顯式給值或 regenerate_token=true 時變更
  // （改狀態不能順帶換 token，否則封禁/解封會讓用戶訂閱鏈接失效）
  app.put('/admin/users/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid user id' }, 400);
    const db = c.env.DB;
    const user = await db.prepare('SELECT id FROM users WHERE id = ?').bind(id).first<{ id: number }>();
    if (!user) return c.json({ error: 'user not found' }, 404);

    const body = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (body === null || typeof body !== 'object') return c.json({ error: 'invalid request body' }, 400);

    const sets: string[] = [];
    const binds: unknown[] = [];
    let regeneratedToken = false;
    if (typeof body.client_token === 'string' && body.client_token !== '') {
      sets.push('client_token = ?');
      binds.push(body.client_token);
      regeneratedToken = true;
    } else if (body.regenerate_token === true) {
      sets.push('client_token = ?');
      binds.push('rf_' + randomHex(32));
      regeneratedToken = true;
    }
    if (body.status !== undefined) {
      const valid = new Set(['active', 'suspended', 'banned']);
      if (typeof body.status !== 'string' || !valid.has(body.status)) {
        return c.json({ error: "invalid status, must be 'active', 'suspended', or 'banned'" }, 400);
      }
      sets.push('status = ?');
      binds.push(body.status);
      // 封禁/停用即吊銷該用戶全部會話
      if (body.status !== 'active') sets.push('token_version = token_version + 1');
    }
    if (body.subscription_status !== undefined) {
      if (!['active', 'pending', 'expired'].includes(body.subscription_status as string)) {
        return c.json({ error: "invalid subscription_status, must be 'active', 'pending', or 'expired'" }, 400);
      }
      sets.push('subscription_status = ?');
      binds.push(body.subscription_status);
    }
    for (const key of ['expire_time', 'traffic_limit_bytes', 'traffic_used_bytes', 'rate_limit_bps', 'max_devices'] as const) {
      const v = body[key];
      if (v === undefined) continue;
      if (!isNonNegInt(v)) return c.json({ error: `${key} must be a non-negative integer` }, 400);
      sets.push(`${key} = ?`);
      binds.push(v);
    }
    if (sets.length === 0) return c.json({ error: 'no valid fields to update' }, 400);
    sets.push('updated_at = ?');
    binds.push(new Date().toISOString(), id);
    const r = await db
      .prepare(`UPDATE users SET ${sets.join(', ')} WHERE id = ?`)
      .bind(...binds)
      .run();
    if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'failed to update user' }, 500);
    if (regeneratedToken) await clearUserDevices(db, id);

    const fresh = await db.prepare(`SELECT ${USER_COLS} FROM users WHERE id = ?`).bind(id).first<Record<string, unknown>>();
    if (!fresh) return c.json({ error: 'user not found' }, 404);
    return c.json(adminUserJson(fresh));
  });

  // 手動開通：按商品配置開通/續費（不建訂單，不動庫存）
  app.post('/admin/users/:id/grant', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid user id' }, 400);
    const body = await c.req.json<{ product_id?: unknown }>().catch(() => null);
    const productId = Number(body?.product_id);
    if (!Number.isInteger(productId) || productId <= 0) return c.json({ error: 'product_id is required' }, 400);
    const db = c.env.DB;
    const plan = await db
      .prepare('SELECT name, duration_days, traffic_bytes, speed_limit_bps, max_devices FROM products WHERE id = ?')
      .bind(productId)
      .first<ProductPlan>();
    if (!plan) return c.json({ error: 'product not found' }, 404);
    const r = await activationStatement(db, id, plan, Math.floor(Date.now() / 1000)).run();
    if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'user not found' }, 404);
    const fresh = await db.prepare(`SELECT ${USER_COLS} FROM users WHERE id = ?`).bind(id).first<Record<string, unknown>>();
    return c.json(adminUserJson(fresh!));
  });

  // ── Orders（payment.go AdminListOrders/AdminGetOrder/AdminRefundOrder）─────

  const ORDER_COLS =
    'o.id, o.user_id, o.product_id, o.amount, o.status, o.provider, o.payment_url, o.created_at, o.updated_at';

  // AdminListOrders：JOIN users/products，status + search 過濾，{"data","total","page","per_page"}
  app.get('/admin/orders', ...guard, async (c) => {
    const { offset, limit } = parsePagination(c);
    const status = c.req.query('status') ?? '';
    const search = c.req.query('search') ?? '';

    const conds: string[] = [];
    const binds: unknown[] = [];
    if (status !== '') {
      conds.push('o.status = ?');
      binds.push(status);
    }
    if (search !== '') {
      conds.push('(o.id = ? OR u.username LIKE ?)');
      binds.push(search, `%${search}%`);
    }
    const where = conds.length > 0 ? ` WHERE ${conds.join(' AND ')}` : '';

    const total = await db_count(
      c.env.DB,
      'SELECT COUNT(*) AS n FROM orders o LEFT JOIN users u ON u.id = o.user_id' + where,
      ...binds,
    );
    const rs = await c.env.DB.prepare(
      `SELECT ${ORDER_COLS}, u.username AS username, p.name AS product_name ` +
        'FROM orders o LEFT JOIN users u ON u.id = o.user_id LEFT JOIN products p ON p.id = o.product_id' +
        where +
        ' ORDER BY o.created_at DESC LIMIT ? OFFSET ?',
    )
      .bind(...binds, limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ data: rs.results, total, page: offset / limit + 1, per_page: limit });
  });

  // AdminGetOrder：訂單 + 內嵌 product 物件（對齊 orderWithProduct Preload）
  app.get('/admin/orders/:id', ...guard, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid order id' }, 400);
    const db = c.env.DB;
    const order = await db
      .prepare(`SELECT ${ORDER_COLS} FROM orders o WHERE o.id = ?`)
      .bind(id)
      .first<Record<string, unknown>>();
    if (!order) return c.json({ error: 'order not found' }, 404);
    const product = await db
      .prepare('SELECT id, name, type, price, stock, status, currency, created_at, updated_at FROM products WHERE id = ?')
      .bind(order.product_id)
      .first<Record<string, unknown>>();
    return c.json({ ...order, product: product ?? null });
  });

  // AdminRefundOrder：paid → refunded + 商品庫存 +1
  app.post('/admin/orders/:id/refund', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid order id' }, 400);
    const db = c.env.DB;
    const order = await db
      .prepare(`SELECT ${ORDER_COLS} FROM orders o WHERE o.id = ?`)
      .bind(id)
      .first<Record<string, unknown>>();
    if (!order) return c.json({ error: 'order not found' }, 404);
    if (order.status !== 'paid') return c.json({ error: 'can only refund paid orders' }, 400);

    const now = new Date().toISOString();
    const upd = await db
      .prepare("UPDATE orders SET status = 'refunded', updated_at = ? WHERE id = ?")
      .bind(now, id)
      .run();
    if ((upd.meta.changes ?? 0) === 0) return c.json({ error: 'failed to refund order' }, 500);
    const stock = await db
      .prepare('UPDATE products SET stock = stock + 1, updated_at = ? WHERE id = ?')
      .bind(now, order.product_id)
      .run();
    if ((stock.meta.changes ?? 0) === 0) return c.json({ error: 'failed to restore product stock' }, 500);

    return c.json({
      message: 'order refunded',
      order: { ...order, status: 'refunded', updated_at: now },
    });
  });

  // ── Stats（stats.go GetAdminStats）─────────────────────────────────────────

  app.get('/admin/stats', ...guard, async (c) => {
    const db = c.env.DB;
    const now = new Date();
    const nowUnix = Math.floor(now.getTime() / 1000);
    const monthStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1)).toISOString();
    const heartbeatCutoff = new Date(now.getTime() - 5 * 60 * 1000).toISOString();

    const [totalUsers, activeUsers, activeOrders, totalProducts, totalNodes, onlineNodes] = await Promise.all([
      db_count(db, 'SELECT COUNT(*) AS n FROM users'),
      db_count(
        db,
        'SELECT COUNT(*) AS n FROM users WHERE subscription_status = ? AND (expire_time = 0 OR expire_time > ?)',
        'active',
        nowUnix,
      ),
      db_count(db, 'SELECT COUNT(*) AS n FROM orders WHERE status = ?', 'paid'),
      db_count(db, 'SELECT COUNT(*) AS n FROM products'),
      db_count(db, 'SELECT COUNT(*) AS n FROM nodes'),
      db_count(
        db,
        'SELECT COUNT(*) AS n FROM nodes WHERE last_heartbeat >= ? OR status = ?',
        heartbeatCutoff,
        'active',
      ),
    ]);

    const revenue = await db
      .prepare("SELECT COALESCE(SUM(amount), 0) AS v FROM orders WHERE status = 'paid' AND created_at >= ?")
      .bind(monthStart)
      .first<{ v: number }>();
    const nodeTraffic = await db
      .prepare('SELECT COALESCE(SUM(traffic_up), 0) AS up, COALESCE(SUM(traffic_down), 0) AS down FROM nodes')
      .first<{ up: number; down: number }>();

    // Traffic trend：近 7 天，按日分桶（"MM-DD"，對齊 Go strftime('%m-%d', ...)）
    const trendStart = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()) - 6 * 86400000)
      .toISOString();
    const points: { day: string; upload: number; download: number }[] = [];
    for (let i = 0; i < 7; i++) {
      const d = new Date(Date.parse(trendStart) + i * 86400000);
      points.push({
        day: `${String(d.getUTCMonth() + 1).padStart(2, '0')}-${String(d.getUTCDate()).padStart(2, '0')}`,
        upload: 0,
        download: 0,
      });
    }
    const trendRows = await db
      .prepare(
        'SELECT substr(recorded_at, 6, 5) AS day, SUM(upload_bytes) AS upload, SUM(download_bytes) AS download ' +
          'FROM traffic_records WHERE recorded_at >= ? GROUP BY substr(recorded_at, 6, 5)',
      )
      .bind(trendStart)
      .all<{ day: string; upload: number; download: number }>();
    const byDay = new Map(points.map((p) => [p.day, p]));
    for (const r of trendRows.results) {
      const p = byDay.get(r.day);
      if (p) {
        p.upload = r.upload;
        p.download = r.download;
      }
    }

    const recent = await db
      .prepare(
        `SELECT ${ORDER_COLS}, u.username AS username FROM orders o ` +
          'LEFT JOIN users u ON u.id = o.user_id ORDER BY o.created_at DESC LIMIT 5',
      )
      .all<Record<string, unknown>>();

    return c.json({
      total_users: totalUsers,
      active_users: activeUsers,
      active_orders: activeOrders,
      total_products: totalProducts,
      revenue_mtd: revenue?.v ?? 0,
      total_nodes: totalNodes,
      online_nodes: onlineNodes,
      node_traffic_up: nodeTraffic?.up ?? 0,
      node_traffic_down: nodeTraffic?.down ?? 0,
      traffic_trend: points,
      recent_orders: recent.results,
    });
  });

  // ── Nodes CRUD（node.go）──────────────────────────────────────────────────

  // CreateNode
  app.post('/admin/nodes', ...guard, adminCsrf, async (c) => {
    const body = await c.req
      .json<Record<string, unknown>>()
      .catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);
    const transport = parseTransport(body);
    if ('error' in transport) return c.json({ error: transport.error }, 400);

    const name = typeof body.name === 'string' ? body.name : '';
    const type = typeof body.type === 'string' ? body.type : '';
    const address = typeof body.address === 'string' ? body.address : '';
    const protocol = typeof body.protocol === 'string' ? body.protocol : '';
    const port = Number(body.port ?? 0);
    if (name === '' || type === '' || address === '' || port === 0 || protocol === '') {
      return c.json({ error: 'name, type, address, port and protocol are required' }, 400);
    }
    if (!Number.isInteger(port) || port < 1 || port > 65535) {
      return c.json({ error: 'port must be between 1 and 65535' }, 400);
    }
    if (!VALID_PROTOCOLS.has(protocol)) return c.json({ error: PROTOCOL_ERROR }, 400);
    const validTypes = new Set(['v2ray', 'xray']);
    if (!validTypes.has(type)) {
      return c.json({ error: 'type must be one of: v2ray, xray' }, 400);
    }

    const t = { ws_path: null, ...transport.fields };
    const now = new Date().toISOString();
    const token = 'nd_' + randomHex(32);
    const ins = await c.env.DB.prepare(
      "INSERT INTO nodes (name, type, address, port, protocol, status, traffic_up, traffic_down, user_id, " +
        'network, security, ws_path, token, created_at, updated_at) ' +
        "VALUES (?, ?, ?, ?, ?, 'inactive', 0, 0, ?, 'ws', 'none', ?, ?, ?, ?)",
    )
      .bind(name, type, address, port, protocol, Number(body.user_id ?? 0), t.ws_path, token, now, now)
      .run();
    if ((ins.meta.changes ?? 0) === 0) return c.json({ error: 'failed to create node' }, 500);

    return c.json(
      nodeJson({
        id: ins.meta.last_row_id,
        name,
        type,
        address,
        port,
        protocol,
        status: 'inactive',
        traffic_up: 0,
        traffic_down: 0,
        user_id: Number(body.user_id ?? 0),
        ...t,
        last_heartbeat: null,
        created_at: now,
        updated_at: now,
      }),
      201,
    );
  });

  // ListNode：可選 status 過濾 + 分頁
  app.get('/admin/nodes', ...guard, async (c) => {
    const { offset, limit } = parsePagination(c);
    const status = c.req.query('status') ?? '';
    const where = status !== '' ? ' WHERE status = ?' : '';
    const binds: unknown[] = status !== '' ? [status] : [];
    const total = await db_count(c.env.DB, 'SELECT COUNT(*) AS n FROM nodes' + where, ...binds);
    const rs = await c.env.DB.prepare(`SELECT ${NODE_COLS} FROM nodes${where} ORDER BY id LIMIT ? OFFSET ?`)
      .bind(...binds, limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ data: rs.results.map(nodeJson), total, page: offset / limit + 1, per_page: limit });
  });

  // GetNode
  app.get('/admin/nodes/:id', ...guard, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const node = await c.env.DB.prepare(`SELECT ${NODE_COLS} FROM nodes WHERE id = ?`)
      .bind(id)
      .first<Record<string, unknown>>();
    if (!node) return c.json({ error: 'node not found' }, 404);
    return c.json(nodeJson(node));
  });

  // UpdateNode：基本欄位 + 傳輸層欄位，有值才更新
  app.put('/admin/nodes/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const db = c.env.DB;
    const node = await db.prepare(`SELECT ${NODE_COLS} FROM nodes WHERE id = ?`).bind(id).first<Record<string, unknown>>();
    if (!node) return c.json({ error: 'node not found' }, 404);

    const body = await c.req
      .json<Record<string, unknown>>()
      .catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);
    const transport = parseTransport(body);
    if ('error' in transport) return c.json({ error: transport.error }, 400);

    const updates: Record<string, unknown> = { ...transport.fields };
    if (body.protocol !== undefined && !VALID_PROTOCOLS.has(String(body.protocol))) {
      return c.json({ error: PROTOCOL_ERROR }, 400);
    }
    for (const key of ['name', 'type', 'address', 'protocol', 'status'] as const) {
      if (body[key] !== undefined) updates[key] = body[key];
    }
    if (body.port !== undefined) updates.port = Number(body.port);
    if (Object.keys(updates).length > 0) {
      updates.updated_at = new Date().toISOString();
      const sets = Object.keys(updates)
        .map((k) => `${k} = ?`)
        .join(', ');
      const r = await db
        .prepare(`UPDATE nodes SET ${sets} WHERE id = ?`)
        .bind(...Object.values(updates), id)
        .run();
      if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'failed to update node' }, 500);
    }

    const fresh = await db.prepare(`SELECT ${NODE_COLS} FROM nodes WHERE id = ?`).bind(id).first<Record<string, unknown>>();
    if (!fresh) return c.json({ error: 'node not found' }, 404);
    return c.json(nodeJson(fresh));
  });

  // DeleteNode
  app.delete('/admin/nodes/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const r = await c.env.DB.prepare('DELETE FROM nodes WHERE id = ?').bind(id).run();
    if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'node not found' }, 404);
    return c.json({ message: 'node deleted' });
  });

  // GenerateNodeToken：輪換 daemon token（nd_ + hex32）
  app.post('/admin/nodes/:id/token', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const db = c.env.DB;
    const node = await db.prepare('SELECT id FROM nodes WHERE id = ?').bind(id).first<{ id: number }>();
    if (!node) return c.json({ error: 'node not found' }, 404);
    const token = 'nd_' + randomHex(32);
    const r = await db.prepare('UPDATE nodes SET token = ?, updated_at = ? WHERE id = ?')
      .bind(token, new Date().toISOString(), id)
      .run();
    if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'failed to save token' }, 500);
    // ponytail: Go 版這裡呼叫 middleware.InvalidateNodeToken 清進程內 token 快取；
    // Workers 無進程內節點 token 快取（daemon 走 D1 直查），無需失效。
    return c.json({ token });
  });

  // 預覽節點 Xray 配置（與 daemon 拉取的 config 相同；只讀，不記心跳）
  app.get('/admin/nodes/:id/config', ...guard, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const node = await c.env.DB.prepare('SELECT id, port, protocol, ws_path, status FROM nodes WHERE id = ?')
      .bind(id)
      .first<NodeConfigRow>();
    if (!node) return c.json({ error: 'node not found' }, 404);
    if (node.status !== 'active') return c.json({ error: 'NODE_DISABLED' }, 403);
    return c.json(await buildNodeXrayConfig(c.env.DB, node, Math.floor(Date.now() / 1000)));
  });

  // ── Traffic（traffic.go）──────────────────────────────────────────────────

  // ReportTraffic：記錄數據點 + 節點/用戶累計計數（原子 SQL 遞增）
  app.post('/admin/traffic/report', ...guard, adminCsrf, async (c) => {
    const body = await c.req
      .json<{ node_id?: unknown; user_id?: unknown; upload_bytes?: unknown; download_bytes?: unknown }>()
      .catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);

    const nodeId = Number(body.node_id ?? 0);
    const userId = Number(body.user_id ?? 0);
    const up = Number(body.upload_bytes ?? 0);
    const down = Number(body.download_bytes ?? 0);
    if (nodeId === 0 || userId === 0) return c.json({ error: 'node_id and user_id are required' }, 400);

    const db = c.env.DB;
    const node = await db.prepare('SELECT id FROM nodes WHERE id = ?').bind(nodeId).first<{ id: number }>();
    if (!node) return c.json({ error: 'node not found' }, 404);

    const recordedAt = new Date().toISOString();
    const ins = await db
      .prepare('INSERT INTO traffic_records (node_id, user_id, upload_bytes, download_bytes, recorded_at) VALUES (?, ?, ?, ?, ?)')
      .bind(nodeId, userId, up, down, recordedAt)
      .run();
    if ((ins.meta.changes ?? 0) === 0) return c.json({ error: 'failed to record traffic' }, 500);

    const nodeUpd = await db
      .prepare('UPDATE nodes SET traffic_up = traffic_up + ?, traffic_down = traffic_down + ? WHERE id = ?')
      .bind(up, down, nodeId)
      .run();
    if ((nodeUpd.meta.changes ?? 0) === 0) return c.json({ error: 'failed to update node traffic counters' }, 500);

    const userUpd = await db
      .prepare('UPDATE users SET traffic_used_bytes = traffic_used_bytes + ? WHERE id = ?')
      .bind(up + down, userId)
      .run();
    if ((userUpd.meta.changes ?? 0) === 0) return c.json({ error: 'failed to update user traffic counters' }, 500);

    return c.json(
      { id: ins.meta.last_row_id, node_id: nodeId, user_id: userId, upload_bytes: up, download_bytes: down, recorded_at: recordedAt },
      201,
    );
  });

  // GetTrafficStats：user_id/node_id/since/until 過濾，按 node+user 聚合 {"data":[...]}
  app.get('/admin/traffic/stats', ...guard, async (c) => {
    const conds: string[] = [];
    const binds: unknown[] = [];
    const nodeId = parseInt(c.req.query('node_id') ?? '', 10);
    const userId = parseInt(c.req.query('user_id') ?? '', 10);
    if (Number.isInteger(nodeId) && nodeId > 0) {
      conds.push('node_id = ?');
      binds.push(nodeId);
    }
    if (Number.isInteger(userId) && userId > 0) {
      conds.push('user_id = ?');
      binds.push(userId);
    }
    for (const [key, op] of [['since', '>='], ['until', '<=']] as const) {
      const raw = c.req.query(key) ?? '';
      if (raw === '') continue;
      const t = Date.parse(raw); // RFC3339
      if (Number.isFinite(t)) {
        conds.push(`recorded_at ${op} ?`);
        binds.push(new Date(t).toISOString());
      }
    }
    const where = conds.length > 0 ? ` WHERE ${conds.join(' AND ')}` : '';
    const rs = await c.env.DB.prepare(
      'SELECT node_id, user_id, COALESCE(SUM(upload_bytes), 0) AS total_upload, ' +
        'COALESCE(SUM(download_bytes), 0) AS total_download FROM traffic_records' +
        where +
        ' GROUP BY node_id, user_id',
    )
      .bind(...binds)
      .all<Record<string, unknown>>();
    return c.json({ data: rs.results });
  });

  // ── Products CRUD（product.go ListProducts/CreateProduct/UpdateProduct/DeleteProduct）──
  // 主線補缺：首輪遺漏；刪除=歸檔（status→archived），契約對齊 Go

  app.get('/admin/products', ...guard, async (c) => {
    const { offset, limit } = parsePagination(c);
    const total = await db_count(c.env.DB, 'SELECT COUNT(*) AS n FROM products');
    const rs = await c.env.DB.prepare(
      'SELECT id, name, type, price, stock, status, currency, created_at, updated_at FROM products ORDER BY id ASC LIMIT ? OFFSET ?',
    )
      .bind(limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ products: rs.results, total, page: offset / limit + 1, per_page: limit });
  });

  app.post('/admin/products', ...guard, adminCsrf, async (c) => {
    const req = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (!req) return c.json({ error: 'invalid request body' }, 400);
    const name = typeof req.name === 'string' ? req.name : '';
    const price = typeof req.price === 'number' ? req.price : 0;
    if (!name || price <= 0) return c.json({ error: 'name and price are required' }, 400);
    const validTypes = ['subscription', 'monthly', 'quarterly', 'half-yearly', 'yearly', 'one-time', 'trial'];
    const type = typeof req.type === 'string' ? req.type : '';
    if (type && !validTypes.includes(type)) {
      return c.json(
        { error: 'type must be one of: subscription, monthly, quarterly, half-yearly, yearly, one-time, trial' },
        400,
      );
    }
    const validStatuses = ['active', 'inactive', 'archived'];
    const status = typeof req.status === 'string' ? req.status : '';
    if (status && !validStatuses.includes(status)) {
      return c.json({ error: 'status must be one of: active, inactive, archived' }, 400);
    }
    const plan = parseProductPlan(req);
    if ('error' in plan) return c.json({ error: plan.error }, 400);
    const now = new Date().toISOString();
    const stock = typeof req.stock === 'number' ? Math.trunc(req.stock) : 0;
    const extra = Object.entries(plan.fields);
    const cols = ['name', 'type', 'price', 'stock', 'status', 'created_at', 'updated_at', ...extra.map(([k]) => k)];
    const rs = await c.env.DB.prepare(
      `INSERT INTO products (${cols.join(', ')}) VALUES (${cols.map(() => '?').join(', ')})`,
    )
      .bind(name, type, price, stock, status || 'active', now, now, ...extra.map(([, v]) => v))
      .run();
    const product = await c.env.DB.prepare(`SELECT ${PRODUCT_COLS} FROM products WHERE id = ?`).bind(rs.meta.last_row_id).first();
    return c.json({ product }, 201);
  });

  app.put('/admin/products/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id)) return c.json({ error: 'invalid product id' }, 400);
    const product = await c.env.DB.prepare('SELECT * FROM products WHERE id = ?').bind(id).first<Record<string, unknown>>();
    if (!product) return c.json({ error: 'product not found' }, 404);
    const req = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (!req) return c.json({ error: 'invalid request body' }, 400);
    const updates: string[] = [];
    const binds: unknown[] = [];
    if (req.name !== undefined) { updates.push('name = ?'); binds.push(req.name); }
    if (req.type !== undefined) { updates.push('type = ?'); binds.push(req.type); }
    if (req.price !== undefined) { updates.push('price = ?'); binds.push(req.price); }
    if (req.stock !== undefined) { updates.push('stock = ?'); binds.push(Math.trunc(Number(req.stock))); }
    if (req.status !== undefined) { updates.push('status = ?'); binds.push(req.status); }
    const plan = parseProductPlan(req);
    if ('error' in plan) return c.json({ error: plan.error }, 400);
    for (const [k, v] of Object.entries(plan.fields)) { updates.push(`${k} = ?`); binds.push(v); }
    if (updates.length === 0) return c.json({ product });
    updates.push('updated_at = ?');
    binds.push(new Date().toISOString(), id);
    await c.env.DB.prepare(`UPDATE products SET ${updates.join(', ')} WHERE id = ?`).bind(...binds).run();
    const updated = await c.env.DB.prepare(`SELECT ${PRODUCT_COLS} FROM products WHERE id = ?`).bind(id).first();
    return c.json({ product: updated });
  });

  app.delete('/admin/products/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id)) return c.json({ error: 'invalid product id' }, 400);
    const product = await c.env.DB.prepare('SELECT * FROM products WHERE id = ?').bind(id).first<Record<string, unknown>>();
    if (!product) return c.json({ error: 'product not found' }, 404);
    // Go DeleteProduct：歸檔而非物理刪除
    await c.env.DB.prepare("UPDATE products SET status = 'archived', updated_at = ? WHERE id = ?")
      .bind(new Date().toISOString(), id)
      .run();
    return c.json({ message: 'product archived' });
  });

  return app;
}

// publicProductRoutes — GET /products（product.go ListActiveProducts，無鑑權）
export function publicProductRoutes() {
  const app = new Hono<{ Bindings: Env }>();

  app.get('/products', async (c) => {
    const { offset, limit } = parsePagination(c);
    const total = await db_count(c.env.DB, "SELECT COUNT(*) AS n FROM products WHERE status = 'active'");
    const rs = await c.env.DB.prepare(
      "SELECT id, name, type, price, stock, status, currency, created_at, updated_at FROM products " +
        "WHERE status = 'active' ORDER BY id ASC LIMIT ? OFFSET ?",
    )
      .bind(limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ products: rs.results, total, page: offset / limit + 1, per_page: limit });
  });

  return app;
}
