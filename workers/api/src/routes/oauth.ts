// Google OAuth 2.0 / OIDC authorization code flow（portal 登入，無 Turnstile）
// GET  /public/oauth/google/start    → 導向 Google
// GET  /public/oauth/google/callback → 換 token、find-or-create、發 session cookie、導回 portal

import { Hono } from 'hono';
import { getCookie } from 'hono/cookie';
import bcrypt from 'bcryptjs';
import { randomHex } from '../lib/csrf';
import { ensureUserCredentials, issuePortalSession } from '../lib/portalSession';
import { isValidEmail, USER_PROFILE_COLS, type UserRow } from '../lib/user';
import type { Env } from '../index';

const BCRYPT_COST = 10;
const OAUTH_STATE_COOKIE = 'oauth_google_state';
const OAUTH_STATE_TTL = 600;
const GOOGLE_AUTH = 'https://accounts.google.com/o/oauth2/v2/auth';
const GOOGLE_TOKEN = 'https://oauth2.googleapis.com/token';
const GOOGLE_USERINFO = 'https://openidconnect.googleapis.com/userinfo';

type GoogleUserInfo = {
  sub?: string;
  email?: string;
  email_verified?: boolean | string;
  name?: string;
};

type UserWithOAuth = UserRow & { password_hash: string; token_version: number; google_sub?: string | null };

function googleConfigured(env: Env): boolean {
  return !!(env.GOOGLE_CLIENT_ID && env.GOOGLE_CLIENT_SECRET);
}

function redirectUri(c: { env: Env; req: { url: string } }): string {
  if (envRedirect(c.env)) return envRedirect(c.env)!;
  const origin = new URL(c.req.url).origin;
  return `${origin}/api/v1/public/oauth/google/callback`;
}

function envRedirect(env: Env): string | undefined {
  const v = env.GOOGLE_REDIRECT_URI?.trim();
  return v || undefined;
}

function portalBase(env: Env): string {
  return (env.PORTAL_URL || 'https://xv.rfplay.uk').replace(/\/$/, '');
}

function oauthErrorUrl(env: Env, code: string): string {
  return `${portalBase(env)}/login?oauth_error=${encodeURIComponent(code)}`;
}

// 必須用 c.redirect（走 c.newResponse，合併先前 c.header 的 Set-Cookie）；
// 裸 Response.redirect 會丟掉 clearOAuthStateCookie，state cookie 殘留到 TTL。
function oauthErrorRedirect(c: { env: Env; redirect: (url: string, status: 302) => Response }, code: string): Response {
  return c.redirect(oauthErrorUrl(c.env, code), 302);
}

function clearOAuthStateCookie(): string {
  return `${OAUTH_STATE_COOKIE}=; Path=/; Max-Age=0; Secure; HttpOnly; SameSite=Lax`;
}

function setOAuthStateCookie(state: string): string {
  return `${OAUTH_STATE_COOKIE}=${state}; Path=/; Max-Age=${OAUTH_STATE_TTL}; Secure; HttpOnly; SameSite=Lax`;
}

function normalizeEmail(email: string): string {
  return email.trim().toLowerCase();
}

/** 從 email local-part 生成可用 username，衝突時加後綴 */
async function allocateUsername(db: D1Database, email: string, displayName?: string): Promise<string> {
  const local = email.split('@')[0] || 'user';
  const fromName = (displayName || local)
    .toLowerCase()
    .replace(/[^a-z0-9_]+/g, '_')
    .replace(/^_+|_+$/g, '')
    .slice(0, 24);
  let base = fromName || 'user';
  if (base.length < 3) base = `user_${base}`;

  for (let i = 0; i < 8; i++) {
    const candidate = i === 0 ? base : `${base}_${randomHex(3)}`;
    const taken = await db.prepare('SELECT id FROM users WHERE username = ?').bind(candidate).first();
    if (!taken) return candidate;
  }
  return `g_${randomHex(8)}`;
}

