// runEntitlementMaintenance 的 traffic_records 保留期 + traffic_daily 聚合（真實 SQLite）
import { describe, expect, it } from 'vitest';
import { createTestD1 } from '../testing/d1';
import { runEntitlementMaintenance } from './entitlement';

const NOW = 1_800_000_000;
const DAY = 86400;

function insertRecord(raw: ReturnType<typeof createTestD1>['raw'], at: number, up: number, down: number) {
  raw
    .prepare(
      'INSERT INTO traffic_records (node_id, user_id, upload_bytes, download_bytes, recorded_at) VALUES (?, ?, ?, ?, ?)',
    )
    .run(1, 2, up, down, new Date(at * 1000).toISOString());
}

describe('runEntitlementMaintenance traffic retention', () => {
  it('aggregates records older than 14d into traffic_daily, deletes them, and is idempotent', async () => {
    const { db, raw } = createTestD1();
    const oldTs = NOW - 15 * DAY;
    insertRecord(raw, oldTs, 100, 200);
    insertRecord(raw, oldTs + 3600, 50, 100);
    insertRecord(raw, NOW - 3600, 7, 9); // 近期數據，必須保留

    await runEntitlementMaintenance(db, NOW);

    const remaining = raw.prepare('SELECT recorded_at, upload_bytes, download_bytes FROM traffic_records').all();
    expect(remaining.length).toBe(1);
    expect(remaining[0].upload_bytes).toBe(7);

    const daily = raw
      .prepare('SELECT day, upload_bytes, download_bytes, records FROM traffic_daily')
      .all();
    expect(daily.length).toBe(1);
    expect(daily[0]).toEqual({
      day: new Date(oldTs * 1000).toISOString().slice(0, 10),
      upload_bytes: 150,
      download_bytes: 300,
      records: 2,
    });

    // 再跑一次：舊明細已刪，不得重複累加
    await runEntitlementMaintenance(db, NOW);
    expect(
      raw.prepare('SELECT upload_bytes, download_bytes, records FROM traffic_daily').get(),
    ).toEqual({ upload_bytes: 150, download_bytes: 300, records: 2 });
    expect(raw.prepare('SELECT COUNT(*) AS n FROM traffic_records').get()).toEqual({ n: 1 });
  });
});
