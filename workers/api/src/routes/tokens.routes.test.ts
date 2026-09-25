// /admin/tokens（at_ 自建訂閱令牌）路由面：簾發 → 訂閱可用 → 列表 → 吊銷 403 → 續期換行 → 鑑權。
// 建庫走 createTestD1（readdirSync 按序執行 migrations/*.sql，含 0009_token_only）。
import { describe, expect, it } from 'vitest';
import { createApp, type Env } from '../index';
import { signJwt } from '../lib/jwt';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';
const GB = 1073741824;

function setup() {
  const { db, raw } = createTestD1();
  raw.exec(
    "INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'x', 'admin');" +
      "INSERT INTO users (id, username, password_hash, role) VALUES (2, 'alice', 'x', 'user');" +
      // active 節點：訂閱拉取需至少一個可編碼節點（照 node.routes.test.ts 插法）
      "INSERT INTO nodes (id, name, type, address, port, protocol, status, user_id, network, security, ws_path, token) VALUES (9, 'hk', 'xray', 'node-hk.example.com', 20001, 'vless', 'active', 1, 'tcp', 'reality', '/ws', 'nd_test');",
  );
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
  } as unknown as Env;
  const app = createApp();

  const req = async (method: string, path: string, body?: unknown, token?: string) => {
    const jwt = token ?? (await signJwt({ user_id: 1, username: 'admin', role: 'admin' }, SECRET, 3600));
    return app.request(
      `/api/v1${path}`,
      {
        method,
        headers: { Authorization: `Bearer ${jwt}`, 'Content-Type': 'application/json' },
        body: body === undefined ? undefined : JSON.stringify(body),
      },
      env,
    );
  };

  // 簾發一個 token，返回響應 JSON
  const issue = async (body: Record<string, unknown> = { duration_days: 30, traffic_limit_gb: 10, note: 't1' }) => {
    const res = await req('POST', '/admin/tokens', body);
    expect(res.status).toBe(201);
    return (await res.json()) as {
      token: string;
      id: number;
      username: string;
      note: string | null;
      client_token_prefix: string;
    };
  };

  const sub = (token: string) => app.request(`/api/v1/client/links/${token}`, {}, env);

  return { raw, app, env, req, issue, sub };
}

describe('POST /admin/tokens', () => {
  it('簾發 → 201，at_ token；token_only 合成用戶（expire=now+days、note 入 display_name、email NULL）', async () => {
    const s = setup();
    const body = await s.issue({ duration_days: 30, traffic_limit_gb: 10, note: 'friend' });
    expect(body.token).toMatch(/^at_[0-9a-f]{64}$/);
    expect(body.username).toMatch(/^at_[0-9a-f]{12}$/);
    expect(body.note).toBe('friend');
    expect(body.client_token_prefix).toBe(body.token.slice(0, 11));

    const row = s.raw
      .prepare(
        'SELECT account_type a, display_name n, subscription_status st, email, password_hash p, ' +
          'traffic_limit_bytes t, expire_time e, client_token c FROM users WHERE id = ?',
      )
      .get(body.id) as Record<string, unknown>;
    expect(row.a).toBe('token_only');
    expect(row.n).toBe('friend');
    expect(row.st).toBe('active');
    expect(row.email).toBeNull();
    expect(row.c).toBe(body.token);
    expect(String(row.p)).toMatch(/^\$2[aby]\$/); // bcrypt（不可登入的隨機密碼）
    expect(row.t).toBe(10 * GB);
    const now = Math.floor(Date.now() / 1000);
    expect(row.e as number).toBeGreaterThan(now + 29 * 86400);
    expect(row.e as number).toBeLessThanOrEqual(now + 30 * 86400);
  });

  it.each([
    ['duration_days 非正整數', { duration_days: 0, traffic_limit_gb: 1 }],
    ['duration_days 非整數', { duration_days: 1.5, traffic_limit_gb: 1 }],
    ['traffic_limit_gb 非正數', { duration_days: 1, traffic_limit_gb: -5 }],
    ['note 超長', { duration_days: 1, traffic_limit_gb: 1, note: 'x'.repeat(201) }],
  ])('%s → 400', async (_name, body) => {
    const s = setup();
    const res = await s.req('POST', '/admin/tokens', body);
    expect(res.status).toBe(400);
  });
});

