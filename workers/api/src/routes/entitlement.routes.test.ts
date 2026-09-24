// 端到端：後台建商品 → 手動開通 → 訂閱鏈接可用；超額/封禁/過期時鏈接被拒
import { describe, expect, it } from 'vitest';
import { createApp, type Env } from '../index';
import { signJwt } from '../lib/jwt';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';
const DAY = 86400;

async function setup() {
  const { db, raw } = createTestD1();
  raw.exec(
    "INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'x', 'admin');" +
      "INSERT INTO users (id, username, password_hash, client_token, vless_uuid) VALUES (2, 'alice', 'x', 'rf_alice', '11111111-2222-4333-8444-555555555555');" +
      "INSERT INTO nodes (name, type, address, port, protocol, status, user_id, network, security, ws_path) VALUES ('hk', 'cf', 'hk.example.com', 443, 'vless', 'active', 1, 'xhttp', 'tls', '/rfhttp/');",
  );
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
  } as unknown as Env;
  const app = createApp();
  const token = await signJwt({ user_id: 1, username: 'admin', role: 'admin' }, SECRET, 3600);
  const admin = (path: string, method: string, body?: unknown) =>
    app.request(
      `/api/v1${path}`,
      {
        method,
        headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
      },
      env,
    );
  const links = () => app.request('/api/v1/client/links/rf_alice', {}, env);
  return { raw, admin, links };
}

describe('entitlement routes', () => {
  it('建商品 → 開通 → 鏈接可用；權益取自商品', async () => {
    const { admin, links } = await setup();
    expect((await links()).status).toBe(403);

    const created = await admin('/admin/products', 'POST', {
      name: 'Quarter', type: 'quarterly', price: 30, currency: 'CNY',
      duration_days: 90, traffic_bytes: 200 * 1024 ** 3, speed_limit_bps: 0,
    });
    expect(created.status).toBe(201);
    const { product } = (await created.json()) as { product: Record<string, unknown> };
    expect(product).toMatchObject({ currency: 'CNY', duration_days: 90, traffic_bytes: 200 * 1024 ** 3 });

    const granted = await admin('/admin/users/2/grant', 'POST', { product_id: product.id });
    expect(granted.status).toBe(200);
    const user = (await granted.json()) as Record<string, number | string>;
    const now = Math.floor(Date.now() / 1000);
    expect(user.subscription_status).toBe('active');
    expect(user.expire_time as number).toBeGreaterThanOrEqual(now + 90 * DAY - 5);

    const res = await links();
    expect(res.status).toBe(200);
    expect(atob(await res.text())).toContain('vless://11111111-2222-4333-8444-555555555555@hk.example.com:443');
  });

  it.each([
    ['traffic_used_bytes = traffic_limit_bytes', 'TRAFFIC_EXCEEDED'],
    ["status = 'banned'", 'ACCOUNT_DISABLED'],
    ['expire_time = 1', 'SUBSCRIPTION_EXPIRED'],
  ])('%s → 403 %s', async (patch, reason) => {
    const { raw, links } = await setup();
    raw.exec(
      `UPDATE users SET subscription_status = 'active', expire_time = 0, traffic_limit_bytes = 100, traffic_used_bytes = 1 WHERE id = 2;` +
        `UPDATE users SET ${patch} WHERE id = 2;`,
    );
    const res = await links();
    expect(res.status).toBe(403);
    expect(await res.json()).toEqual({ error: reason });
  });

  it('後台校驗非法欄位', async () => {
    const { admin } = await setup();
    expect((await admin('/admin/users/2', 'PUT', { expire_time: -1 })).status).toBe(400);
    expect((await admin('/admin/users/2', 'PUT', { subscription_status: 'vip' })).status).toBe(400);
    expect((await admin('/admin/products', 'POST', { name: 'x', price: 1, currency: 'EUR' })).status).toBe(400);
    expect((await admin('/admin/products', 'POST', { name: 'x', price: 1, duration_days: 0 })).status).toBe(400);
    expect((await admin('/admin/users/2/grant', 'POST', { product_id: 999 })).status).toBe(404);
  });

  it('後台可重置已用流量與到期時間', async () => {
    const { admin, raw } = await setup();
    const res = await admin('/admin/users/2', 'PUT', { traffic_used_bytes: 0, expire_time: 0, subscription_status: 'active' });
    expect(res.status).toBe(200);
    expect(raw.prepare('SELECT subscription_status s, expire_time e FROM users WHERE id = 2').get()).toEqual({ s: 'active', e: 0 });
  });
});
