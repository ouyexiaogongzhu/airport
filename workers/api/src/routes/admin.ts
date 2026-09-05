// /admin/* + 公開 /products — 逐字移植 manager/internal/handler/{user,payment,stats,node,
// node_config,traffic,product}.go 的 admin 面。掛載點：/api/v1。
// 全部 WebAuth("admin_session") → AdminOnly；非 GET 再加 WebCSRF("admin_csrf")（對齊 main.go）。

import { Hono } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import { verifyJwt } from '../lib/jwt';
import { constantTimeEqual, randomHex } from '../lib/csrf';
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
    vless_uuid: u.vless_uuid,
    created_at: u.created_at,
    updated_at: u.updated_at,
  };
}

const USER_COLS =
  'id, username, role, balance, status, client_token, subscription_status, subscription_tier, ' +
  'traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start, ' +
  'vless_uuid, created_at, updated_at';

// nodeJson — 對齊 Go model.Node 的 JSON 形狀：token 是 json:"-"，其餘全輸出。
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
    network: n.network,
    security: n.security,
    ws_path: n.ws_path,
    server_name: n.server_name,
    reality_public_key: n.reality_public_key,
    reality_short_id: n.reality_short_id,
    last_heartbeat: n.last_heartbeat,
    created_at: n.created_at,
    updated_at: n.updated_at,
  };
}

const NODE_COLS =
  'id, name, type, address, port, protocol, status, traffic_up, traffic_down, user_id, ' +
  'network, security, ws_path, server_name, reality_public_key, reality_short_id, ' +
  'last_heartbeat, created_at, updated_at';

// userSetVersion — 對齊 Go userSetVersion（FNV-1a over uint64，回傳 int64 包裝值）
function userSetVersion(userIDs: number[]): number {
  let h = 14695981039346656037n;
  for (const id of userIDs) {
    h ^= BigInt(id);
    h *= 1099511628211n;
  }
  return Number(BigInt.asIntN(64, h));
}

function firstNonEmpty(a: unknown, b: string): string {
  const s = typeof a === 'string' ? a : '';
  return s !== '' ? s : b;
}

