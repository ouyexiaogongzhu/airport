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
      "VALUES ('hk', 'cf', 'hk.example.com', 443, 'vless', 'active', 1, 'xhttp', 'tls', '/rfhttp/');",
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

describe('device auth: bearer fallback and CSRF', () => {
  async function accessToken(ttl = 3600, extra: { tv?: number; typ?: 'refresh' } = {}) {
    return signJwt({ user_id: 2, username: 'alice', role: 'user', tv: extra.tv ?? 0, ...extra }, SECRET, ttl);
  }

  it('invalid, expired, or revoked Bearer falls back to the session cookie', async () => {
    const { req, raw } = setup();
    const session = await accessToken();
    const expired = await accessToken(-10);
    const refresh = await accessToken(3600, { typ: 'refresh' });

    const expiredOk = await req('/client/devices', {
      headers: { Authorization: `Bearer ${expired}`, Cookie: `session=${session}` },
    });
    expect(expiredOk.status).toBe(200);

    const refreshOk = await req('/client/devices', {
      headers: { Authorization: `Bearer ${refresh}`, Cookie: `session=${session}` },
    });
    expect(refreshOk.status).toBe(200);

    const stale = await accessToken();
    raw.exec('UPDATE users SET token_version = 1 WHERE id = 2');
    const fresh = await accessToken(3600, { tv: 1 });
    const revokedOk = await req('/client/devices', {
      headers: { Authorization: `Bearer ${stale}`, Cookie: `session=${fresh}` },
    });
    expect(revokedOk.status).toBe(200);

    const badFormat = await req('/client/devices', {
      headers: { Authorization: 'Token abc', Cookie: `session=${fresh}` },
    });
    expect(badFormat.status).toBe(200);
  });

  it('cookie-only GET does not require CSRF', async () => {
    const { req } = setup();
    const session = await accessToken();
    const list = await req('/client/devices', { headers: { Cookie: `session=${session}` } });
    expect(list.status).toBe(200);
    expect(await list.json()).toMatchObject({ max_devices: 2, used: 0, devices: [] });
  });

  it('returns header errors only when no session cookie can be used', async () => {
    const { req, raw } = setup();

    const missing = await req('/client/devices');
    expect(missing.status).toBe(401);
    expect(await missing.json()).toEqual({ error: 'missing authorization header' });

    const format = await req('/client/devices', { headers: { Authorization: 'Token abc' } });
    expect(format.status).toBe(401);
    expect(await format.json()).toEqual({ error: 'invalid authorization header format' });

    const emptyBearer = await req('/client/devices', { headers: { Authorization: 'Bearer' } });
    expect(emptyBearer.status).toBe(401);
    expect(await emptyBearer.json()).toEqual({ error: 'invalid authorization header format' });

    const bothBad = await req('/client/devices', {
      headers: { Authorization: 'Bearer not-a-jwt', Cookie: 'session=not-a-jwt' },
    });
    expect(bothBad.status).toBe(401);
    expect(await bothBad.json()).toEqual({ error: 'invalid or expired token' });

    const formatAndBadCookie = await req('/client/devices', {
      headers: { Authorization: 'Token abc', Cookie: 'session=not-a-jwt' },
    });
    expect(formatAndBadCookie.status).toBe(401);
    expect(await formatAndBadCookie.json()).toEqual({ error: 'invalid or expired token' });

    const session = await accessToken();
    raw.exec("UPDATE users SET status = 'banned' WHERE id = 2");
    const disabled = await req('/client/devices', {
      headers: { Authorization: 'Bearer not-a-jwt', Cookie: `session=${session}` },
    });
    expect(disabled.status).toBe(403);
    expect(await disabled.json()).toEqual({ error: 'ACCOUNT_DISABLED' });
  });

  it('DELETE requires CSRF unless the accepted credential is the bearer token', async () => {
    const { req } = setup(1);
    expect(
      (await req('/client/links/rf_alice', { headers: { 'X-Device-Id': 'csrf----01-aaaaaaaaaaaa' } })).status,
    ).toBe(200);
    const session = await accessToken();
    const bearer = await accessToken();
    const listed = (await (
      await req('/client/devices', { headers: { Cookie: `session=${session}` } })
    ).json()) as { devices: { id: number }[] };
    const id = listed.devices[0].id;

    const cookieOnly = await req(`/client/devices/${id}`, {
      method: 'DELETE',
      headers: { Cookie: `session=${session}` },
    });
    expect(cookieOnly.status).toBe(403);
    expect(await cookieOnly.json()).toEqual({ error: 'CSRF_INVALID' });

    const dummyAuth = await req(`/client/devices/${id}`, {
      method: 'DELETE',
      headers: { Authorization: 'Bearer not-a-jwt', Cookie: `session=${session}` },
    });
    expect(dummyAuth.status).toBe(403);
    expect(await dummyAuth.json()).toEqual({ error: 'CSRF_INVALID' });

    const mismatch = await req(`/client/devices/${id}`, {
      method: 'DELETE',
      headers: {
        Authorization: 'Bearer not-a-jwt',
        Cookie: `session=${session}; csrf=csrf-cookie`,
        'X-CSRF-Token': 'wrong',
      },
    });
    expect(mismatch.status).toBe(403);

    const withCsrf = await req(`/client/devices/${id}`, {
      method: 'DELETE',
      headers: {
        Authorization: 'Bearer not-a-jwt',
        Cookie: `session=${session}; csrf=csrf-cookie`,
        'X-CSRF-Token': 'csrf-cookie',
      },
    });
    expect(withCsrf.status).toBe(200);
    expect(await withCsrf.json()).toEqual({ ok: true });

    expect(
      (await req('/client/links/rf_alice', { headers: { 'X-Device-Id': 'csrf----02-bbbbbbbbbbbb' } })).status,
    ).toBe(200);
    const again = (await (
      await req('/client/devices', { headers: { Authorization: `Bearer ${bearer}` } })
    ).json()) as { devices: { id: number }[] };
    const bearerDelete = await req(`/client/devices/${again.devices[0].id}`, {
      method: 'DELETE',
      headers: {
        Authorization: `Bearer ${bearer}`,
        Cookie: 'csrf=csrf-cookie',
        'X-CSRF-Token': 'wrong',
      },
    });
    expect(bearerDelete.status).toBe(200);
  });
});
