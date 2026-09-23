// /public/register|login — 逐字移植 manager/internal/handler/auth.go Register/Login
// 掛載點：/api/v1（對齊 cmd/server/main.go v1 group）。
// 進程內限流（sync.Map）已刪除 → CF WAF + Turnstile 接管。
// 圖形驗證碼 /captcha 已刪除 → Turnstile siteverify。

import { Hono } from 'hono';
import bcrypt from 'bcryptjs';
import { verifyTurnstile } from '../lib/turnstile';
import { sessionCookie, refreshCookie, csrfCookie, clearHostOnlyAuthCookies } from '../lib/cookies';
import { randomHex } from '../lib/csrf';
import { signTokens } from '../lib/session';
import { sanitizedUser, type UserRow } from '../lib/user';
import type { Env } from '../index';

// Go bcrypt.DefaultCost == bcryptjs 預設 rounds == 10，顯式寫出以免漂移
const BCRYPT_COST = 10;

type UserWithHash = UserRow & { password_hash: string; token_version: number };
type Credentials = { id: number; username: string; role: string; token_version?: number };

// setWebAuthCookies：session(2h)+refresh(90d)+csrf(2h 非 httpOnly)；
// 回傳 Bearer(2h) 與 refresh token 供跨站前端（pages.dev）localStorage 兜底與續期
async function issueSession(
  c: { env: Env; header: (name: 'Set-Cookie', value: string, opts?: { append?: boolean }) => void },
  user: Credentials,
): Promise<{ token: string; refresh_token: string } | null> {
  const secret = c.env.JWT_SECRET;
  if (!secret) return null;
  const domain = c.env.COOKIE_DOMAIN;
  const t = await signTokens(user, secret, 'portal');
  // Domain cookie 不會覆蓋舊 host-only；登入前先清，避免雙份同名 session
  if (domain) {
    for (const v of clearHostOnlyAuthCookies(['session', 'refresh', 'csrf'])) {
      c.header('Set-Cookie', v, { append: true });
    }
  }
  c.header('Set-Cookie', sessionCookie('session', t.session, domain), { append: true });
  c.header('Set-Cookie', refreshCookie('refresh', t.refresh, domain), { append: true });
  c.header('Set-Cookie', csrfCookie('csrf', randomHex(32), domain), { append: true });
  return { token: t.bearer, refresh_token: t.refresh };
}

// Fiber BodyParser 語意：JSON 解析失敗或欄位型別不符 → 400 "invalid request body"；
// 欄位缺失 → 零值 "" → 後續 "username and password are required"。
function parseCredentials(body: unknown): { ok: true; username: string; password: string } | { ok: false } {
  if (body === null || typeof body !== 'object') return { ok: false };
  const { username, password } = body as { username?: unknown; password?: unknown };
  if ((username !== undefined && typeof username !== 'string') || (password !== undefined && typeof password !== 'string')) {
    return { ok: false };
  }
  return { ok: true, username: username ?? '', password: password ?? '' };
}

// ensureUserCredentials（credentials.go）：vless_uuid / ss_password(16B) / trojan_password(24B)
function ensureUserCredentials() {
  return {
    vless_uuid: crypto.randomUUID(),
    ss_password: randomHex(16),
    trojan_password: randomHex(24),
  };
}

export function publicRoutes() {
  const app = new Hono<{ Bindings: Env }>();

  app.post('/public/register', async (c) => {
    let body: unknown;
    try {
      body = await c.req.json();
    } catch {
      return c.json({ error: 'invalid request body' }, 400);
    }
    const parsed = parseCredentials(body);
    if (!parsed.ok) {
      return c.json({ error: 'invalid request body' }, 400);
    }

    const ts = await verifyTurnstile(
      (body as Record<string, unknown>)['cf-turnstile-response'],
      c.req.header('CF-Connecting-IP'),
      c.env,
    );
    if (!ts.ok) return c.json({ error: ts.error }, ts.status);

    const { username, password } = parsed;
    if (username === '' || password === '') {
      return c.json({ error: 'username and password are required' }, 400);
    }
    if (password.length < 8) {
      return c.json({ error: 'password must be at least 8 characters' }, 400);
    }

    // Check existing user
    const existing = await c.env.DB.prepare('SELECT id FROM users WHERE username = ?').bind(username).first();
    if (existing) {
      return c.json({ error: 'username already exists' }, 409);
    }

    // Hash password
    let hash: string;
    try {
      hash = await bcrypt.hash(password, BCRYPT_COST);
    } catch {
      return c.json({ error: 'failed to hash password' }, 500);
    }

    const creds = ensureUserCredentials();
    const clientToken = 'rf_' + randomHex(32);
    const now = new Date().toISOString();

    let userId: number;
    try {
      const result = await c.env.DB.prepare(
        `INSERT INTO users
           (username, password_hash, role, status, balance, subscription_status, subscription_tier,
            traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start,
            client_token, vless_uuid, ss_password, trojan_password, created_at, updated_at)
         VALUES (?, ?, 'user', 'active', 0, 'pending', NULL, 0, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?)`,
      )
        .bind(username, hash, clientToken, creds.vless_uuid, creds.ss_password, creds.trojan_password, now, now)
        .run();
      userId = Number(result.meta.last_row_id);
      if (!userId) return c.json({ error: 'failed to create user' }, 500);
    } catch {
      // GORM Create 失敗（含 UNIQUE race）同樣落 500 "failed to create user"
      return c.json({ error: 'failed to create user' }, 500);
    }

    const user: UserRow = {
      id: userId,
      username,
      role: 'user',
      status: 'active',
      balance: 0,
      subscription_status: 'pending',
      subscription_tier: null,
      traffic_limit_bytes: 0,
      traffic_used_bytes: 0,
      expire_time: 0,
      rate_limit_bps: 0,
      traffic_period_start: 0,
      client_token: clientToken,
      created_at: now,
      email: null,
      phone: null,
      display_name: null,
      billing_address: null,
    };

    // Go 同樣永遠返回 token；跨站前端靠它 Bearer 兜底
    const tokens = await issueSession(c, user);
    if (!tokens) return c.json({ error: 'failed to establish session' }, 500);
    return c.json({ ...tokens, user: sanitizedUser(user) }, 201);
  });

  app.post('/public/login', async (c) => {
    let body: unknown;
    try {
      body = await c.req.json();
    } catch {
      return c.json({ error: 'invalid request body' }, 400);
    }
    const parsed = parseCredentials(body);
    if (!parsed.ok) {
      return c.json({ error: 'invalid request body' }, 400);
    }

    const ts = await verifyTurnstile(
      (body as Record<string, unknown>)['cf-turnstile-response'],
      c.req.header('CF-Connecting-IP'),
      c.env,
    );
    if (!ts.ok) return c.json({ error: ts.error }, ts.status);

    const { username, password } = parsed;
    if (username === '' || password === '') {
      return c.json({ error: 'username and password are required' }, 400);
    }

    const user = await c.env.DB.prepare('SELECT * FROM users WHERE username = ?').bind(username).first<UserWithHash>();
    if (!user) {
      return c.json({ error: 'invalid username or password' }, 401);
    }

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) {
      return c.json({ error: 'invalid username or password' }, 401);
    }

    if (user.status !== 'active') {
      return c.json({ error: 'account is not active' }, 403);
    }

    // Go AuthResponse 永遠含 token；跨站前端（pages.dev）靠它做 Bearer 兜底
    const tokens = await issueSession(c, user);
    if (!tokens) return c.json({ error: 'failed to establish session' }, 500);
    return c.json({ ...tokens, user: sanitizedUser(user) });
  });

  return app;
}
