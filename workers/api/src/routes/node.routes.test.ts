// daemon 節點面：簽名、配置內容（只含可服務用戶）、流量上報記賬
import { describe, expect, it } from 'vitest';
import { createApp, type Env } from '../index';
import { signJwt } from '../lib/jwt';
import { nodeSignature } from '../lib/nodehmac';
import { createTestD1 } from '../testing/d1';

const TOKEN = 'nd_test';
const SECRET = 'test-secret';

type Inbound = {
  tag: string;
  listen: string;
  port: number;
  protocol: string;
  settings: { clients?: { id: string; email: string }[]; address?: string };
  streamSettings?: Record<string, unknown>;
};
type XrayConfig = {
  inbounds: Inbound[];
  api: { services: string[] };
  routing: { rules: Record<string, unknown>[] };
  policy: { levels: Record<string, Record<string, unknown>> };
  _meta: { node_id: number; user_ids: number[]; version: number; api_port: number };
};

function setup() {
  const { db, raw } = createTestD1();
  raw.exec(
    "INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'x', 'admin');" +
      // 2 可服務；3 過期；4 封禁；5 超額；6 pending；7 可服務但缺 UUID
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid, traffic_used_bytes) VALUES (2, 'alice', 'x', 'active', 'uuid-2', 10);" +
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid, expire_time) VALUES (3, 'bob', 'x', 'active', 'uuid-3', 1);" +
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid, status) VALUES (4, 'carol', 'x', 'active', 'uuid-4', 'banned');" +
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid, traffic_limit_bytes, traffic_used_bytes) VALUES (5, 'dave', 'x', 'active', 'uuid-5', 100, 100);" +
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid) VALUES (6, 'eve', 'x', 'pending', 'uuid-6');" +
      "INSERT INTO users (id, username, password_hash, subscription_status, vless_uuid) VALUES (7, 'frank', 'x', 'active', NULL);" +
      "INSERT INTO nodes (id, name, type, address, port, protocol, status, user_id, network, security, ws_path, token) VALUES (9, 'hk', 'xray', 'node-hk.example.com', 20001, 'vless', 'active', 1, 'tcp', 'reality', '/ws', 'nd_test');",
  );
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
  } as unknown as Env;
  const app = createApp();

  const signed = async (method: string, path: string, body = '', opts: { ts?: number; token?: string; sig?: string } = {}) => {
    const ts = String(opts.ts ?? Math.floor(Date.now() / 1000));
    const sig = opts.sig ?? (await nodeSignature(opts.token ?? TOKEN, method, path, ts, body));
    return app.request(
      path,
      {
        method,
        headers: { 'X-Node-Timestamp': ts, 'X-Node-Signature': sig, 'Content-Type': 'application/json' },
        body: method === 'GET' ? undefined : body,
      },
      env,
    );
  };
  const getConfig = () => signed('GET', `/api/v1/node/${TOKEN}/config`);
  const report = (payload: unknown) => signed('POST', `/api/v1/node/${TOKEN}/traffic/report`, JSON.stringify(payload));
  return { raw, app, env, signed, getConfig, report };
}

describe('nodeSignature', () => {
  it('與 daemon sync.go signRequest 算法一致（獨立計算的向量）', async () => {
    expect(await nodeSignature(TOKEN, 'GET', '/api/v1/node/nd_test/config', '1700000000', '')).toBe(
      'db241703223b7520909120404c8cb4609e771a7e62a4e3dcd4e6b95bcb000635',
    );
  });
});

