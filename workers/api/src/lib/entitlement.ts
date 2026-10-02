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
  /** When set (incl. 0 = unlimited), copied onto users.max_devices on grant/pay. NULL = leave user as-is. */
  max_devices?: number | null;
};

const DAY = 86400;
/** 未付款訂單多久後回收庫存（cron 每小時跑，實際鎖定 30–90 分鐘） */
const ORDER_TTL = 1800;

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
  const applyMax = plan.max_devices != null && Number.isFinite(plan.max_devices);
  return db
    .prepare(
      "UPDATE users SET subscription_status = 'active', subscription_tier = ?," +
        ' traffic_limit_bytes = ?, rate_limit_bps = ?, traffic_used_bytes = 0, traffic_period_start = ?,' +
        ' expire_time = MAX(?, COALESCE(expire_time, 0)) + ?, updated_at = ?' +
        (applyMax ? ', max_devices = ?' : '') +
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
      ...(applyMax ? [plan.max_devices] : []),
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

  // traffic_records 保留 14 天：先聚合進 traffic_daily，同一 batch 內刪除明細（原子，防重複累加）
  const cutoff = new Date((now - 14 * DAY) * 1000).toISOString();
  const hasOld = await db
    .prepare('SELECT 1 FROM traffic_records WHERE recorded_at < ? LIMIT 1')
    .bind(cutoff)
    .first();
  if (hasOld) {
    const results = await db.batch([
      db
        .prepare(
          'INSERT INTO traffic_daily (day, upload_bytes, download_bytes, records) ' +
            'SELECT substr(recorded_at, 1, 10), SUM(upload_bytes), SUM(download_bytes), COUNT(*) ' +
            'FROM traffic_records WHERE recorded_at < ? GROUP BY substr(recorded_at, 1, 10) ' +
            'ON CONFLICT(day) DO UPDATE SET upload_bytes = upload_bytes + excluded.upload_bytes,' +
            ' download_bytes = download_bytes + excluded.download_bytes, records = records + excluded.records',
        )
        .bind(cutoff),
      db.prepare('DELETE FROM traffic_records WHERE recorded_at < ?').bind(cutoff),
    ]);
    const deleted = results[1]?.meta?.changes ?? 0;
    console.log(
      JSON.stringify({
        event: 'traffic_daily_maintenance',
        aggregated_rows: deleted,
        deleted_rows: deleted,
        cutoff,
      }),
    );
  }

  // traffic_batches 去重表只留 7 天；表可能尚不存在（0008 部署順序靠後、cron 可能先跑），吞錯
  try {
    await db
      .prepare('DELETE FROM traffic_batches WHERE recorded_at < ?')
      .bind(new Date((now - 7 * DAY) * 1000).toISOString())
      .run();
  } catch (e) {
    console.log(JSON.stringify({ event: 'traffic_batches_cleanup_skipped', error: String(e) }));
  }

  // 棄單佔用的庫存要回收：建單即扣 stock（web.ts:216），原本只有插單失敗與取支付連結失敗
  // 兩條路徑會回補；用戶建單後不付款，庫存就永久被鎖住 —— 任何登入用戶反覆建單即可清空。
  // 復用既有 hourly cron；orders 上已有 (status, created_at) 索引，無需遷移。
  // 回補與改狀態放同一 db.batch（原子），否則兩次 cron 會重複加庫存。
  // 用既有 'failed' 狀態，不新增枚舉值 —— payment.ts:127 已在用，且回調只在 pending 時激活，
  // 過期單的遲到回調會被 payment.ts:55 擋下。
  const orderCutoff = new Date((now - ORDER_TTL) * 1000).toISOString();
  const stale = await db
    .prepare("SELECT COUNT(*) AS n FROM orders WHERE status = 'pending' AND created_at < ?")
    .bind(orderCutoff)
    .first<{ n: number }>();
  if ((stale?.n ?? 0) > 0) {
    await db.batch([
      db
        .prepare(
          'UPDATE products SET stock = stock + (' +
            " SELECT COUNT(*) FROM orders o WHERE o.product_id = products.id AND o.status = 'pending' AND o.created_at < ?" +
            ' ), updated_at = ?' +
            " WHERE id IN (SELECT product_id FROM orders WHERE status = 'pending' AND created_at < ?)",
        )
        .bind(orderCutoff, ts, orderCutoff),
      db
        .prepare("UPDATE orders SET status = 'failed', updated_at = ? WHERE status = 'pending' AND created_at < ?")
        .bind(ts, orderCutoff),
    ]);
    console.log(
      JSON.stringify({ event: 'abandoned_orders_reclaimed', orders: stale?.n ?? 0, cutoff: orderCutoff }),
    );
  }
}
