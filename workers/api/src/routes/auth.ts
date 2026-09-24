// /auth/* + /admin/auth/* — 逐字移植 manager/internal/handler/auth.go 會話端點
// 掛載點：/api/v1。WebAuth("session") 語意（middleware/webauth.go）：session cookie
// 缺失、驗簽失敗、token_version 不符或帳號非 active → 401 {"error":"SESSION_EXPIRED"}。

import { Hono } from 'hono';
import type { Context } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import bcrypt from 'bcryptjs';
import { verifyJwt } from '../lib/jwt';
import { verifyTurnstile } from '../lib/turnstile';
import {
  PORTAL_SESSION_TTL,
  ADMIN_SESSION_TTL,
  sessionCookie,
  refreshCookie,
  csrfCookie,
  clearAuthCookies,
  clearHostOnlyAuthCookies,
} from '../lib/cookies';
import { randomHex } from '../lib/csrf';
import {
  authenticate,
  authenticateAccessCandidates,
  bumpTokenVersion,
  signTokens,
  signAccess,
  BEARER_TTL,
  PORTAL_BEARER_TTL,
  type SessionUser,
} from '../lib/session';
import { sanitizedUser, USER_PROFILE_COLS, type UserRow } from '../lib/user';
import type { Env } from '../index';

type AppEnv = { Bindings: Env; Variables: { userId: number; role: string } };
type UserWithHash = UserRow & { password_hash: string; token_version: number };

