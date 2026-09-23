// GET/PUT /user/profile — 聯絡欄位更新與 email 校驗
import { describe, expect, it } from 'vitest';
import bcrypt from 'bcryptjs';
import { createApp, type Env } from '../index';
import { signTokens } from '../lib/session';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';
const PASSWORD = 'password123';

function setup() {
  const { db, raw } = createTestD1();
  const hash = bcrypt.hashSync(PASSWORD, 4);
  raw
    .prepare(
      "INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status) " +
        "VALUES (2, 'alice', ?, 'rf_alice', '11111111-2222-4333-8444-555555555555', 'active')",
    )
    .run(hash);
  raw
    .prepare(
      "INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status, email) " +
        "VALUES (3, 'bob', ?, 'rf_bob', '22222222-3333-4444-8555-666666666666', 'active', 'taken@example.com')",
    )
    .run(hash);
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
    TURNSTILE_DISABLED: '1',
  } as unknown as Env;
  const app = createApp();
  const req = (path: string, init: RequestInit = {}) => app.request(`/api/v1${path}`, init, env);
  const auth = async (id: number) => {
    const u = raw.prepare('SELECT id, username, role, token_version FROM users WHERE id = ?').get(id) as {
      id: number;
      username: string;
      role: string;
      token_version: number;
    };
    const t = await signTokens(u, SECRET, 'portal');
    return {
      Authorization: `Bearer ${t.bearer}`,
      'Content-Type': 'application/json',
    };
  };
  return { raw, req, auth };
}

describe('GET /user/profile', () => {
  it('returns sanitized profile with null profile fields by default', async () => {
    const { req, auth } = setup();
    const res = await req('/user/profile', { headers: await auth(2) });
    expect(res.status).toBe(200);
    const body = (await res.json()) as Record<string, unknown>;
    expect(body.username).toBe('alice');
    expect(body.email).toBeNull();
    expect(body.phone).toBeNull();
    expect(body.display_name).toBeNull();
    expect(body.billing_address).toBeNull();
    expect(body).not.toHaveProperty('password_hash');
  });
});

describe('PUT /user/profile', () => {
  it('updates email, phone, display_name, billing_address', async () => {
    const { req, auth, raw } = setup();
    const res = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({
        email: 'alice@example.com',
        phone: '+15550100',
        display_name: 'Alice A',
        billing_address: '42 Billing Rd',
      }),
    });
    expect(res.status).toBe(200);
    const body = (await res.json()) as Record<string, unknown>;
    expect(body.email).toBe('alice@example.com');
    expect(body.phone).toBe('+15550100');
    expect(body.display_name).toBe('Alice A');
    expect(body.billing_address).toBe('42 Billing Rd');
    expect(body.username).toBe('alice');

    const row = raw.prepare('SELECT email, phone, display_name, billing_address FROM users WHERE id = 2').get() as {
      email: string;
      phone: string;
      display_name: string;
      billing_address: string;
    };
    expect(row.email).toBe('alice@example.com');
    expect(row.display_name).toBe('Alice A');
  });

  it('rejects invalid email format', async () => {
    const { req, auth } = setup();
    const res = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({ email: 'not-an-email' }),
    });
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: 'invalid email format' });
  });

  it('rejects duplicate email', async () => {
    const { req, auth } = setup();
    const res = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({ email: 'taken@example.com' }),
    });
    expect(res.status).toBe(409);
    expect(await res.json()).toEqual({ error: 'email already exists' });
  });

  it('clears email with empty string; username login field still works', async () => {
    const { req, auth, raw } = setup();
    raw.exec("UPDATE users SET email = 'alice@example.com' WHERE id = 2");
    const clear = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({ email: '' }),
    });
    expect(clear.status).toBe(200);
    expect(((await clear.json()) as Record<string, unknown>).email).toBeNull();

    const rename = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({ username: 'alice2' }),
    });
    expect(rename.status).toBe(200);
    expect(((await rename.json()) as Record<string, unknown>).username).toBe('alice2');
  });

  it('requires at least one allowlisted field', async () => {
    const { req, auth } = setup();
    const res = await req('/user/profile', {
      method: 'PUT',
      headers: await auth(2),
      body: JSON.stringify({ role: 'admin' }),
    });
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: 'no valid fields to update' });
  });
});
