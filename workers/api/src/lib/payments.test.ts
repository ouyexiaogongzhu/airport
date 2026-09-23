// activateSubscription 冪等契約 — 在真實 SQLite（跑完 migrations）上驗證：
//   1. pending 訂單回調 → 按商品配置開通；重複回調零副作用
//   2. 用戶更新以「訂單仍為 pending」為閘門，且排在訂單翻轉之前（順序顛倒則第一次調用就不會開通）

import { describe, it, expect } from 'vitest';
import { createTestD1 } from '../testing/d1';
import { activateSubscription } from './payments';

const DAY = 86400;

function seed() {
  const { db, raw } = createTestD1();
  raw.exec(
    "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid) VALUES (7, 'alice', 'x', 'pending', 'uuid-7');" +
      "INSERT INTO products (id, name, type, price, duration_days, traffic_bytes, speed_limit_bps) VALUES (1, 'Pro 90d', 'subscription', 10, 90, 5000, 800);" +
      "INSERT INTO orders (id, user_id, product_id, amount, status) VALUES (100, 7, 1, 10, 'pending');",
  );
  const user = () => raw.prepare('SELECT * FROM users WHERE id = 7').get()!;
  const orderStatus = () => raw.prepare('SELECT status FROM orders WHERE id = 100').get()!.status;
  return { db, raw, user, orderStatus };
}

describe('activateSubscription', () => {
  it('按商品配置開通：paid + 到期 = now + duration_days，流量/限速取商品值', async () => {
    const { db, user, orderStatus } = seed();
    const now = Math.floor(Date.now() / 1000);
    await activateSubscription(db, 100, 7, 1);
    expect(orderStatus()).toBe('paid');
    const u = user();
    expect(u).toMatchObject({
      subscription_status: 'active',
      subscription_tier: 'Pro 90d',
      traffic_limit_bytes: 5000,
      rate_limit_bps: 800,
      traffic_used_bytes: 0,
    });
    expect(u.expire_time as number).toBeGreaterThanOrEqual(now + 90 * DAY);
    expect(u.expire_time as number).toBeLessThanOrEqual(now + 90 * DAY + 5);
  });

  it('重複回調零副作用', async () => {
    const { db, user } = seed();
    await activateSubscription(db, 100, 7, 1);
    const before = user().expire_time;
    await activateSubscription(db, 100, 7, 1);
    expect(user().expire_time).toBe(before);
  });

  it('續費從未到期的舊到期時間順延', async () => {
    const { db, raw, user } = seed();
    raw.exec("UPDATE users SET subscription_status = 'active', expire_time = 2000000000 WHERE id = 7");
    await activateSubscription(db, 100, 7, 1);
    expect(user().expire_time).toBe(2000000000 + 90 * DAY);
  });

  it('訂單/用戶/商品不匹配時拋錯', async () => {
    const { db } = seed();
    await expect(activateSubscription(db, 999, 7, 1)).rejects.toThrow('order or product not found');
    await expect(activateSubscription(db, 100, 8, 1)).rejects.toThrow('order or product not found');
  });
});
