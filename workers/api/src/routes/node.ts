// daemon 節點面 — 對齊 daemon internal/sync/sync.go fetchConfig / reportTraffic。掛載點：/api/v1。
// 按 nodes.token 定位節點，校驗 X-Node-Timestamp / X-Node-Signature（見 lib/nodehmac.ts）。
import { Hono } from 'hono';
import type { Context } from 'hono';
import type { Env } from '../index';
import { buildNodeXrayConfig, type NodeConfigRow } from '../lib/nodeconfig';
import { verifyNodeSignature } from '../lib/nodehmac';

type NodeRow = NodeConfigRow & { name: string };

// 單次上報的用戶條目上限
const MAX_ENTRIES = 5000;

// 驗簽失敗統一回 401，不區分 token 不存在與簽名錯誤
async function authNode(c: Context<{ Bindings: Env }>, body: string): Promise<NodeRow | null> {
  const token = c.req.param('token') ?? '';
  if (token === '') return null;
  const node = await c.env.DB.prepare('SELECT id, name, port, protocol, ws_path, network, status FROM nodes WHERE token = ?')
    .bind(token)
    .first<NodeRow>();
  if (!node) return null;
  const ok = await verifyNodeSignature(
    token,
    c.req.method,
    new URL(c.req.url).pathname,
    c.req.header('X-Node-Timestamp'),
    c.req.header('X-Node-Signature'),
    body,
    Math.floor(Date.now() / 1000),
  );
  return ok ? node : null;
}

function isNonNegInt(v: unknown): v is number {
  return typeof v === 'number' && Number.isSafeInteger(v) && v >= 0;
}

export function nodeRoutes() {
  const app = new Hono<{ Bindings: Env }>();

  // 拉配置：{node_id,name,protocol,config}；同時記心跳（不動 updated_at）
  app.get('/node/:token/config', async (c) => {
    const node = await authNode(c, '');
    if (!node) return c.json({ error: 'INVALID_NODE_SIGNATURE' }, 401);
    const now = Math.floor(Date.now() / 1000);
    const config = await buildNodeXrayConfig(c.env.DB, node, now);
    await c.env.DB.prepare('UPDATE nodes SET last_heartbeat = ? WHERE id = ?')
      .bind(new Date(now * 1000).toISOString(), node.id)
      .run();
    return c.json({ node_id: node.id, name: node.name, protocol: node.protocol, config });
  });

  // 批量上報 {node_id?, traffic:[{user_id,upload_bytes,download_bytes}]}：
  // 同一 batch 內寫 traffic_records、累加用戶已用流量與節點計數、記心跳。
  // 條目經 json_each 展開，語句數與用戶數無關（D1 單次調用有查詢數上限）。
  app.post('/node/:token/traffic/report', async (c) => {
    const raw = await c.req.text();
    const node = await authNode(c, raw);
    if (!node) return c.json({ error: 'INVALID_NODE_SIGNATURE' }, 401);

    let body: { node_id?: unknown; traffic?: unknown };
    try {
      body = JSON.parse(raw);
    } catch {
      return c.json({ error: 'invalid request body' }, 400);
    }
    if (body === null || typeof body !== 'object' || !Array.isArray(body.traffic)) {
      return c.json({ error: 'traffic must be an array' }, 400);
    }
    if (body.node_id !== undefined && body.node_id !== 0 && body.node_id !== node.id) {
      return c.json({ error: 'node_id does not match token' }, 400);
    }
    if (body.traffic.length > MAX_ENTRIES) return c.json({ error: 'too many entries' }, 413);

    // 同一用戶多條合併
    const byUser = new Map<number, { up: number; down: number }>();
    for (const e of body.traffic as Record<string, unknown>[]) {
      const uid = e?.user_id;
      const up = e?.upload_bytes ?? 0;
      const down = e?.download_bytes ?? 0;
      if (!isNonNegInt(uid) || uid === 0 || !isNonNegInt(up) || !isNonNegInt(down)) {
        return c.json({ error: 'invalid traffic entry' }, 400);
      }
      const cur = byUser.get(uid) ?? { up: 0, down: 0 };
      cur.up += up;
      cur.down += down;
      byUser.set(uid, cur);
    }
    const entries = [...byUser].filter(([, t]) => t.up > 0 || t.down > 0).map(([u, t]) => ({ u, up: t.up, down: t.down }));
    const json = JSON.stringify(entries);
    const recordedAt = new Date().toISOString();
    const db = c.env.DB;

    const stmts: D1PreparedStatement[] = [];
    if (entries.length > 0) {
      stmts.push(
        db
          .prepare(
            'INSERT INTO traffic_records (node_id, user_id, upload_bytes, download_bytes, recorded_at) ' +
              "SELECT ?, json_extract(value, '$.u'), json_extract(value, '$.up'), json_extract(value, '$.down'), ? " +
              "FROM json_each(?) WHERE json_extract(value, '$.u') IN (SELECT id FROM users)",
          )
          .bind(node.id, recordedAt, json),
        db
          .prepare(
            'UPDATE users SET traffic_used_bytes = COALESCE(traffic_used_bytes, 0) + ' +
              "(SELECT json_extract(value, '$.up') + json_extract(value, '$.down') FROM json_each(?) WHERE json_extract(value, '$.u') = users.id) " +
              "WHERE id IN (SELECT json_extract(value, '$.u') FROM json_each(?))",
          )
          .bind(json, json),
      );
    }
    stmts.push(
      db
        .prepare(
          'UPDATE nodes SET last_heartbeat = ?, ' +
            "traffic_up = COALESCE(traffic_up, 0) + (SELECT COALESCE(SUM(json_extract(value, '$.up')), 0) FROM json_each(?)), " +
            "traffic_down = COALESCE(traffic_down, 0) + (SELECT COALESCE(SUM(json_extract(value, '$.down')), 0) FROM json_each(?)) " +
            'WHERE id = ?',
        )
        .bind(recordedAt, json, json, node.id),
    );
    const results = await db.batch(stmts);
    const accepted = entries.length > 0 ? (results[0].meta.changes ?? 0) : 0;
    return c.json({ ok: true, accepted });
  });

  return app;
}