async function findOrCreateGoogleUser(
  db: D1Database,
  info: { sub: string; email: string; name?: string },
): Promise<{ user: UserWithOAuth } | { error: string; status: 403 | 409 | 500 }> {
  const bySub = await db
    .prepare(`SELECT ${USER_PROFILE_COLS}, password_hash, token_version, google_sub FROM users WHERE google_sub = ?`)
    .bind(info.sub)
    .first<UserWithOAuth>();
  if (bySub) {
    if (bySub.status !== 'active') return { error: 'account is not active', status: 403 };
    return { user: bySub };
  }

  const byEmail = await db
    .prepare(`SELECT ${USER_PROFILE_COLS}, password_hash, token_version, google_sub FROM users WHERE email = ? COLLATE NOCASE`)
    .bind(info.email)
    .first<UserWithOAuth>();

  if (byEmail) {
    if (byEmail.status !== 'active') return { error: 'account is not active', status: 403 };
    if (byEmail.google_sub === info.sub) return { user: byEmail };
    // 本地帳號的 email 從未驗證（註冊/改信箱僅查重）：不得自動綁定 google_sub，
    // 否則攻擊者可註冊填受害者 Gmail → 受害者 Google 登入被導進攻擊者帳號（帳號預占接管）。
    // 已綁其他 Google 帳號或從未綁定：一律要求密碼登入；綁定日後做帳號內手動流程。
    return {
      error: byEmail.google_sub ? 'email already linked to another Google account' : 'email registered',
      status: 409,
    };
  }

  // 新建 OAuth 用戶：不可登入密碼的隨機 hash
  let hash: string;
  try {
    hash = await bcrypt.hash(randomHex(32), BCRYPT_COST);
  } catch {
    return { error: 'failed to create user', status: 500 };
  }

  const username = await allocateUsername(db, info.email, info.name);
  const creds = ensureUserCredentials();
  const clientToken = 'rf_' + randomHex(32);
  const now = new Date().toISOString();
  const displayName = info.name?.trim() || null;

  try {
    const result = await db
      .prepare(
        `INSERT INTO users
           (username, password_hash, role, status, balance, subscription_status, subscription_tier,
            traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, traffic_period_start,
            client_token, vless_uuid, ss_password, trojan_password, email, display_name, google_sub,
            created_at, updated_at)
         VALUES (?, ?, 'user', 'active', 0, 'pending', NULL, 0, 0, 0, 0, 0, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
      )
      .bind(
        username,
        hash,
        clientToken,
        creds.vless_uuid,
        creds.ss_password,
        creds.trojan_password,
        info.email,
        displayName,
        info.sub,
        now,
        now,
      )
      .run();
    const userId = Number(result.meta.last_row_id);
    if (!userId) return { error: 'failed to create user', status: 500 };

    const user: UserWithOAuth = {
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
      email: info.email,
      phone: null,
      display_name: displayName,
      billing_address: null,
      password_hash: hash,
      token_version: 0,
      google_sub: info.sub,
    };
    return { user };
  } catch {
    return { error: 'failed to create user', status: 500 };
  }
}

export function oauthRoutes() {
  const app = new Hono<{ Bindings: Env }>();

  app.get('/public/oauth/google/start', (c) => {
    if (!googleConfigured(c.env)) {
      return oauthErrorRedirect(c, 'google_not_configured');
    }
    const state = randomHex(16);
    const params = new URLSearchParams({
      client_id: c.env.GOOGLE_CLIENT_ID!,
      redirect_uri: redirectUri(c),
      response_type: 'code',
      scope: 'openid email profile',
      state,
      access_type: 'online',
      prompt: 'select_account',
    });
    c.header('Set-Cookie', setOAuthStateCookie(state), { append: true });
    return c.redirect(`${GOOGLE_AUTH}?${params.toString()}`, 302);
  });

  app.get('/public/oauth/google/callback', async (c) => {
    if (!googleConfigured(c.env)) {
      return oauthErrorRedirect(c, 'google_not_configured');
    }

    const err = c.req.query('error');
    if (err) {
      c.header('Set-Cookie', clearOAuthStateCookie(), { append: true });
      return oauthErrorRedirect(c, err === 'access_denied' ? 'access_denied' : 'google_denied');
    }

    const code = c.req.query('code');
    const state = c.req.query('state');
    const cookieState = getCookie(c, OAUTH_STATE_COOKIE);
    c.header('Set-Cookie', clearOAuthStateCookie(), { append: true });

    if (!code || !state || !cookieState || state !== cookieState) {
      return oauthErrorRedirect(c, 'invalid_state');
    }

    const redir = redirectUri(c);
    let tokenRes: Response;
    try {
      tokenRes = await fetch(GOOGLE_TOKEN, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({
          code,
          client_id: c.env.GOOGLE_CLIENT_ID!,
          client_secret: c.env.GOOGLE_CLIENT_SECRET!,
          redirect_uri: redir,
          grant_type: 'authorization_code',
        }),
      });
    } catch {
      return oauthErrorRedirect(c, 'token_exchange_failed');
    }

    if (!tokenRes.ok) {
      return oauthErrorRedirect(c, 'token_exchange_failed');
    }

    const tokenJson = (await tokenRes.json()) as { access_token?: string };
    if (!tokenJson.access_token) {
      return oauthErrorRedirect(c, 'token_exchange_failed');
    }

    let infoRes: Response;
    try {
      infoRes = await fetch(GOOGLE_USERINFO, {
        headers: { Authorization: `Bearer ${tokenJson.access_token}` },
      });
    } catch {
      return oauthErrorRedirect(c, 'userinfo_failed');
    }
    if (!infoRes.ok) {
      return oauthErrorRedirect(c, 'userinfo_failed');
    }

    const info = (await infoRes.json()) as GoogleUserInfo;
    const sub = typeof info.sub === 'string' ? info.sub : '';
    const emailRaw = typeof info.email === 'string' ? info.email : '';
    const verified = info.email_verified === true || info.email_verified === 'true';
    if (!sub || !emailRaw || !verified) {
      return oauthErrorRedirect(c, 'email_unverified');
    }
    const email = normalizeEmail(emailRaw);
    if (!isValidEmail(email)) {
      return oauthErrorRedirect(c, 'invalid_email');
    }

    const result = await findOrCreateGoogleUser(c.env.DB, {
      sub,
      email,
      name: typeof info.name === 'string' ? info.name : undefined,
    });
    if ('error' in result) {
      const code =
        result.error === 'account is not active'
          ? 'account_disabled'
          : result.error === 'email registered'
            ? 'email_registered'
            : result.error === 'email already linked to another Google account'
              ? 'email_linked'
              : 'create_failed';
      return oauthErrorRedirect(c, code);
    }

    const tokens = await issuePortalSession(c, result.user);
    if (!tokens) {
      return oauthErrorRedirect(c, 'session_failed');
    }

    // Domain=rfplay.uk session cookie 對 xv.rfplay.uk 可見；直接進 dashboard
    return c.redirect(`${portalBase(c.env)}/dashboard`, 302);
  });

  return app;
}

/** 測試用：導出 find-or-create 邏輯 */
export { findOrCreateGoogleUser, allocateUsername };
