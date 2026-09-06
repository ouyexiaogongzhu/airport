// /auth/* + /admin/auth/* — 逐字移植 manager/internal/handler/auth.go 會話端點
// 掛載點：/api/v1。WebAuth("session") 語意（middleware/webauth.go）：session cookie
// 缺失或 HS256 驗簽失敗 → 401 {"error":"SESSION_EXPIRED"}。

import { Hono } from 'hono';
import type { Context } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import bcrypt from 'bcryptjs';
import { signJwt, verifyJwt } from '../lib/jwt';
import { SESSION_TTL, REFRESH_TTL, sessionCookie, refreshCookie, csrfCookie, clearAuthCookies } from '../lib/cookies';
import { randomHex } from '../lib/csrf';
import { sanitizedUser, type UserRow } from '../lib/user';
import type { Env } from '../index';

type AppEnv = { Bindings: Env; Variables: { userId: number } };
type UserWithHash = UserRow & { password_hash: string };

// setAdminAuthCookies：admin_session(30d) + admin_refresh(90d) + admin_csrf(30d 非 httpOnly)
async function issueAdminCookies(c: Context<AppEnv>, user: { id: number; username: string; role: string }) {
  const secret = c.env.JWT_SECRET;
  if (!secret) return false;
  const domain = c.env.COOKIE_DOMAIN;
  const base = { user_id: user.id, username: user.username, role: user.role };
  c.header('Set-Cookie', sessionCookie('admin_session', await signJwt(base, secret, SESSION_TTL), domain), { append: true });
  c.header('Set-Cookie', refreshCookie('admin_refresh', await signJwt(base, secret, REFRESH_TTL), domain), { append: true });
  c.header('Set-Cookie', csrfCookie('admin_csrf', randomHex(32), domain), { append: true });
  return true;
}

// GetCSRFToken 同一 handler 掛 /auth/csrf 與 /admin/auth/csrf：缺才發，雙 cookie 都補
function csrfHandler(c: Context<AppEnv>) {
  const domain = c.env.COOKIE_DOMAIN;
  if (!getCookie(c, 'csrf')) {
    c.header('Set-Cookie', csrfCookie('csrf', randomHex(32), domain), { append: true });
  }
  if (!getCookie(c, 'admin_csrf')) {
    c.header('Set-Cookie', csrfCookie('admin_csrf', randomHex(32), domain), { append: true });
  }
  return c.json({ ok: true });
}

export function authRoutes() {
  const app = new Hono<AppEnv>();

  // middleware.WebAuth("session")
  const webAuth = createMiddleware<AppEnv>(async (c, next) => {
    const secret = c.env.JWT_SECRET;
    // 跨站前端（pages.dev）第三方 cookie 被瀏覽器丟棄 → Bearer 兜底（同 JWT/密鑰）
    const bearer = c.req.header('Authorization')?.replace(/^Bearer /i, '');
    const token = (secret ? getCookie(c, 'session') : undefined) || bearer;
    if (!secret || !token) {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    const claims = await verifyJwt(token, secret);
    if (!claims || typeof claims.user_id !== 'number') {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    c.set('userId', claims.user_id);
    await next();
  });

  const userCols =
    'id, username, role, status, balance, subscription_status, subscription_tier, ' +
    'traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start, ' +
    'client_token, created_at';

  app.get('/auth/csrf', (c) => csrfHandler(c));
  app.get('/admin/auth/csrf', (c) => csrfHandler(c));

  // ValidateSession
  app.get('/auth/validate', webAuth, async (c) => {
    const user = await c.env.DB.prepare(`SELECT ${userCols} FROM users WHERE id = ?`)
      .bind(c.get('userId'))
      .first<UserRow>();
    if (!user) {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    return c.json({ user: sanitizedUser(user) });
  });

  // Refresh：WebAuth(session) 先行（對齊 main.go 路由鏈），再以 refresh cookie 重簽 session
  app.post('/auth/refresh', webAuth, async (c) => {
    const secret = c.env.JWT_SECRET as string;
    const refreshToken = getCookie(c, 'refresh');
    if (!refreshToken) {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    const claims = await verifyJwt(refreshToken, secret);
    if (!claims || typeof claims.user_id !== 'number') {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    const user = await c.env.DB.prepare('SELECT id, username, role FROM users WHERE id = ?')
      .bind(claims.user_id)
      .first<{ id: number; username: string; role: string }>();
    if (!user) {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    c.header(
      'Set-Cookie',
      sessionCookie(
        'session',
        await signJwt({ user_id: user.id, username: user.username, role: user.role }, secret, SESSION_TTL),
        c.env.COOKIE_DOMAIN,
      ),
      { append: true },
    );
    return c.json({ ok: true });
  });

  // Logout：清全部 6 個 cookie（portal + admin）
  app.post('/auth/logout', webAuth, (c) => {
    for (const v of clearAuthCookies(c.env.COOKIE_DOMAIN)) {
      c.header('Set-Cookie', v, { append: true });
    }
    return c.json({ ok: true });
  });

  // AdminLogin
  app.post('/admin/auth/login', async (c) => {
    let body: unknown;
    try {
      body = await c.req.json();
    } catch {
      return c.json({ error: 'invalid request body' }, 400);
    }
    if (body === null || typeof body !== 'object') {
      return c.json({ error: 'invalid request body' }, 400);
    }
    const { username, password } = body as { username?: unknown; password?: unknown };
    if ((username !== undefined && typeof username !== 'string') || (password !== undefined && typeof password !== 'string')) {
      return c.json({ error: 'invalid request body' }, 400);
    }
    const u = (username ?? '') as string;
    const p = (password ?? '') as string;
    if (u === '' || p === '') {
      return c.json({ error: 'username and password are required' }, 400);
    }

    const user = await c.env.DB.prepare('SELECT * FROM users WHERE username = ?').bind(u).first<UserWithHash>();
    if (!user) {
      return c.json({ error: 'invalid username or password' }, 401);
    }
    const valid = await bcrypt.compare(p, user.password_hash);
    if (!valid) {
      return c.json({ error: 'invalid username or password' }, 401);
    }
    if (user.status !== 'active') {
      return c.json({ error: 'account is not active' }, 403);
    }
    if (user.role !== 'admin') {
      return c.json({ error: 'admin access required' }, 403);
    }

    const ok = await issueAdminCookies(c, user);
    if (!ok) return c.json({ error: 'failed to establish session' }, 500);

    // 跨站前端（pages.dev）cookie 存不住 → 附 Bearer token 供 localStorage 兜底
    const secret = c.env.JWT_SECRET;
    const token = secret
      ? await signJwt({ user_id: user.id, username: user.username, role: user.role }, secret, 24 * 3600)
      : undefined;
    return c.json({ user: sanitizedUser(user), role: user.role, ...(token ? { token } : {}) });
  });

  // AdminLogout：只清 admin 三件套
  app.post('/admin/auth/logout', (c) => {
    for (const v of clearAuthCookies(c.env.COOKIE_DOMAIN)) {
      if (v.startsWith('admin_')) c.header('Set-Cookie', v, { append: true });
    }
    return c.json({ ok: true });
  });

  return app;
}
