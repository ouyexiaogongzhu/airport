// /user/* + /web/client-token* — 逐字移植 manager/internal/handler/user.go
// （GetProfile/UpdateProfile）、payment.go（CreateOrder/ListOrders/GetOrder）、
// subscription.go（GetClientToken/RegenerateClientToken）。
// 掛載點：/api/v1。WebAuth("session")；CSRF 僅 mutate 路由（對齊 main.go）。

import { Hono } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import { authenticateAccessCandidates } from '../lib/session';
import { constantTimeEqual } from '../lib/csrf';
import { sanitizedUser, isValidEmail, USER_PROFILE_COLS, type UserRow } from '../lib/user';
import { createPaymentURL } from '../lib/payments';
import { clearUserDevices } from '../lib/devices';
import type { Env } from '../index';

type AppEnv = { Bindings: Env; Variables: { userId: number; username: string; role: string } };

const PROFILE_FIELDS = ['username', 'email', 'phone', 'display_name', 'billing_address'] as const;
type ProfileField = (typeof PROFILE_FIELDS)[number];

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

const VALID_PROVIDERS = new Set(['mock', 'bepusdt', 'paypal']);

export function webRoutes() {
  const app = new Hono<AppEnv>();

  // middleware.WebAuth("session")（webauth.go）：cookie 缺失/驗簽失敗 → 401 SESSION_EXPIRED
  const webAuth = createMiddleware<AppEnv>(async (c, next) => {
    const secret = c.env.JWT_SECRET;
    // cookie 優先；失效時再試 Bearer（勿用 cookie||bearer：過期 Domain cookie 會擋住有效 Bearer）
    const bearer = c.req.header('Authorization')?.replace(/^Bearer /i, '');
    const r = await authenticateAccessCandidates(c.env.DB, secret, [
      secret ? getCookie(c, 'session') : undefined,
      bearer,
    ]);
    if (!('user' in r)) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    c.set('userId', r.user.id);
    c.set('username', r.user.username);
    c.set('role', r.user.role);
    await next();
  });

  // middleware.WebCSRF("csrf")（webauth.go）：非安全方法要求 X-CSRF-Token === cookie
  const webCsrf = createMiddleware<AppEnv>(async (c, next) => {
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
    const cookie = getCookie(c, 'csrf');
    if (!header || !cookie || !constantTimeEqual(header, cookie)) {
      return c.json({ error: 'CSRF_INVALID' }, 403);
    }
    await next();
  });

  // GetProfile
  app.get('/user/profile', webAuth, async (c) => {
    const user = await c.env.DB.prepare(`SELECT ${USER_PROFILE_COLS} FROM users WHERE id = ?`)
      .bind(c.get('userId'))
      .first<UserRow>();
    if (!user) return c.json({ error: 'user not found' }, 404);
    return c.json(sanitizedUser(user));
  });

  // UpdateProfile：allowlist username / email / phone / display_name / billing_address（不影響 username 登入）
  app.put('/user/profile', webAuth, webCsrf, async (c) => {
    const body = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);

    const updates: Partial<Record<ProfileField, string | null>> = {};
    for (const key of PROFILE_FIELDS) {
      if (!(key in body)) continue;
      const raw = body[key];
      if (raw === null) {
        updates[key] = null;
        continue;
      }
      if (typeof raw !== 'string') {
        return c.json({ error: `${key} must be a string or null` }, 400);
      }
      updates[key] = raw.trim();
    }
    if (Object.keys(updates).length === 0) {
      return c.json({ error: 'no valid fields to update' }, 400);
    }

    if ('username' in updates) {
      const username = updates.username ?? '';
      if (username === '' || username.length > 64) {
        return c.json({ error: 'username must be 1-64 characters' }, 400);
      }
      const taken = await c.env.DB.prepare('SELECT id FROM users WHERE username = ? AND id != ?')
        .bind(username, c.get('userId'))
        .first();
      if (taken) return c.json({ error: 'username already exists' }, 409);
    }

    if ('email' in updates) {
      const email = updates.email;
      if (email != null && email !== '' && !isValidEmail(email)) {
        return c.json({ error: 'invalid email format' }, 400);
      }
      // 空字串視為清除
      if (email === '') updates.email = null;
      if (updates.email) {
        const taken = await c.env.DB.prepare('SELECT id FROM users WHERE email = ? AND id != ?')
          .bind(updates.email, c.get('userId'))
          .first();
        if (taken) return c.json({ error: 'email already exists' }, 409);
      }
    }

    if ('phone' in updates) {
      const phone = updates.phone;
      if (phone != null && phone.length > 32) {
        return c.json({ error: 'phone must be at most 32 characters' }, 400);
      }
      if (phone === '') updates.phone = null;
    }

    if ('display_name' in updates) {
      const name = updates.display_name;
      if (name != null && name.length > 128) {
        return c.json({ error: 'display_name must be at most 128 characters' }, 400);
      }
      if (name === '') updates.display_name = null;
    }

    if ('billing_address' in updates) {
      const addr = updates.billing_address;
      if (addr != null && addr.length > 512) {
        return c.json({ error: 'billing_address must be at most 512 characters' }, 400);
      }
      if (addr === '') updates.billing_address = null;
    }

    const setParts: string[] = [];
    const binds: unknown[] = [];
    for (const key of PROFILE_FIELDS) {
      if (!(key in updates)) continue;
      setParts.push(`${key} = ?`);
      binds.push(updates[key] ?? null);
    }
    const now = new Date().toISOString();
    setParts.push('updated_at = ?');
    binds.push(now, c.get('userId'));

    const result = await c.env.DB.prepare(`UPDATE users SET ${setParts.join(', ')} WHERE id = ?`)
      .bind(...binds)
      .run();
    if ((result.meta.changes ?? 0) === 0) {
      return c.json({ error: 'failed to update profile' }, 500);
    }
    const user = await c.env.DB.prepare(`SELECT ${USER_PROFILE_COLS} FROM users WHERE id = ?`)
      .bind(c.get('userId'))
      .first<UserRow>();
    if (!user) return c.json({ error: 'user not found' }, 404);
    return c.json(sanitizedUser(user));
  });

  // CreateOrder：校驗產品 → 原子扣庫存 + 插 pending 訂單 → createPaymentURL → 存 payment_url
  app.post('/user/orders', webAuth, webCsrf, async (c) => {
    const db = c.env.DB;
    const body = await c.req.json<{ product_id?: unknown; provider?: unknown }>().catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);

    const productId = Number(body.product_id ?? 0);
    if (!Number.isInteger(productId) || productId <= 0) {
      return c.json({ error: 'product_id is required' }, 400);
    }

    const product = await db
      .prepare('SELECT id, name, price, stock, status, currency FROM products WHERE id = ?')
      .bind(productId)
      .first<{ id: number; name: string; price: number; stock: number; status: string; currency: string | null }>();
    if (!product) return c.json({ error: 'product not found' }, 404);
    if (product.status !== 'active') return c.json({ error: 'product is not available' }, 400);

    // PayPal 以 USD 計價：非 USD 產品拒走 paypal，否則 CNY 定價會以同數字美元扣款
    if (typeof body.provider === 'string' && body.provider === 'paypal' && product.currency !== 'USD') {
      return c.json({ error: 'product does not support paypal payment' }, 400);
    }

    const provider = typeof body.provider === 'string' && body.provider !== '' ? body.provider : 'mock';
    if (!VALID_PROVIDERS.has(provider)) {
      return c.json({ error: 'invalid payment provider' }, 400);
    }

    const userId = c.get('userId');
    const now = new Date().toISOString();

    // Go 事務（鎖行查庫存 → 插單 → stock-1）在 D1 拆為：帶 stock>0 條件的原子扣減，
    // changes==0 即缺貨；插單失敗則回補庫存。
    const dec = await db
      .prepare('UPDATE products SET stock = stock - 1, updated_at = ? WHERE id = ? AND stock > 0')
      .bind(now, productId)
      .run();
    if ((dec.meta.changes ?? 0) === 0) {
      return c.json({ error: 'product out of stock' }, 400);
    }

    let orderId: number | undefined;
    try {
      const ins = await db
        .prepare(
          "INSERT INTO orders (user_id, product_id, amount, status, provider, payment_url, created_at, updated_at) " +
            "VALUES (?, ?, ?, 'pending', ?, NULL, ?, ?)",
        )
        .bind(userId, productId, product.price, provider, now, now)
        .run();
      orderId = ins.meta.last_row_id;
    } catch {
      await db
        .prepare('UPDATE products SET stock = stock + 1 WHERE id = ?')
        .bind(productId)
        .run();
      return c.json({ error: 'failed to create order' }, 500);
    }

    // URL 構造對齊 Go CreateOrder（回調走 Worker 公開路由 /public/payment/callback/）
    const origin = new URL(c.req.url).origin;
    const notifyURL = `${origin}/api/v1/public/payment/callback/${provider}`;
    const redirectURL = `${origin}/user/orders/${orderId}`;

    let paymentURL: string;
    try {
      paymentURL = await createPaymentURL(
        { id: orderId, userId, amount: product.price },
        provider,
        c.env,
        notifyURL,
        redirectURL,
      );
    } catch {
      return c.json({ error: 'failed to create payment' }, 500);
    }

    const saved = await db
      .prepare('UPDATE orders SET payment_url = ?, updated_at = ? WHERE id = ?')
      .bind(paymentURL, now, orderId)
      .run();
    if ((saved.meta.changes ?? 0) === 0) {
      return c.json({ error: 'failed to save payment url' }, 500);
    }

    // Go model.Order JSON 形狀（GORM 欄位序不影響 JSON）
    return c.json(
      {
        order: {
          id: orderId,
          user_id: userId,
          product_id: productId,
          amount: product.price,
          status: 'pending',
          provider,
          payment_url: paymentURL,
          created_at: now,
          updated_at: now,
        },
        payment_url: paymentURL,
      },
      201,
    );
  });

  // ListOrders：created_at DESC + 分頁 {"data","total","page","per_page"}
  app.get('/user/orders', webAuth, async (c) => {
    const { offset, limit } = parsePagination(c);
    const userId = c.get('userId');
    const total = await db_count(
      c.env.DB,
      'SELECT COUNT(*) AS n FROM orders WHERE user_id = ?',
      userId,
    );
    const rs = await c.env.DB.prepare(
      'SELECT id, user_id, product_id, amount, status, provider, payment_url, created_at, updated_at ' +
        'FROM orders WHERE user_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?',
    )
      .bind(userId, limit, offset)
      .all<Record<string, unknown>>();
    return c.json({ data: rs.results, total, page: offset / limit + 1, per_page: limit });
  });

  // GetOrder
  app.get('/user/orders/:id', webAuth, async (c) => {
    const userId = c.get('userId');
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) {
      return c.json({ error: 'invalid order id' }, 400);
    }
    const order = await c.env.DB.prepare(
      'SELECT id, user_id, product_id, amount, status, provider, payment_url, created_at, updated_at ' +
        'FROM orders WHERE id = ?',
    )
      .bind(id)
      .first<{ id: number; user_id: number } & Record<string, unknown>>();
    if (!order) return c.json({ error: 'order not found' }, 404);
    if (order.user_id !== userId) return c.json({ error: 'forbidden' }, 403);
    return c.json(order);
  });

  // GetClientToken：脫敏 token（>12 字元 → 前7 + *** + 後4）
  app.get('/web/client-token', webAuth, async (c) => {
    const user = await c.env.DB.prepare('SELECT client_token, created_at FROM users WHERE id = ?')
      .bind(c.get('userId'))
      .first<{ client_token: string | null; created_at: string | null }>();
    if (!user) return c.json({ error: 'user not found' }, 404);
    if (!user.client_token) return c.json({ error: 'no client token' }, 404);
    const token = user.client_token;
    const masked = token.length > 12 ? `${token.slice(0, 7)}***${token.slice(-4)}` : token;
    const createdUnix = user.created_at ? Math.floor(Date.parse(user.created_at) / 1000) : 0;
    return c.json({ token: masked, created_at: Number.isFinite(createdUnix) ? createdUnix : 0 });
  });

  // RegenerateClientToken："rf_" + 32 隨機位元組 hex（crypto.getRandomValues，對齊 Go rand.Read）
  // 不變量：只改 client_token + updated_at。不得觸及 traffic_*、expire_time、vless_uuid、
  // token_version、subscription_* 等權益欄位（訂閱鏈接輪換 ≠ 重開通 / ≠ 換 UUID）。
  app.post('/web/client-token/regenerate', webAuth, webCsrf, async (c) => {
    const user = await c.env.DB.prepare('SELECT id FROM users WHERE id = ?')
      .bind(c.get('userId'))
      .first<{ id: number }>();
    if (!user) return c.json({ error: 'user not found' }, 404);
    const b = new Uint8Array(32);
    crypto.getRandomValues(b);
    const newToken = 'rf_' + Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
    const now = new Date().toISOString();
    const r = await c.env.DB.prepare(
      'UPDATE users SET client_token = ?, updated_at = ? WHERE id = ?',
    )
      .bind(newToken, now, user.id)
      .run();
    if ((r.meta.changes ?? 0) === 0) {
      return c.json({ error: 'failed to generate token' }, 500);
    }
    // New subscription URL → free previous device slots (clients must re-import).
    await clearUserDevices(c.env.DB, user.id);
    return c.json({ token: newToken });
  });

  return app;
}

async function db_count(db: D1Database, sql: string, ...binds: unknown[]): Promise<number> {
  const row = await db
    .prepare(sql)
    .bind(...binds)
    .first<{ n: number }>();
  return row?.n ?? 0;
}
