// RegenerateClientToken 不變量：只換 client_token，權益 / UUID / 流量計數不得變動。
import { describe, expect, it } from 'vitest';
import bcrypt from 'bcryptjs';
import { createApp, type Env } from '../index';
import { signTokens } from '../lib/session';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';
const PASSWORD = 'password123';

const ENTITLEMENT_COLS =
  'client_token, traffic_used_bytes, traffic_limit_bytes, expire_time, rate_limit_bps, ' +
  'traffic_period_start, subscription_status, subscription_tier, vless_uuid, token_version, status';

type EntitlementSnapshot = {
  client_token: string;
  traffic_used_bytes: number;
  traffic_limit_bytes: number;
  expire_time: number;
  rate_limit_bps: number;
  traffic_period_start: number;
  subscription_status: string;
  subscription_tier: string | null;
  vless_uuid: string;
  token_version: number;
  status: string;
};

function setup() {
  const { db, raw } = createTestD1();
  const hash = bcrypt.hashSync(PASSWORD, 4);
  raw
    .prepare(
      'INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status, ' +
        'subscription_tier, traffic_limit_bytes, traffic_used_bytes, expire_time, rate_limit_bps, ' +
        'traffic_period_start, token_version) VALUES (2, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
    )
    .run(
      'alice',
      hash,
      'rf_old_token_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
      '11111111-2222-4333-8444-555555555555',
      'active',
      'pro',
      2147483648, // 2 GiB
      214748365, // ~0.2 GiB used
      4102444800,
      1048576,
      1700000000,
      0,
    );
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
    TURNSTILE_DISABLED: '1',
  } as unknown as Env;
  const app = createApp();
  const req = (path: string, init: RequestInit = {}) => app.request(`/api/v1${path}`, init, env);
  const snapshot = (): EntitlementSnapshot =>
    raw.prepare(`SELECT ${ENTITLEMENT_COLS} FROM users WHERE id = 2`).get() as EntitlementSnapshot;
  return { raw, req, snapshot, tokens: () => signTokens({ id: 2, username: 'alice', role: 'user', token_version: 0 }, SECRET) };
}

describe('POST /web/client-token/regenerate', () => {
  it('只更新 client_token + updated_at，不改動流量與權益欄位，也不換 vless_uuid', async () => {
    const { raw, req, snapshot, tokens } = setup();
    const before = snapshot();
    const updatedAtBefore = raw.prepare('SELECT updated_at u FROM users WHERE id = 2').get() as { u: string | null };

    const { bearer } = await tokens();
    const res = await req('/web/client-token/regenerate', {
      method: 'POST',
      headers: { Authorization: `Bearer ${bearer}`, 'Content-Type': 'application/json' },
    });
    expect(res.status).toBe(200);
    const body = (await res.json()) as { token: string };
    expect(body.token).toMatch(/^rf_[0-9a-f]{64}$/);
    expect(body.token).not.toBe(before.client_token);

    const after = snapshot();
    expect(after.client_token).toBe(body.token);
    expect(after.traffic_used_bytes).toBe(before.traffic_used_bytes);
    expect(after.traffic_limit_bytes).toBe(before.traffic_limit_bytes);
    expect(after.expire_time).toBe(before.expire_time);
    expect(after.rate_limit_bps).toBe(before.rate_limit_bps);
    expect(after.traffic_period_start).toBe(before.traffic_period_start);
    expect(after.subscription_status).toBe(before.subscription_status);
    expect(after.subscription_tier).toBe(before.subscription_tier);
    expect(after.vless_uuid).toBe(before.vless_uuid);
    expect(after.token_version).toBe(before.token_version);
    expect(after.status).toBe(before.status);

    const updatedAtAfter = raw.prepare('SELECT updated_at u FROM users WHERE id = 2').get() as { u: string };
    expect(updatedAtAfter.u).toBeTruthy();
    if (updatedAtBefore.u) expect(updatedAtAfter.u).not.toBe(updatedAtBefore.u);

    // 舊 token 失效；profile / 新訂閱鏈仍反映原 traffic_used_bytes
    expect((await req(`/client/links/${before.client_token}`)).status).toBe(401);
    const profile = await req('/user/profile', {
      headers: { Authorization: `Bearer ${bearer}` },
    });
    expect(profile.status).toBe(200);
    const p = (await profile.json()) as { traffic_used_bytes: number; client_token: string };
    expect(p.traffic_used_bytes).toBe(before.traffic_used_bytes);
    expect(p.client_token).toBe(body.token);

    raw.exec(
      "INSERT INTO nodes (id, name, type, address, port, protocol, status, user_id) " +
        "VALUES (1, 'n1', 'entry', '1.1.1.1', 443, 'vless', 'active', 1)",
    );
    const newLinks = await req(`/client/links/${body.token}`);
    expect(newLinks.status).toBe(200);
    expect(newLinks.headers.get('Subscription-Userinfo')).toBe(
      `upload=0; download=${before.traffic_used_bytes}; total=${before.traffic_limit_bytes}; expire=${before.expire_time}`,
    );
  });
});