describe('token 訂閱生命週期', () => {
  it('簾發後 GET /client/links/:token → 200；列表含該行（prefix 11 字符）', async () => {
    const s = setup();
    const t = await s.issue();

    const sub = await s.sub(t.token);
    expect(sub.status).toBe(200);
    expect((await sub.text()).length).toBeGreaterThan(0);

    const list = await s.req('GET', '/admin/tokens');
    expect(list.status).toBe(200);
    const data = ((await list.json()) as { data: Record<string, unknown>[] }).data;
    const row = data.find((r) => r.id === t.id);
    expect(row).toMatchObject({
      username: t.username,
      note: 't1',
      subscription_status: 'active',
      client_token_prefix: t.token.slice(0, 11),
    });
  });

  it('吊銷 → suspended；原 token 訂閱 → 403；registered 用戶 / 未知 id → 404', async () => {
    const s = setup();
    const t = await s.issue();
    expect((await s.sub(t.token)).status).toBe(200);

    expect((await s.req('POST', `/admin/tokens/${t.id}/revoke`)).status).toBe(200);
    expect(
      s.raw.prepare('SELECT subscription_status s FROM users WHERE id = ?').get(t.id),
    ).toEqual({ s: 'suspended' });
    expect((await s.sub(t.token)).status).toBe(403);

    // alice 是 registered，不是 token → 404
    expect((await s.req('POST', '/admin/tokens/2/revoke')).status).toBe(404);
    expect((await s.req('POST', '/admin/tokens/999/revoke')).status).toBe(404);
  });

  it('renew → 舊行 suspended，新 token 訂閱 200，note 沿用舊行', async () => {
    const s = setup();
    const old = await s.issue({ duration_days: 30, traffic_limit_gb: 10, note: 'keepme' });

    const res = await s.req('POST', `/admin/tokens/${old.id}/renew`, { duration_days: 7, traffic_limit_gb: 5 });
    expect(res.status).toBe(201);
    const fresh = (await res.json()) as { token: string; id: number; note: string | null; traffic_limit_bytes: number };
    expect(fresh.token).toMatch(/^at_[0-9a-f]{64}$/);
    expect(fresh.token).not.toBe(old.token);
    expect(fresh.note).toBe('keepme');
    expect(fresh.traffic_limit_bytes).toBe(5 * GB);

    expect(
      s.raw.prepare('SELECT subscription_status s FROM users WHERE id = ?').get(old.id),
    ).toEqual({ s: 'suspended' });
    expect((await s.sub(old.token)).status).toBe(403);
    expect((await s.sub(fresh.token)).status).toBe(200);
  });
});

describe('鑑權', () => {
  it('假 Bearer + admin cookie 簽發仍要 CSRF，不建 token 用戶', async () => {
    const s = setup();
    const session = await signJwt({ user_id: 1, username: 'admin', role: 'admin' }, SECRET, 3600);
    const before = (s.raw.prepare('SELECT COUNT(*) AS n FROM users').get() as { n: number }).n;
    const res = await s.app.request(
      '/api/v1/admin/tokens',
      {
        method: 'POST',
        headers: {
          Cookie: `admin_session=${session}`,
          Authorization: 'Bearer not-a-jwt',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ duration_days: 1, traffic_limit_gb: 1 }),
      },
      s.env,
    );
    expect(res.status).toBe(403);
    expect(await res.json()).toEqual({ error: 'CSRF_INVALID' });
    expect((s.raw.prepare('SELECT COUNT(*) AS n FROM users').get() as { n: number }).n).toBe(before);
  });

  it('admin cookie + 雙提交 CSRF 可以簽發', async () => {
    const s = setup();
    const session = await signJwt({ user_id: 1, username: 'admin', role: 'admin' }, SECRET, 3600);
    const res = await s.app.request(
      '/api/v1/admin/tokens',
      {
        method: 'POST',
        headers: {
          Cookie: `admin_session=${session}; admin_csrf=ct`,
          'X-CSRF-Token': 'ct',
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ duration_days: 1, traffic_limit_gb: 1 }),
      },
      s.env,
    );
    expect(res.status).toBe(201);
  });

  it('非 admin → 403；無憑證 → 401', async () => {
    const s = setup();
    const userJwt = await signJwt({ user_id: 2, username: 'alice', role: 'user' }, SECRET, 3600);
    expect((await s.req('GET', '/admin/tokens', undefined, userJwt)).status).toBe(403);
    expect(
      (await s.req('POST', '/admin/tokens', { duration_days: 1, traffic_limit_gb: 1 }, userJwt)).status,
    ).toBe(403);
    expect((await s.app.request('/api/v1/admin/tokens', {}, s.env)).status).toBe(401);
  });
});
