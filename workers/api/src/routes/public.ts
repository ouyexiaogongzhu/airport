// /public/register|login — 逐字移植 manager/internal/handler/auth.go Register/Login
// 掛載點：/api/v1（對齊 cmd/server/main.go v1 group）。
// 進程內限流（sync.Map）已刪除 → CF WAF + Turnstile 接管。
// 圖形驗證碼 /captcha 已刪除 → Turnstile siteverify。
// 登入 identifier：username 或 email（欄位名仍為 username，相容舊客戶端）。

import { Hono } from 'hono';
import bcrypt from 'bcryptjs';
import { verifyTurnstile } from '../lib/turnstile';
import { ensureUserCredentials, issuePortalSession } from '../lib/portalSession';
import { randomHex } from '../lib/csrf';
import { isValidEmail, sanitizedUser, DUMMY_BCRYPT_HASH, type UserRow } from '../lib/user';
import type { Env } from '../index';

// Go bcrypt.DefaultCost == bcryptjs 預設 rounds == 10，顯式寫出以免漂移
const BCRYPT_COST = 10;

type UserWithHash = UserRow & { password_hash: string; token_version: number };

// Fiber BodyParser 語意：JSON 解析失敗或欄位型別不符 → 400 "invalid request body"；
// 欄位缺失 → 零值 "" → 後續 "username and password are required"。
function parseCredentials(
  body: unknown,
): { ok: true; username: string; password: string; email: string } | { ok: false } {
  if (body === null || typeof body !== 'object') return { ok: false };
  const { username, password, email } = body as {
    username?: unknown;
    password?: unknown;
    email?: unknown;
  };
  if (
    (username !== undefined && typeof username !== 'string') ||
    (password !== undefined && typeof password !== 'string') ||
    (email !== undefined && typeof email !== 'string')
  ) {
    return { ok: false };
  }
  return {
    ok: true,
    username: username ?? '',
    password: password ?? '',
    email: (email ?? '').trim(),
  };
}

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

/** 以 username 或 email 查找用戶（email 大小寫不敏感） */
async function findUserByIdentifier(db: D1Database, identifier: string): Promise<UserWithHash | null> {
  const id = identifier.trim();
  if (!id) return null;
  const byUsername = await db.prepare('SELECT * FROM users WHERE username = ?').bind(id).first<UserWithHash>();
  if (byUsername) return byUsername;
  if (!id.includes('@')) return null;
  const email = normalizeEmail(id);
  if (!isValidEmail(email)) return null;
  return db.prepare('SELECT * FROM users WHERE email = ? COLLATE NOCASE').bind(email).first<UserWithHash>();
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
    const emailRaw = parsed.email;
    if (username === '' || password === '') {
      return c.json({ error: 'username and password are required' }, 400);
    }
    if (password.length < 8) {
      return c.json({ error: 'password must be at least 8 characters' }, 400);
    }

    let email: string | null = null;
    if (emailRaw !== '') {
      email = normalizeEmail(emailRaw);
      if (!isValidEmail(email)) {
        return c.json({ error: 'invalid email format' }, 400);
      }
      const emailTaken = await c.env.DB.prepare('SELECT id FROM users WHERE email = ? COLLATE NOCASE')
        .bind(email)
        .first();
      if (emailTaken) {
        return c.json({ error: 'email already exists' }, 409);
      }
    }

    const existing = await c.env.DB.prepare('SELECT id FROM users WHERE username = ?').bind(username).first();
    if (existing) {
      return c.json({ error: 'username already exists' }, 409);
    }

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
            client_token, vless_uuid, ss_password, trojan_password, email, created_at, updated_at)
         VALUES (?, ?, 'user', 'active', 0, 'pending', NULL, 0, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?)`,
      )
        .bind(
          username,
          hash,
          clientToken,
          creds.vless_uuid,
          creds.ss_password,
          creds.trojan_password,
          email,
          now,
          now,
        )
        .run();
      userId = Number(result.meta.last_row_id);
      if (!userId) return c.json({ error: 'failed to create user' }, 500);
    } catch {
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
      email,
      phone: null,
      display_name: null,
      billing_address: null,
    };

    const tokens = await issuePortalSession(c, user);
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

    const user = await findUserByIdentifier(c.env.DB, username);
    if (!user) {
      await bcrypt.compare(password, DUMMY_BCRYPT_HASH); // 抹平時序差異，防用戶名枚舉
      return c.json({ error: 'invalid username or password' }, 401);
    }

    const valid = await bcrypt.compare(password, user.password_hash);
    if (!valid) {
      return c.json({ error: 'invalid username or password' }, 401);
    }

    if (user.status !== 'active') {
      return c.json({ error: 'account is not active' }, 403);
    }

    const tokens = await issuePortalSession(c, user);
    if (!tokens) return c.json({ error: 'failed to establish session' }, 500);
    return c.json({ ...tokens, user: sanitizedUser(user) });
  });

  return app;
}
