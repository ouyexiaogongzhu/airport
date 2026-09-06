// activateSubscription 冪等契約測試 — 用最小 D1 fake 釘住兩個性質：
//   1. pending 訂單回調 → 激活（paid + 順延 30d）；重複回調零副作用
//   2. 用戶更新必須以「訂單仍為 pending」為閘門（併發重複回調不得雙重順延）——
//      fake 按語句求值順序評估閘門，若有人把訂單翻轉挪回用戶更新之前，第一次調用即失敗
// 運行：workers/api 下 `npx vitest run src/lib/payments.test.ts`

import { describe, it, expect } from 'vitest';
import { activateSubscription } from './payments';

const GB_BYTES = 1073741824;
const DUR = 30 * 86400;

type UserRow = {
  id: number;
  subscription_status: string;
  subscription_tier: string | null;
  traffic_limit_bytes: number;
  traffic_used_bytes: number;
  expire_time: number;
  traffic_period_start: number;
};
type OrderRow = { id: number; user_id: number; product_id: number; amount: number; status: string };
type Stmt = { sql: string; args: unknown[] };

function makeFakeDb(users: Map<number, UserRow>, orders: Map<number, OrderRow>): D1Database {
  const products = new Map([[1, { name: 'Pro 30d' }]]);

  const evalUpdate = (s: Stmt): number => {
    if (s.sql.startsWith('UPDATE users')) {
      const [tier, limit, periodStart, now, dur, userId] = s.args as [string, number, number, number, number, number];
      // 閘門：EXISTS(orders WHERE id=? AND status='pending')，按語句求值當下判斷（args 末位是 orderId）
      const gateOrderId = s.args[s.args.length - 1] as number;
      const gate = orders.get(gateOrderId);
      if (gate?.status !== 'pending') return 0;
      const u = users.get(userId);
      if (!u) return 0;
      u.subscription_status = 'active';
      u.subscription_tier = tier;
      u.traffic_limit_bytes = limit;
      u.traffic_used_bytes = 0;
      u.traffic_period_start = periodStart;
      u.expire_time = Math.max(now, u.expire_time) + dur;
      return 1;
    }
    if (s.sql.startsWith('UPDATE orders')) {
      const [, orderId] = s.args as [unknown, number];
      const o = orders.get(orderId);
      if (!o || o.status !== 'pending') return 0;
      o.status = 'paid';
      return 1;
    }
    throw new Error('unexpected sql in fake: ' + s.sql);
  };

  const prepare = (sql: string) => ({
    bind: (...args: unknown[]) => ({
      sql,
      args,
      // activateSubscription 的 SELECT（orders JOIN products）走這裡
      first: async <T,>(): Promise<T | null> => {
        if (!sql.startsWith('SELECT o.amount')) return null;
        const [orderId, userId, productId] = args as [number, number, number];
        const o = orders.get(orderId);
        const p = products.get(productId);
        if (!o || o.user_id !== userId || !p) return null;
        return { amount: o.amount, name: p.name } as T;
      },
      run: async () => ({ meta: { changes: evalUpdate({ sql, args }) } }),
    }),
  });

  // db.batch 收到的是 prepare(...).bind(...) 產物（含 sql/args），按序求值 = 單一事務語義
  return {
    prepare,
    batch: async (stmts: Stmt[]) => stmts.map((s) => ({ meta: { changes: evalUpdate(s) } })),
  } as unknown as D1Database;
}

function seed(): { users: Map<number, UserRow>; orders: Map<number, OrderRow> } {
  const users = new Map([
    [
      7,
      {
        id: 7,
        subscription_status: 'pending',
        subscription_tier: null,
        traffic_limit_bytes: 0,
        traffic_used_bytes: 0,
        expire_time: 0,
        traffic_period_start: 0,
      } as UserRow,
    ],
  ]);
  const orders = new Map([[100, { id: 100, user_id: 7, product_id: 1, amount: 10, status: 'pending' } as OrderRow]]);
  return { users, orders };
}

describe('activateSubscription', () => {
  it('activates pending order: paid + expire=now+30d + tier/limit set', async () => {
    const { users, orders } = seed();
    await activateSubscription(makeFakeDb(users, orders), 100, 7, 1);
    expect(orders.get(100)!.status).toBe('paid');
    const u = users.get(7)!;
    expect(u.subscription_status).toBe('active');
    expect(u.subscription_tier).toBe('Pro 30d');
    expect(u.traffic_limit_bytes).toBe(Math.trunc(10 * GB_BYTES));
    expect(u.expire_time).toBeGreaterThanOrEqual(Math.floor(Date.now() / 1000) + DUR - 5);
  });

  it('duplicate callback is a no-op (expire unchanged)', async () => {
    const { users, orders } = seed();
    const db = makeFakeDb(users, orders);
    await activateSubscription(db, 100, 7, 1);
    const before = users.get(7)!.expire_time;
    await activateSubscription(db, 100, 7, 1);
    expect(users.get(7)!.expire_time).toBe(before);
  });

  it('renewal extends from existing expire_time (顺延), not from now', async () => {
    const { users, orders } = seed();
    const u = users.get(7)!;
    u.expire_time = 2000000000; // 遠在未來
    await activateSubscription(makeFakeDb(users, orders), 100, 7, 1);
    expect(u.expire_time).toBe(2000000000 + DUR);
  });

  it('throws when order/user/product do not line up', async () => {
    const { users, orders } = seed();
    await expect(activateSubscription(makeFakeDb(users, orders), 999, 7, 1)).rejects.toThrow(
      'order or product not found',
    );
  });
});
