// 服務資格與開通 — 訂閱鏈接、/client/subscription、節點用戶列表必須共用同一判定。
// expire_time = 0 表示永不過期；traffic_limit_bytes = 0 表示不限流量。

export type EntitlementUser = {
  status: string | null;
  subscription_status: string | null;
  expire_time: number | null;
  traffic_limit_bytes: number | null;
  traffic_used_bytes: number | null;
};

export type BlockReason = 'ACCOUNT_DISABLED' | 'SUBSCRIPTION_PENDING' | 'SUBSCRIPTION_EXPIRED' | 'TRAFFIC_EXCEEDED';

export function serviceBlock(u: EntitlementUser, now: number): BlockReason | null {
  if ((u.status ?? 'active') !== 'active') return 'ACCOUNT_DISABLED';
  const sub = u.subscription_status ?? '';
  if (sub === 'pending') return 'SUBSCRIPTION_PENDING';
  if (sub !== 'active') return 'SUBSCRIPTION_EXPIRED';
  const expire = u.expire_time ?? 0;
  if (expire !== 0 && expire <= now) return 'SUBSCRIPTION_EXPIRED';
  const limit = u.traffic_limit_bytes ?? 0;
  if (limit > 0 && (u.traffic_used_bytes ?? 0) >= limit) return 'TRAFFIC_EXCEEDED';
  return null;
}

// 與 serviceBlock 等價的 SQL 條件（users 表，綁定一個參數 now），用於批量篩選節點用戶
export const SERVICEABLE_SQL =
  "COALESCE(status, 'active') = 'active' AND subscription_status = 'active'" +
  ' AND (COALESCE(expire_time, 0) = 0 OR expire_time > ?)' +
  ' AND (COALESCE(traffic_limit_bytes, 0) = 0 OR COALESCE(traffic_used_bytes, 0) < traffic_limit_bytes)';

export type ProductPlan = {
  name: string;
  duration_days: number | null;
  traffic_bytes: number | null;
  speed_limit_bps: number | null;
};

const DAY = 86400;

// 開通/續費：到期時間從 max(now, 舊到期) 順延；流量額度重置為商品值並清零已用。
// gate 為附加的 WHERE 條件（如支付回調的「訂單仍為 pending」），保證冪等。
export function activationStatement(
  db: D1Database,
  userId: number,
  plan: ProductPlan,
  now: number,
  gate?: { sql: string; binds: unknown[] },
): D1PreparedStatement {
  const days = plan.duration_days && plan.duration_days > 0 ? plan.duration_days : 30;
  return db
    .prepare(
      "UPDATE users SET subscription_status = 'active', subscription_tier = ?," +
        ' traffic_limit_bytes = ?, rate_limit_bps = ?, traffic_used_bytes = 0, traffic_period_start = ?,' +
        ' expire_time = MAX(?, COALESCE(expire_time, 0)) + ?, updated_at = ?' +
        ` WHERE id = ?${gate ? ` AND ${gate.sql}` : ''}`,
    )
    .bind(
      plan.name,
      plan.traffic_bytes ?? 0,
      plan.speed_limit_bps ?? 0,
      now,
      now,
      days * DAY,
      new Date(now * 1000).toISOString(),
      userId,
      ...(gate?.binds ?? []),
    );
}

const TRAFFIC_PERIOD = 30 * DAY;

// Cron：標記已過期用戶；按 30 天週期重置流量（長週期商品的流量額度按月計）
export async function runEntitlementMaintenance(db: D1Database, now: number): Promise<void> {
  const ts = new Date(now * 1000).toISOString();
  await db.batch([
    db
      .prepare(
        "UPDATE users SET subscription_status = 'expired', updated_at = ?" +
          " WHERE subscription_status = 'active' AND COALESCE(expire_time, 0) > 0 AND expire_time <= ?",
      )
      .bind(ts, now),
    db
      .prepare(
        'UPDATE users SET traffic_used_bytes = 0,' +
          ' traffic_period_start = traffic_period_start + CAST((? - traffic_period_start) / ? AS INTEGER) * ?, updated_at = ?' +
          " WHERE subscription_status = 'active' AND traffic_period_start > 0 AND ? - traffic_period_start >= ?",
      )
      .bind(now, TRAFFIC_PERIOD, TRAFFIC_PERIOD, ts, now, TRAFFIC_PERIOD),
  ]);
}
