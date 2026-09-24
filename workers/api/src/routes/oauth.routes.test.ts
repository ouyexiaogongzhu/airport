// Email 登入 / 註冊 + Google OAuth 路由
import { describe, expect, it, vi, afterEach } from 'vitest';
import bcrypt from 'bcryptjs';
import { createApp, type Env } from '../index';
import { createTestD1 } from '../testing/d1';
import { findOrCreateGoogleUser } from './oauth';

const SECRET = 'test-secret';
const PASSWORD = 'password123';

function setup(extra: Partial<Env> = {}) {
  const { db, raw } = createTestD1();
  const hash = bcrypt.hashSync(PASSWORD, 4);
  raw
    .prepare(
      "INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status, email) " +
        "VALUES (2, 'alice', ?, 'rf_alice', '11111111-2222-4333-8444-555555555555', 'active', 'alice@example.com')",
    )
    .run(hash);
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
    TURNSTILE_DISABLED: '1',
    PORTAL_URL: 'https://xv.rfplay.uk',
    COOKIE_DOMAIN: 'rfplay.uk',
    GOOGLE_CLIENT_ID: 'test-client-id',
    GOOGLE_CLIENT_SECRET: 'test-client-secret',
    ...extra,
  } as unknown as Env;
  const app = createApp();
  const req = (path: string, init: RequestInit = {}) =>
    app.request(`https://api.rfplay.uk/api/v1${path}`, init, env);
  return { raw, req, db, env };
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('email login / register', () => {
  it('logs in with email as identifier', async () => {
    const { req } = setup();
    const res = await req('/public/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'alice@example.com', password: PASSWORD }),
    });
    expect(res.status).toBe(200);
    const body = (await res.json()) as { user: { username: string; email: string } };
    expect(body.user.username).toBe('alice');
    expect(body.user.email).toBe('alice@example.com');
  });

  it('logs in with username still works', async () => {
    const { req } = setup();
    const res = await req('/public/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'alice', password: PASSWORD }),
    });
    expect(res.status).toBe(200);
  });

  it('registers with email', async () => {
    const { req, raw } = setup();
    const res = await req('/public/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: 'bob',
        password: 'password123',
        email: 'Bob@Example.COM',
      }),
    });
    expect(res.status).toBe(201);
    const body = (await res.json()) as { user: { email: string } };
    expect(body.user.email).toBe('bob@example.com');
    const row = raw.prepare('SELECT email FROM users WHERE username = ?').get('bob') as { email: string };
    expect(row.email).toBe('bob@example.com');
  });

  it('rejects duplicate email on register', async () => {
    const { req } = setup();
    const res = await req('/public/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: 'other',
        password: 'password123',
        email: 'alice@example.com',
      }),
    });
    expect(res.status).toBe(409);
    expect(await res.json()).toEqual({ error: 'email already exists' });
  });
});

describe('Google OAuth', () => {
  it('start redirects to Google with state cookie', async () => {
    const { req } = setup();
    const res = await req('/public/oauth/google/start', { redirect: 'manual' });
    expect(res.status).toBe(302);
    const loc = res.headers.get('Location')!;
    expect(loc).toContain('accounts.google.com');
    expect(loc).toContain('client_id=test-client-id');
    expect(loc).toContain(encodeURIComponent('https://api.rfplay.uk/api/v1/public/oauth/google/callback'));
    const setCookie = res.headers.getSetCookie().join(';');
    expect(setCookie).toContain('oauth_google_state=');
  });

  it('start without secrets redirects to portal error', async () => {
    const { req } = setup({ GOOGLE_CLIENT_ID: '', GOOGLE_CLIENT_SECRET: '' });
    const res = await req('/public/oauth/google/start', { redirect: 'manual' });
    expect(res.status).toBe(302);
    expect(res.headers.get('Location')).toBe('https://xv.rfplay.uk/login?oauth_error=google_not_configured');
  });

  it('callback creates user and redirects to dashboard', async () => {
    const { req, raw } = setup();
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string | URL) => {
        const u = String(url);
        if (u.includes('oauth2.googleapis.com/token')) {
          return new Response(JSON.stringify({ access_token: 'ya29.test' }), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          });
        }
        if (u.includes('openidconnect.googleapis.com/userinfo')) {
          return new Response(
            JSON.stringify({
              sub: 'google-sub-1',
              email: 'newuser@gmail.com',
              email_verified: true,
              name: 'New User',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          );
        }
        throw new Error(`unexpected fetch ${u}`);
      }),
    );

    const res = await req('/public/oauth/google/callback?code=abc&state=xyz', {
      headers: { Cookie: 'oauth_google_state=xyz' },
      redirect: 'manual',
    });
    expect(res.status).toBe(302);
    expect(res.headers.get('Location')).toBe('https://xv.rfplay.uk/dashboard');
    const cookies = res.headers.getSetCookie().join('\n');
    expect(cookies).toContain('session=');
    expect(cookies).toContain('Domain=rfplay.uk');

    const row = raw.prepare('SELECT username, email, google_sub FROM users WHERE google_sub = ?').get('google-sub-1') as {
      username: string;
      email: string;
      google_sub: string;
    };
    expect(row.email).toBe('newuser@gmail.com');
    expect(row.google_sub).toBe('google-sub-1');
    expect(row.username.length).toBeGreaterThan(0);
  });

  it('callback 不自動綁定既有 email 帳號（防帳號預占），要求密碼登入', async () => {
    const { db } = setup();
    const r = await findOrCreateGoogleUser(db, {
      sub: 'google-sub-alice',
      email: 'alice@example.com',
      name: 'Alice',
    });
    expect('error' in r).toBe(true);
    if ('error' in r) expect(r.error).toBe('email registered');
    const row = (await db.prepare('SELECT google_sub FROM users WHERE id = 2').first()) as {
      google_sub: string | null;
    };
    expect(row.google_sub).toBeNull();
  });

  it('rejects invalid state', async () => {
    const { req } = setup();
    const res = await req('/public/oauth/google/callback?code=abc&state=bad', {
      headers: { Cookie: 'oauth_google_state=good' },
      redirect: 'manual',
    });
    expect(res.status).toBe(302);
    expect(res.headers.get('Location')).toContain('oauth_error=invalid_state');
  });
});
