import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { createTestD1 } from '../testing/d1';
import {
  SERVICEABLE_SQL,
  activationStatement,
  runEntitlementMaintenance,
  serviceBlock,
  type EntitlementUser,
} from './entitlement';

const NOW = 1_800_000_000;
const DAY = 86400;

const base: EntitlementUser = {
  status: 'active',
  subscription_status: 'active',
  expire_time: NOW + DAY,
  traffic_limit_bytes: 100,
  traffic_used_bytes: 10,
};

function insertUser(raw: ReturnType<typeof createTestD1>['raw'], id: number, u: Partial<EntitlementUser> & Record<string, unknown> = {}) {
  const row = { ...base, ...u };
  raw
    .prepare(
      'INSERT INTO users (id, username, password_hash, status, subscription_status, expire_time, traffic_limit_bytes, traffic_used_bytes, traffic_period_start, vless_uuid) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
    )
    .run(
      id,
      `u${id}`,
      'x',
      row.status,
      row.subscription_status,
      row.expire_time,
      row.traffic_limit_bytes,
      row.traffic_used_bytes,
      (u.traffic_period_start as number | undefined) ?? 0,
      'vless_uuid' in u ? (u.vless_uuid as string | null) : `uuid-${id}`,
    );
}

describe('serviceBlock', () => {
  it('有效用戶可用', () => {
    expect(serviceBlock(base, NOW)).toBeNull();
  });

  it.each([
    [{ status: 'banned' }, 'ACCOUNT_DISABLED'],
    [{ status: 'suspended' }, 'ACCOUNT_DISABLED'],
    [{ subscription_status: 'pending' }, 'SUBSCRIPTION_PENDING'],
    [{ subscription_status: 'expired' }, 'SUBSCRIPTION_EXPIRED'],
    [{ expire_time: NOW }, 'SUBSCRIPTION_EXPIRED'],
    [{ traffic_used_bytes: 100 }, 'TRAFFIC_EXCEEDED'],
  ] as const)('%o → %s', (patch, reason) => {
    expect(serviceBlock({ ...base, ...patch }, NOW)).toBe(reason);
  });

  it('expire_time=0 永不過期、traffic_limit=0 不限流量', () => {
    expect(serviceBlock({ ...base, expire_time: 0, traffic_limit_bytes: 0, traffic_used_bytes: 1e15 }, NOW)).toBeNull();
  });

  it('SERVICEABLE_SQL 與 serviceBlock 在所有組合上結論一致', async () => {
    const { db, raw } = createTestD1();
    const statuses = ['active', 'banned'];
    const subs = ['active', 'pending', 'expired'];
    const expires = [0, NOW - 1, NOW, NOW + 1];
    const traffic: [number, number][] = [[0, 999], [100, 99], [100, 100]];
    const expected = new Map<number, boolean>();
    let id = 1;
    for (const status of statuses)
      for (const subscription_status of subs)
        for (const expire_time of expires)
          for (const [traffic_limit_bytes, traffic_used_bytes] of traffic) {
            const u = { status, subscription_status, expire_time, traffic_limit_bytes, traffic_used_bytes };
            insertUser(raw, id, u);
            expected.set(id, serviceBlock(u, NOW) === null);
            id++;
          }
    const { results } = await db.prepare(`SELECT id FROM users WHERE ${SERVICEABLE_SQL}`).bind(NOW).all<{ id: number }>();
    const got = new Set(results.map((r) => r.id));
    for (const [uid, ok] of expected) expect([uid, got.has(uid)]).toEqual([uid, ok]);
  });
});

describe('activationStatement', () => {
  const plan = { name: 'Pro', duration_days: 90, traffic_bytes: 500, speed_limit_bps: 1000 };

  it('新開通：到期 = now + 時長，額度按商品，已用清零', async () => {
    const { db, raw } = createTestD1();
    insertUser(raw, 1, { subscription_status: 'pending', expire_time: 0, traffic_used_bytes: 77 });
    await activationStatement(db, 1, plan, NOW).run();
    const u = raw.prepare('SELECT * FROM users WHERE id = 1').get()!;
    expect(u).toMatchObject({
      subscription_status: 'active',
      subscription_tier: 'Pro',
      expire_time: NOW + 90 * DAY,
      traffic_limit_bytes: 500,
      traffic_used_bytes: 0,
      rate_limit_bps: 1000,
      traffic_period_start: NOW,
    });
  });

  it('續費從未到期的舊到期時間順延', async () => {
    const { db, raw } = createTestD1();
    insertUser(raw, 1, { expire_time: NOW + 10 * DAY });
    await activationStatement(db, 1, plan, NOW).run();
    expect(raw.prepare('SELECT expire_time FROM users WHERE id = 1').get()!.expire_time).toBe(NOW + 100 * DAY);
  });

  it('gate 不滿足時不更新', async () => {
    const { db, raw } = createTestD1();
    insertUser(raw, 1, { subscription_status: 'pending' });
    const r = await activationStatement(db, 1, plan, NOW, { sql: '1 = 0', binds: [] }).run();
    expect(r.meta.changes).toBe(0);
  });
});

describe('runEntitlementMaintenance', () => {
  it('標記已過期用戶，不動永久與未到期用戶', async () => {
    const { db, raw } = createTestD1();
    insertUser(raw, 1, { expire_time: NOW - 1 });
    insertUser(raw, 2, { expire_time: 0 });
    insertUser(raw, 3, { expire_time: NOW + 1 });
    await runEntitlementMaintenance(db, NOW);
    const subs = raw.prepare('SELECT id, subscription_status s FROM users ORDER BY id').all().map((r) => r.s);
    expect(subs).toEqual(['expired', 'active', 'active']);
  });

  it('滿 30 天重置已用流量，週期起點對齊到最近一個週期', async () => {
    const { db, raw } = createTestD1();
    insertUser(raw, 1, { traffic_used_bytes: 50, traffic_period_start: NOW - 65 * DAY, expire_time: 0 });
    insertUser(raw, 2, { traffic_used_bytes: 50, traffic_period_start: NOW - 29 * DAY, expire_time: 0 });
    await runEntitlementMaintenance(db, NOW);
    const [u1, u2] = raw.prepare('SELECT traffic_used_bytes used, traffic_period_start start FROM users ORDER BY id').all();
    expect(u1).toEqual({ used: 0, start: NOW - 5 * DAY });
    expect(u2).toEqual({ used: 50, start: NOW - 29 * DAY });
  });
});

describe('migration 0002', () => {
  it('補齊缺失的 vless_uuid 為 UUIDv4 格式', async () => {
    const { raw } = createTestD1();
    insertUser(raw, 1, { vless_uuid: null });
    const migration = readFileSync(new URL('../../migrations/0002_entitlement.sql', import.meta.url), 'utf8');
    raw.exec(migration.match(/UPDATE users SET vless_uuid[\s\S]*?;/)![0]);
    const v = raw.prepare('SELECT vless_uuid v FROM users WHERE id = 1').get()!.v as string;
    expect(v).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });
});
