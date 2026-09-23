// Device slots on subscription pulls + list/revoke API
import { describe, expect, it } from 'vitest';
import { createApp, type Env } from '../index';
import { signJwt } from '../lib/jwt';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';

function setup(maxDevices = 2) {
  const { db, raw } = createTestD1();
  raw.exec(
    "INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', 'x', 'admin');" +
      `INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status, expire_time, max_devices) ` +
      `VALUES (2, 'alice', 'x', 'rf_alice', '11111111-2222-4333-8444-555555555555', 'active', 4102444800, ${maxDevices});` +
      "INSERT INTO nodes (name, type, address, port, protocol, status, user_id, network, security, ws_path) " +
      "VALUES ('hk', 'cf', 'hk.example.com', 443, 'vless', 'active', 1, 'ws', 'tls', '/ws');",
  );
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
  } as unknown as Env;
  const app = createApp();
  const req = (path: string, init: RequestInit = {}) => app.request(`/api/v1${path}`, init, env);
  return { raw, req, env };
}

describe('device limit on /client/links', () => {
  it('registers devices and rejects over limit with DEVICE_LIMIT_EXCEEDED', async () => {
    const { req, raw } = setup(2);

    const ok1 = await req('/client/links/rf_alice', {
      headers: { 'User-Agent': 'ClashA/1', 'X-Device-Id': 'slot----01-aaaaaaaaaaaa' },
    });
    expect(ok1.status).toBe(200);

    const ok2 = await req('/client/links/rf_alice', {
      headers: { 'User-Agent': 'ClashB/1', 'X-Device-Id': 'slot----02-bbbbbbbbbbbb' },
    });
    expect(ok2.status).toBe(200);

    const denied = await req('/client/links/rf_alice', {
      headers: { 'User-Agent': 'ClashC/1', 'X-Device-Id': 'slot----03-cccccccccccc' },
    });
    expect(denied.status).toBe(403);
    const body = (await denied.json()) as { error: string; max_devices: number };
    expect(body.error).toBe('DEVICE_LIMIT_EXCEEDED');
    expect(body.max_devices).toBe(2);

    // Known device still works
    const refresh = await req('/client/links/rf_alice', {
      headers: { 'X-Device-Id': 'slot----01-aaaaaaaaaaaa' },
    });
    expect(refresh.status).toBe(200);

    expect(raw.prepare('SELECT COUNT(*) AS n FROM user_devices WHERE user_id = 2').get()).toEqual({ n: 2 });
  });

  it('list + delete frees a slot', async () => {
    const { req } = setup(1);
    expect(
      (
        await req('/client/links/rf_alice', {
          headers: { 'X-Device-Id': 'only----01-aaaaaaaaaaaa' },
        })
      ).status,
    ).toBe(200);
    expect(
      (
        await req('/client/links/rf_alice', {
          headers: { 'X-Device-Id': 'only----02-bbbbbbbbbbbb' },
        })
      ).status,
    ).toBe(403);

    const token = await signJwt({ user_id: 2, username: 'alice', role: 'user' }, SECRET, 3600);
    const list = await req('/client/devices', {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(list.status).toBe(200);
    const listed = (await list.json()) as {
      max_devices: number;
      used: number;
      devices: { id: number }[];
    };
    expect(listed).toMatchObject({ max_devices: 1, used: 1 });
    expect(listed.devices).toHaveLength(1);

    const del = await req(`/client/devices/${listed.devices[0].id}`, {
      method: 'DELETE',
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(del.status).toBe(200);

    expect(
      (
        await req('/client/links/rf_alice', {
          headers: { 'X-Device-Id': 'only----02-bbbbbbbbbbbb' },
        })
      ).status,
    ).toBe(200);
  });

  it('JWT /client/subscription does not consume a device slot', async () => {
    const { req, raw } = setup(1);
    const token = await signJwt({ user_id: 2, username: 'alice', role: 'user' }, SECRET, 3600);
    expect((await req('/client/subscription', { headers: { Authorization: `Bearer ${token}` } })).status).toBe(
      200,
    );
    expect(raw.prepare('SELECT COUNT(*) AS n FROM user_devices WHERE user_id = 2').get()).toEqual({ n: 0 });
  });

  it('max_devices = 0 is unlimited', async () => {
    const { req } = setup(0);
    for (let i = 0; i < 6; i++) {
      const res = await req('/client/links/rf_alice', {
        headers: { 'X-Device-Id': `unlim---0${i}-aaaaaaaaaaaa` },
      });
      expect(res.status).toBe(200);
    }
  });
});