describe('GET /node/:token/config', () => {
  it('簽名正確 → {node_id,name,protocol,config}，Tunnel 形態', async () => {
    const { getConfig, raw } = setup();
    const res = await getConfig();
    expect(res.status).toBe(200);
    const body = (await res.json()) as { node_id: number; name: string; protocol: string; config: XrayConfig };
    expect(body).toMatchObject({ node_id: 9, name: 'hk', protocol: 'vless' });

    const cfg = body.config;
    const inbound = cfg.inbounds.find((i) => i.tag === 'in-vless')!;
    expect(inbound).toMatchObject({ listen: '127.0.0.1', port: 20001, protocol: 'vless' });
    // 舊列 network=tcp / security=reality 被忽略
    expect(inbound.streamSettings).toEqual({ network: 'ws', security: 'none', wsSettings: { path: '/ws' } });
    expect(JSON.stringify(cfg)).not.toMatch(/reality|flow|tlsSettings|certificate/);

    // 只含可服務用戶，email = u{id}
    expect(inbound.settings.clients).toEqual([{ id: 'uuid-2', email: 'u2', level: 0 }]);
    expect(cfg._meta).toMatchObject({ node_id: 9, user_ids: [2], api_port: 10085 });

    // StatsService：api 入站只監聽本機 + 路由 + 按用戶統計
    expect(cfg.api.services).toContain('StatsService');
    const api = cfg.inbounds.find((i) => i.tag === 'api')!;
    expect(api).toMatchObject({ listen: '127.0.0.1', port: 10085, protocol: 'dokodemo-door' });
    expect(cfg.routing.rules[0]).toEqual({ type: 'field', inboundTag: ['api'], outboundTag: 'api' });
    expect(cfg.routing.rules[1]).toMatchObject({ outboundTag: 'block' });
    expect((cfg.routing.rules[1].ip as string[])).toContain('127.0.0.0/8');
    expect(cfg.policy.levels['0']).toMatchObject({ statsUserUplink: true, statsUserDownlink: true });

    const hb = raw.prepare('SELECT last_heartbeat h FROM nodes WHERE id = 9').get()!.h;
    expect(typeof hb).toBe('string');
  });

  it('版本號隨用戶集合與節點傳輸配置變化，否則穩定', async () => {
    const { getConfig, raw } = setup();
    const version = async () => ((await (await getConfig()).json()) as { config: XrayConfig }).config._meta.version;
    const v1 = await version();
    expect(Number.isSafeInteger(v1)).toBe(true);
    expect(await version()).toBe(v1);

    raw.exec("UPDATE nodes SET ws_path = '/other' WHERE id = 9");
    const v2 = await version();
    expect(v2).not.toBe(v1);

    raw.exec('UPDATE nodes SET port = 20002 WHERE id = 9');
    const v3 = await version();
    expect(v3).not.toBe(v2);

    raw.exec("UPDATE users SET status = 'banned' WHERE id = 2");
    const v4 = await version();
    expect(v4).not.toBe(v3);

    raw.exec("UPDATE users SET status = 'active', vless_uuid = 'uuid-2b' WHERE id = 2");
    expect(await version()).not.toBe(v3);
  });

  it('節點非 active：下發空用戶列表（daemon 應用後斷開所有連接）', async () => {
    const { getConfig, raw } = setup();
    raw.exec("UPDATE nodes SET status = 'inactive' WHERE id = 9");
    const res = await getConfig();
    expect(res.status).toBe(200);
    const cfg = ((await res.json()) as { config: XrayConfig }).config;
    expect(cfg.inbounds[0].settings.clients).toEqual([]);
  });

  it('vmess 節點不帶 decryption', async () => {
    const { getConfig, raw } = setup();
    raw.exec("UPDATE nodes SET protocol = 'vmess' WHERE id = 9");
    const cfg = ((await (await getConfig()).json()) as { config: XrayConfig }).config;
    expect(cfg.inbounds[0].settings).toEqual({ clients: [{ id: 'uuid-2', email: 'u2', level: 0 }] });
  });

  it('節點端口與 StatsService 端口衝突時換端口', async () => {
    const { getConfig, raw } = setup();
    raw.exec('UPDATE nodes SET port = 10085 WHERE id = 9');
    const cfg = ((await (await getConfig()).json()) as { config: XrayConfig }).config;
    expect(cfg._meta.api_port).toBe(10086);
    expect(cfg.inbounds.find((i) => i.tag === 'api')!.port).toBe(10086);
  });

  it.each([
    ['簽名錯誤', { sig: '0'.repeat(64) }],
    ['用別的 token 簽名', { token: 'nd_other' }],
    ['時間戳過期', { ts: Math.floor(Date.now() / 1000) - 3600 }],
  ])('%s → 401', async (_name, opts) => {
    const { signed } = setup();
    expect((await signed('GET', `/api/v1/node/${TOKEN}/config`, '', opts)).status).toBe(401);
  });

  it('缺簽名頭 / 未知 token → 401', async () => {
    const { app, env, signed } = setup();
    expect((await app.request(`/api/v1/node/${TOKEN}/config`, {}, env)).status).toBe(401);
    expect((await signed('GET', '/api/v1/node/nd_unknown/config', '', { token: 'nd_unknown' })).status).toBe(401);
  });
});

