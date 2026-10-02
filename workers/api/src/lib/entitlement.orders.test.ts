// 棄單佔用庫存的回收：建單即扣 stock，但只有插單失敗 / 取支付連結失敗會回補，
// 用戶建單不付款就永久鎖住庫存 —— 任何登入用戶反覆建單即可清空。
import { describe, expect, it } from 'vitest';
import { createTestD1 } from '../testing/d1';
import { runEntitlementMaintenance } from './entitlement';

const NOW = 1_800_000_000;
const MINUTE = 60;

function setup() {
  const { db, raw } = createTestD1();
  raw
    .prepare(
      "INSERT INTO users (id, username, password_hash, role, status, subscription_status) VALUES (2, 'alice', 'x', 'user', 'active', 'active')",
    )
    .run();
  raw.prepare("INSERT INTO products (id, name, type, price, stock) VALUES (1, 'p', 'traffic', 10, 5)").run();
  const order = (at: number, status = 'pending') =>
    raw
      .prepare(
        'INSERT INTO orders (user_id, product_id, amount, status, provider, created_at, updated_at) VALUES (2, 1, 10, ?, ?, ?, ?)',
      )
      .run(status, 'mock', new Date(at * 1000).toISOString(), new Date(at * 1000).toISOString());
  const stock = () => (raw.prepare('SELECT stock FROM products WHERE id = 1').get() as { stock: number }).stock;
  return { db, raw, order, stock };
}

describe('runEntitlementMaintenance abandoned orders', () => {
  it('reclaims stock held by unpaid orders, and only once', async () => {
    const { db, order, stock } = setup();
    const stale = NOW - 31 * MINUTE;
    order(stale);
    order(stale);
    order(stale);
    order(NOW - 5 * MINUTE); // 還在付款窗口內，必須保留
    expect(stock()).toBe(5);

    await runEntitlementMaintenance(db, NOW);
    // 三張逾時單各回補 1，近期單不動
    expect(stock()).toBe(8);

    // 再跑一次不得重複加（回補與改狀態在同一原子 batch 內）
    await runEntitlementMaintenance(db, NOW);
    expect(stock()).toBe(8);
  });

  it('不動已付款訂單的庫存', async () => {
    const { db, raw, order, stock } = setup();
    order(NOW - 31 * MINUTE, 'paid');
    expect(stock()).toBe(5);

    await runEntitlementMaintenance(db, NOW);
    expect(stock()).toBe(5);
    const rows = raw.prepare("SELECT status FROM orders WHERE status = 'paid'").all();
    expect(rows.length).toBe(1);
  });
});
