// JWT 密鑰輪換：KV current/previous、bootstrap、dual-key verify、maybeRotate 間隔

import { describe, it, expect } from 'vitest';
import { signJwt, verifyJwt } from './jwt';
import {
  JWT_KV_CURRENT,
  JWT_KV_PREVIOUS,
  JWT_ROTATION_INTERVAL_SEC,
  maybeRotateJwtKeys,
  resolveJwtMaterial,
  rotateJwtKeys,
  verifySecrets,
  type JwtKey,
  type JwtMaterial,
} from './jwtkeys';

const BOOTSTRAP = 'bootstrap-secret-at-least-16';

function memoryKv(): KVNamespace & { store: Map<string, string> } {
  const store = new Map<string, string>();
  return {
    store,
    get: (async (key: string, type?: string) => {
      const v = store.get(key);
      if (v == null) return null;
      if (type === 'json') return JSON.parse(v);
      return v;
    }) as KVNamespace['get'],
    put: (async (key: string, value: string) => {
      store.set(key, value);
    }) as KVNamespace['put'],
    delete: (async (key: string) => {
      store.delete(key);
    }) as KVNamespace['delete'],
    list: (async () => ({ keys: [], list_complete: true, cacheStatus: null })) as KVNamespace['list'],
    getWithMetadata: (async () => ({ value: null, metadata: null, cacheStatus: null })) as unknown as KVNamespace["getWithMetadata"],
  };
}

describe('jwtkeys rotation', () => {
  it('bootstraps from JWT_SECRET when KV empty', async () => {
    const CACHE = memoryKv();
    const m = await resolveJwtMaterial({ CACHE, JWT_SECRET: BOOTSTRAP }, 1_000);
    expect(m).not.toBeNull();
    expect(m!.current.secret).toBe(BOOTSTRAP);
    expect(m!.current.kid).toBe('bootstrap');
    expect(m!.previous).toBeNull();
    const stored = JSON.parse(CACHE.store.get(JWT_KV_CURRENT)!) as JwtKey;
    expect(stored.secret).toBe(BOOTSTRAP);
    expect(stored.rotated_at).toBe(1_000);
  });

  it('returns null without KV keys and without JWT_SECRET', async () => {
    expect(await resolveJwtMaterial({ CACHE: memoryKv() })).toBeNull();
  });

  it('rotate shifts current→previous and dual-key verify keeps old tokens', async () => {
    const CACHE = memoryKv();
    const env = { CACHE, JWT_SECRET: BOOTSTRAP };
    const before = (await resolveJwtMaterial(env, 1_000))!;
    const oldToken = await signJwt(
      { user_id: 1, username: 'a', role: 'user' },
      before.current.secret,
      3600,
      before.current.kid,
    );

    const after = (await rotateJwtKeys(env, 1_000 + JWT_ROTATION_INTERVAL_SEC))!;
    expect(after.current.secret).not.toBe(before.current.secret);
    expect(after.previous!.secret).toBe(before.current.secret);
    expect(after.previous!.kid).toBe(before.current.kid);

    // 舊 token：grace（previous）仍可驗
    expect(await verifyJwt(oldToken, verifySecrets(after))).not.toBeNull();
    // 僅用新密鑰則失敗
    expect(await verifyJwt(oldToken, after.current.secret)).toBeNull();

    // 新簽發只用 current
    const fresh = await signJwt(
      { user_id: 1, username: 'a', role: 'user' },
      after.current.secret,
      3600,
      after.current.kid,
    );
    expect(await verifyJwt(fresh, after.current.secret)).not.toBeNull();
    expect(JSON.parse(atob(fresh.split('.')[0].replace(/-/g, '+').replace(/_/g, '/')))).toMatchObject({
      kid: after.current.kid,
    });
  });

  it('second rotate drops the oldest key (beyond grace)', async () => {
    const CACHE = memoryKv();
    const env = { CACHE, JWT_SECRET: BOOTSTRAP };
    const t0 = (await resolveJwtMaterial(env, 1_000))!;
    const token0 = await signJwt({ user_id: 1, username: 'a', role: 'user' }, t0.current.secret, 3600);

    const t1 = (await rotateJwtKeys(env, 2_000))!;
    const token1 = await signJwt({ user_id: 1, username: 'a', role: 'user' }, t1.current.secret, 3600);

    const t2 = (await rotateJwtKeys(env, 3_000))!;

    expect(await verifyJwt(token0, verifySecrets(t2))).toBeNull();
    expect(await verifyJwt(token1, verifySecrets(t2))).not.toBeNull();
    expect(t2.previous!.secret).toBe(t1.current.secret);
  });

  it('maybeRotateJwtKeys skips until interval elapsed', async () => {
    const CACHE = memoryKv();
    const env = { CACHE, JWT_SECRET: BOOTSTRAP };
    expect(await maybeRotateJwtKeys(env, 1_000)).toBe('skipped'); // seed only
    const seeded = JSON.parse(CACHE.store.get(JWT_KV_CURRENT)!) as JwtKey;

    expect(await maybeRotateJwtKeys(env, seeded.rotated_at + JWT_ROTATION_INTERVAL_SEC - 1)).toBe('skipped');
    expect(await maybeRotateJwtKeys(env, seeded.rotated_at + JWT_ROTATION_INTERVAL_SEC)).toBe('rotated');
    const cur = JSON.parse(CACHE.store.get(JWT_KV_CURRENT)!) as JwtKey;
    expect(cur.secret).not.toBe(seeded.secret);
    expect(CACHE.store.has(JWT_KV_PREVIOUS)).toBe(true);
  });

  it('verifySecrets order is current then previous', () => {
    const m: JwtMaterial = {
      current: { kid: 'c', secret: 'cur', rotated_at: 2 },
      previous: { kid: 'p', secret: 'prev', rotated_at: 1 },
    };
    expect(verifySecrets(m)).toEqual(['cur', 'prev']);
  });
});