describe('POST /node/:token/traffic/report', () => {
  it('批量記賬：traffic_records、用戶已用流量、節點計數與心跳', async () => {
    const { report, raw } = setup();
    const res = await report({
      node_id: 9,
      traffic: [
        { user_id: 2, upload_bytes: 100, download_bytes: 1000 },
        { user_id: 3, upload_bytes: 5, download_bytes: 0 },
        { user_id: 2, upload_bytes: 1, download_bytes: 2 }, // 同一用戶合併
        { user_id: 999, upload_bytes: 7, download_bytes: 7 }, // 不存在的用戶不記錄
        { user_id: 4, upload_bytes: 0, download_bytes: 0 }, // 零流量跳過
      ],
    });
    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ ok: true, accepted: 2 });

    const rows = raw
      .prepare('SELECT node_id n, user_id u, upload_bytes up, download_bytes down FROM traffic_records ORDER BY user_id')
      .all();
    expect(rows).toEqual([
      { n: 9, u: 2, up: 101, down: 1002 },
      { n: 9, u: 3, up: 5, down: 0 },
    ]);
    const used = raw.prepare('SELECT id, traffic_used_bytes t FROM users WHERE id IN (2, 3, 4) ORDER BY id').all();
    expect(used).toEqual([
      { id: 2, t: 10 + 1103 },
      { id: 3, t: 5 },
      { id: 4, t: 0 },
    ]);
    const node = raw.prepare('SELECT traffic_up up, traffic_down down, last_heartbeat h FROM nodes WHERE id = 9').get()!;
    expect(node.up).toBe(101 + 5 + 7);
    expect(node.down).toBe(1002 + 7);
    expect(typeof node.h).toBe('string');
  });

  it('超額後下一次拉配置即移除該用戶', async () => {
    const { report, getConfig, raw } = setup();
    raw.exec('UPDATE users SET traffic_limit_bytes = 1000 WHERE id = 2');
    expect((await report({ traffic: [{ user_id: 2, upload_bytes: 0, download_bytes: 990 }] })).status).toBe(200);
    const cfg = ((await (await getConfig()).json()) as { config: XrayConfig }).config;
    expect(cfg._meta.user_ids).toEqual([]);
  });

  it('空批次只記心跳', async () => {
    const { report, raw } = setup();
    const res = await report({ node_id: 9, traffic: [] });
    expect(res.status).toBe(200);
    expect(raw.prepare('SELECT COUNT(*) n FROM traffic_records').get()!.n).toBe(0);
    expect(typeof raw.prepare('SELECT last_heartbeat h FROM nodes WHERE id = 9').get()!.h).toBe('string');
  });

  it('簽名覆蓋 body：篡改 body → 401', async () => {
    const { app, env } = setup();
    const path = `/api/v1/node/${TOKEN}/traffic/report`;
    const ts = String(Math.floor(Date.now() / 1000));
    const sig = await nodeSignature(TOKEN, 'POST', path, ts, JSON.stringify({ traffic: [] }));
    const res = await app.request(
      path,
      {
        method: 'POST',
        headers: { 'X-Node-Timestamp': ts, 'X-Node-Signature': sig },
        body: JSON.stringify({ traffic: [{ user_id: 2, upload_bytes: 1, download_bytes: 1 }] }),
      },
      env,
    );
    expect(res.status).toBe(401);
  });

  it.each([
    ['node_id 不匹配', { node_id: 8, traffic: [] }],
    ['traffic 不是數組', { traffic: {} }],
    ['負數流量', { traffic: [{ user_id: 2, upload_bytes: -1, download_bytes: 0 }] }],
    ['缺 user_id', { traffic: [{ upload_bytes: 1, download_bytes: 0 }] }],
  ])('%s → 400', async (_name, payload) => {
    const { report, raw } = setup();
    expect((await report(payload)).status).toBe(400);
    expect(raw.prepare('SELECT traffic_used_bytes t FROM users WHERE id = 2').get()!.t).toBe(10);
  });
});

describe('admin 節點編輯（Tunnel 形態）', () => {
  it('只接受 ws_path；network/security/reality 欄位被忽略且不輸出', async () => {
    const { app, env, raw } = setup();
    const token = await signJwt({ user_id: 1, username: 'admin', role: 'admin' }, SECRET, 3600);
    const req = (method: string, path: string, body: unknown) =>
      app.request(
        `/api/v1${path}`,
        { method, headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' }, body: JSON.stringify(body) },
        env,
      );
    const created = await req('POST', '/admin/nodes', {
      name: 'sg', type: 'xray', address: 'node-sg.example.com', port: 20010, protocol: 'vmess',
      ws_path: '/sg', security: 'reality', network: 'tcp', reality_public_key: 'PBK',
    });
    expect(created.status).toBe(201);
    const node = (await created.json()) as Record<string, unknown>;
    expect(node).toMatchObject({ ws_path: '/sg', port: 20010 });
    expect(node).not.toHaveProperty('security');
    expect(node).not.toHaveProperty('reality_public_key');
    expect(raw.prepare('SELECT network, security, reality_public_key r FROM nodes WHERE id = ?').get(node.id as number)).toEqual({
      network: 'ws', security: 'none', r: null,
    });
    expect((await req('PUT', `/admin/nodes/${node.id}`, { ws_path: 'no-slash' })).status).toBe(400);
  });
});