// setAdminAuthCookies：admin_session(30d) + admin_refresh(90d) + admin_csrf(30d 非 httpOnly)
async function issueAdminCookies(c: Context<AppEnv>, user: SessionUser | UserWithHash) {
  const secret = c.env.JWT_SECRET;
  if (!secret) return null;
  const domain = c.env.COOKIE_DOMAIN;
  const t = await signTokens(user, secret, 'admin');
  // Domain cookie 不會覆蓋舊 host-only；登入前先清，避免雙份同名 session
  if (domain) {
    for (const v of clearHostOnlyAuthCookies(['admin_session', 'admin_refresh', 'admin_csrf'])) {
      c.header('Set-Cookie', v, { append: true });
    }
  }
  c.header('Set-Cookie', sessionCookie('admin_session', t.session, domain), { append: true });
  c.header('Set-Cookie', refreshCookie('admin_refresh', t.refresh, domain), { append: true });
  c.header('Set-Cookie', csrfCookie('admin_csrf', randomHex(32), domain), { append: true });
  return t;
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

function bearerOf(c: Context<AppEnv>): string | undefined {
  return c.req.header('Authorization')?.replace(/^Bearer /i, '') || undefined;
}

// refresh token 來源：cookie（同站）或 JSON body.refresh_token（跨站 localStorage）；
// 不看 access token（Authorization 頭可能帶著已過期的 Bearer）
async function refreshCandidates(c: Context<AppEnv>, cookieName: string): Promise<string[]> {
  const body = await c.req.json<{ refresh_token?: unknown }>().catch(() => null);
  const fromBody = typeof body?.refresh_token === 'string' ? body.refresh_token : '';
  return [getCookie(c, cookieName) ?? '', fromBody].filter((t) => t !== '');
}

async function refreshUser(c: Context<AppEnv>, cookieName: string): Promise<SessionUser | null> {
  const secret = c.env.JWT_SECRET;
  if (!secret) return null;
  for (const t of await refreshCandidates(c, cookieName)) {
    const r = await authenticate(c.env.DB, t, secret, 'refresh');
    if ('user' in r) return r.user;
  }
  return null;
}

// 退出：任一當前版本的 token（access / refresh）即可吊銷該用戶全部會話；
// 已吊銷的舊 token 不能再觸發加一（防止被盜舊 token 反覆踢人）
async function revokeFromRequest(c: Context<AppEnv>, names: string[]) {
  const secret = c.env.JWT_SECRET;
  if (!secret) return;
  const body = await c.req.json<{ refresh_token?: unknown }>().catch(() => null);
  const tokens = [...names.map((n) => getCookie(c, n)), bearerOf(c), body?.refresh_token];
  for (const t of tokens) {
    if (typeof t !== 'string' || t === '') continue;
    const claims = await verifyJwt(t, secret);
    if (!claims || typeof claims.user_id !== 'number') continue;
    const r = await bumpTokenVersion(c.env.DB, claims.user_id, claims.tv ?? 0).run();
    if ((r.meta.changes ?? 0) > 0) return;
  }
}

export function authRoutes() {
  const app = new Hono<AppEnv>();

  // middleware.WebAuth(cookieName)：cookie 優先，失效時再試 Bearer（同 JWT/密鑰）
  const sessionAuth = (cookieName: string) =>
    createMiddleware<AppEnv>(async (c, next) => {
      const secret = c.env.JWT_SECRET;
      const r = await authenticateAccessCandidates(c.env.DB, secret, [
        secret ? getCookie(c, cookieName) : undefined,
        bearerOf(c),
      ]);
      if (!('user' in r)) return c.json({ error: 'SESSION_EXPIRED' }, 401);
      c.set('userId', r.user.id);
      c.set('role', r.user.role);
      await next();
    });
  const webAuth = sessionAuth('session');
  const adminSessionAuth = sessionAuth('admin_session');

  app.get('/auth/csrf', (c) => csrfHandler(c));
  app.get('/admin/auth/csrf', (c) => csrfHandler(c));

  // ValidateSession
  app.get('/auth/validate', webAuth, async (c) => {
    const user = await c.env.DB.prepare(`SELECT ${USER_PROFILE_COLS} FROM users WHERE id = ?`)
      .bind(c.get('userId'))
      .first<UserRow>();
    if (!user) {
      return c.json({ error: 'SESSION_EXPIRED' }, 401);
    }
    return c.json({ user: sanitizedUser(user) });
  });

  // 後台專用校驗：只認 admin_session（+ Bearer 兜底），不會被 portal 的 session cookie 冒充
  app.get('/admin/auth/validate', adminSessionAuth, async (c) => {
    if (c.get('role') !== 'admin') return c.json({ error: 'admin access required' }, 403);
    const user = await c.env.DB.prepare(`SELECT ${USER_PROFILE_COLS} FROM users WHERE id = ?`)
      .bind(c.get('userId'))
      .first<UserRow>();
    if (!user) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    return c.json({ user: sanitizedUser(user), role: user.role });
  });

  // Refresh：只校驗 refresh token（access 過期也能續），重簽 session cookie 並回傳新 Bearer
  app.post('/auth/refresh', async (c) => {
    const user = await refreshUser(c, 'refresh');
    if (!user) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    const secret = c.env.JWT_SECRET as string;
    c.header(
      'Set-Cookie',
      sessionCookie('session', await signAccess(user, secret, PORTAL_SESSION_TTL), c.env.COOKIE_DOMAIN),
      { append: true },
    );
    return c.json({ ok: true, token: await signAccess(user, secret, PORTAL_BEARER_TTL) });
  });

  app.post('/admin/auth/refresh', async (c) => {
    const user = await refreshUser(c, 'admin_refresh');
    if (!user) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    if (user.role !== 'admin') return c.json({ error: 'admin access required' }, 403);
    const secret = c.env.JWT_SECRET as string;
    c.header(
      'Set-Cookie',
      sessionCookie('admin_session', await signAccess(user, secret, ADMIN_SESSION_TTL), c.env.COOKIE_DOMAIN),
      { append: true },
    );
    return c.json({ ok: true, token: await signAccess(user, secret, BEARER_TTL) });
  });

  // Logout：吊銷該用戶全部會話（token_version+1），清全部 6 個 cookie（portal + admin）；
  // 會話已失效也照樣清 cookie
  app.post('/auth/logout', async (c) => {
    await revokeFromRequest(c, ['session', 'refresh']);
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

    const ts = await verifyTurnstile(
      (body as Record<string, unknown>)['cf-turnstile-response'],
      c.req.header('CF-Connecting-IP'),
      c.env,
    );
    if (!ts.ok) return c.json({ error: ts.error }, ts.status);

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

    const t = await issueAdminCookies(c, user);
    if (!t) return c.json({ error: 'failed to establish session' }, 500);

    // 跨站前端（pages.dev）cookie 存不住 → 附 Bearer token + refresh token 供 localStorage 兜底
    return c.json({ user: sanitizedUser(user), role: user.role, token: t.bearer, refresh_token: t.refresh });
  });

  // AdminLogout：吊銷會話，只清 admin 三件套
  app.post('/admin/auth/logout', async (c) => {
    await revokeFromRequest(c, ['admin_session', 'admin_refresh']);
    for (const v of clearAuthCookies(c.env.COOKIE_DOMAIN)) {
      if (v.startsWith('admin_')) c.header('Set-Cookie', v, { append: true });
    }
    return c.json({ ok: true });
  });

  return app;
}