export function adminRoutes() {
  const app = new Hono<AppEnv>();

  // middleware.WebAuth("admin_session")：對齊 webauth.go，cookie 缺失/驗簽失敗 → 401 SESSION_EXPIRED
  const adminAuth = createMiddleware<AppEnv>(async (c, next) => {
    const secret = c.env.JWT_SECRET;
    const token = secret ? getCookie(c, 'admin_session') : undefined;
    if (!secret || !token) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    const claims = await verifyJwt(token, secret);
    if (!claims || typeof claims.user_id !== 'number') return c.json({ error: 'SESSION_EXPIRED' }, 401);
    c.set('userId', claims.user_id);
    c.set('username', claims.username);
    c.set('role', claims.role);
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

  // UpdateUser：client_token（空/缺 → 重產 "rf_"+hex32）+ status 白名單
  app.put('/admin/users/:id', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid user id' }, 400);
    const db = c.env.DB;
    const user = await db.prepare('SELECT id FROM users WHERE id = ?').bind(id).first<{ id: number }>();
    if (!user) return c.json({ error: 'user not found' }, 404);

    const body = await c.req.json<{ client_token?: unknown; status?: unknown }>().catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);

    const updates: Record<string, unknown> = {};
    if (typeof body.client_token === 'string' && body.client_token !== '') {
      updates.client_token = body.client_token;
    } else {
      updates.client_token = 'rf_' + randomHex(32);
    }
    if (body.status !== undefined) {
      const valid = new Set(['active', 'suspended', 'banned']);
      if (typeof body.status !== 'string' || !valid.has(body.status)) {
        return c.json({ error: "invalid status, must be 'active', 'suspended', or 'banned'" }, 400);
      }
      updates.status = body.status;
    }
    updates.updated_at = new Date().toISOString();

    const sets = ['client_token = ?', 'updated_at = ?'];
    const binds: unknown[] = [updates.client_token, updates.updated_at];
    if (updates.status !== undefined) {
      sets.push('status = ?');
      binds.push(updates.status);
    }
    binds.push(id);
    const r = await db
      .prepare(`UPDATE users SET ${sets.join(', ')} WHERE id = ?`)
      .bind(...binds)
      .run();
    if ((r.meta.changes ?? 0) === 0) return c.json({ error: 'failed to update user' }, 500);

    const fresh = await db.prepare(`SELECT ${USER_COLS} FROM users WHERE id = ?`).bind(id).first<Record<string, unknown>>();
    if (!fresh) return c.json({ error: 'user not found' }, 404);
    return c.json(adminUserJson(fresh));
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
      .json<{ name?: unknown; type?: unknown; address?: unknown; port?: unknown; protocol?: unknown; user_id?: unknown }>()
      .catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);

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
    const validProtocols = new Set(['vmess', 'vless', 'shadowsocks', 'trojan']);
    if (!validProtocols.has(protocol)) {
      return c.json({ error: 'protocol must be one of: vmess, vless, shadowsocks, trojan' }, 400);
    }
    const validTypes = new Set(['v2ray', 'xray']);
    if (!validTypes.has(type)) {
      return c.json({ error: 'type must be one of: v2ray, xray' }, 400);
    }

    const now = new Date().toISOString();
    const token = 'nd_' + randomHex(32);
    const ins = await c.env.DB.prepare(
      "INSERT INTO nodes (name, type, address, port, protocol, status, traffic_up, traffic_down, user_id, " +
        "network, security, token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'inactive', 0, 0, ?, 'ws', 'none', ?, ?, ?)",
    )
      .bind(name, type, address, port, protocol, Number(body.user_id ?? 0), token, now, now)
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
        network: 'ws',
        security: 'none',
        ws_path: null,
        server_name: null,
        reality_public_key: null,
        reality_short_id: null,
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

  // UpdateNode：name/type/address/port/protocol/status 指針欄位，有值才更新
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

    const updates: Record<string, unknown> = {};
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

  // GenerateNodeConfig：逐字移植 node_config.go BuildNodeXrayConfig（省略進程內 per-node
  // 快取 —— Workers isolate 間不共享記憶體，快取需 KV/DO，且 Go 快取僅是效能層不影響形狀）。
  app.get('/admin/nodes/:id/config', ...guard, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid node id' }, 400);
    const node = await c.env.DB.prepare(
      'SELECT id, address, port, protocol, status, network, security, ws_path, server_name, reality_short_id FROM nodes WHERE id = ?',
    ).bind(id).first<Record<string, unknown>>();
    if (!node) return c.json({ error: 'node not found' }, 404);
    if (node.status !== 'active') return c.json({ error: 'NODE_DISABLED' }, 403);

    const config = await buildNodeXrayConfig(c.env.DB, node);
    // Touch heartbeat（非致命；Go 用 UpdateColumn 避免 bump updated_at）
    await c.env.DB.prepare('UPDATE nodes SET last_heartbeat = ? WHERE id = ?')
      .bind(new Date().toISOString(), id)
      .run();

    return c.json(config);
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

  return app;
}

// buildNodeXrayConfig — 逐字移植 node_config.go buildNodeXrayConfigUncached
async function buildNodeXrayConfig(db: D1Database, node: Record<string, unknown>): Promise<Record<string, unknown>> {
  const nowUnix = Math.floor(Date.now() / 1000);
  const users = await db
    .prepare("SELECT id, vless_uuid FROM users WHERE subscription_status = 'active' AND (expire_time = 0 OR expire_time > ?) ORDER BY id")
    .bind(nowUnix)
    .all<{ id: number; vless_uuid: string | null }>();

  const clients: Record<string, unknown>[] = [];
  const userIDs: number[] = [];
  for (const u of users.results) {
    // ensureUserCredentials：vless_uuid 缺失時記憶體內補 UUIDv4（對齊 Go：此路徑不回寫 DB）
    const vlessUUID = u.vless_uuid || crypto.randomUUID();
    if (vlessUUID === '') continue;
    clients.push({ id: vlessUUID, flow: 'xtls-rprx-vision' });
    userIDs.push(u.id);
  }

  const address = String(node.address ?? '');
  const serverName = firstNonEmpty(node.server_name, address);

  const network = firstNonEmpty(node.network, 'tcp');
  const streamSettings: Record<string, unknown> = { network };
  switch (node.security) {
    case 'tls':
      streamSettings.security = 'tls';
      streamSettings.tlsSettings = {
        certificates: [{ certificateFile: '/etc/xray/tls.crt', keyFile: '/etc/xray/tls.key' }],
      };
      break;
    case 'reality':
      streamSettings.security = 'reality';
      streamSettings.realitySettings = {
        show: false,
        dest: `${serverName}:443`,
        xver: 0,
        serverNames: [serverName],
        privateKey: '', // filled by the node operator via env XRAY_REALITY_PRIVATE_KEY
        shortIds: [firstNonEmpty(node.reality_short_id, '')],
        maxTimeDiff: 0,
        minClientVer: '',
        maxClientVer: '',
        handshake: null,
        decryption: 'none',
        settings: null,
      };
      break;
    default:
      streamSettings.security = 'none';
  }
  if (network === 'ws') {
    const path = firstNonEmpty(node.ws_path, '/');
    streamSettings.wsSettings = { path, headers: { Host: serverName } };
  }

  const inbound = {
    tag: `in-${String(node.protocol)}`,
    port: node.port,
    protocol: node.protocol,
    settings: { clients, decryption: 'none', fallbacks: [] },
    streamSettings,
  };

  const config: Record<string, unknown> = {
    log: { loglevel: 'info', access: '/var/log/xray/access.log', error: '/var/log/xray/error.log' },
    inbounds: [inbound],
    outbounds: [{ protocol: 'freedom', tag: 'direct' }],
    routing: {
      domainStrategy: 'IPIfNonMatch',
      rules: [{ type: 'field', inboundTag: [`in-${String(node.protocol)}`], outboundTag: 'direct' }],
    },
    stats: {},
    policy: {
      levels: {
        '0': {
          handshake: 4,
          connIdle: 300,
          uplinkOnly: 1,
          downlinkOnly: 1,
          statsUserUplink: true,
          statsUserDownlink: true,
        },
      },
    },
  };
  config._meta = { node_id: node.id, user_ids: userIDs, version: userSetVersion(userIDs) };
  return config;
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
