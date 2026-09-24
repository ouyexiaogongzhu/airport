// JWT 簽名密鑰輪換 — 存 KV（CACHE），不依賴 wrangler secret put / CF API。
// 結構：jwt:current + jwt:previous；簽發只用 current；校驗先 current 再 previous（grace）。
// JWT_SECRET 僅作首次 bootstrap / KV 不可用時的兜底；輪換後的活密鑰在 KV。
//
// 不在 isolate 內長緩存材料：避免測試與多 secret 場景串擾；KV 讀成本可接受。

import { randomHex } from './csrf';

export const JWT_KV_CURRENT = 'jwt:current';
export const JWT_KV_PREVIOUS = 'jwt:previous';

/** 每日輪換一次（hourly cron 內檢查間隔，避免每小時寫 KV） */
export const JWT_ROTATION_INTERVAL_SEC = 24 * 3600;

export type JwtKey = {
  kid: string;
  secret: string;
  rotated_at: number;
};

export type JwtMaterial = {
  current: JwtKey;
  previous: JwtKey | null;
};

type JwtEnv = { CACHE: KVNamespace; JWT_SECRET?: string };

export function generateJwtSecret(): string {
  return randomHex(32);
}

export function generateJwtKid(): string {
  return randomHex(8);
}

export function verifySecrets(m: JwtMaterial): string[] {
  return m.previous ? [m.current.secret, m.previous.secret] : [m.current.secret];
}

function parseKey(raw: unknown): JwtKey | null {
  if (!raw || typeof raw !== 'object') return null;
  const o = raw as Record<string, unknown>;
  if (typeof o.kid !== 'string' || typeof o.secret !== 'string' || typeof o.rotated_at !== 'number') return null;
  if (!o.kid || !o.secret) return null;
  return { kid: o.kid, secret: o.secret, rotated_at: o.rotated_at };
}

/** 讀取失敗（KV 異常）會 throw；值不存在或損壞回傳 null —— 呼叫端必須區分兩者 */
async function readKvKey(cache: KVNamespace, key: string): Promise<JwtKey | null> {
  return parseKey(await cache.get(key, 'json'));
}

/** previous 是可選的：讀取失敗視同不存在 */
async function readKvKeyTolerant(cache: KVNamespace, key: string): Promise<JwtKey | null> {
  try {
    return await readKvKey(cache, key);
  } catch {
    return null;
  }
}

async function writeKvKey(cache: KVNamespace, key: string, value: JwtKey): Promise<void> {
  await cache.put(key, JSON.stringify(value));
}

function bootstrapKey(secret: string, now: number): JwtKey {
  return { kid: 'bootstrap', secret, rotated_at: now };
}

/**
 * 讀取簽名材料：KV current（+ previous）；若無則用 JWT_SECRET 種子寫入 current。
 * 無 KV 且無 env secret → null。
 */
export async function resolveJwtMaterial(env: JwtEnv, now = Math.floor(Date.now() / 1000)): Promise<JwtMaterial | null> {
  let current: JwtKey | null;
  try {
    current = await readKvKey(env.CACHE, JWT_KV_CURRENT);
  } catch (e) {
    // 讀取失敗時不可回落 JWT_SECRET：那會把已輪換的活密鑰覆寫回 bootstrap key，
    // 使所有現存會話失效、且洩漏過 JWT_SECRET 的一方重新獲得簽名能力。fail closed。
    console.error(JSON.stringify({ level: 'error', msg: 'jwt key kv read failed; refusing bootstrap fallback', error: String(e) }));
    return null;
  }
  if (current) {
    const previous = await readKvKeyTolerant(env.CACHE, JWT_KV_PREVIOUS);
    return { current, previous };
  }

  const bootstrap = env.JWT_SECRET?.trim();
  if (!bootstrap) return null;

  const key = bootstrapKey(bootstrap, now);
  try {
    await writeKvKey(env.CACHE, JWT_KV_CURRENT, key);
  } catch {
    // KV 寫失敗仍可用 bootstrap 簽驗（本請求）
  }
  return { current: key, previous: null };
}

/**
 * 輪換：current → previous，生成新 current。
 * 不 bump token_version（舊 token 在 previous 窗口內仍可驗）。
 */
export async function rotateJwtKeys(env: JwtEnv, now = Math.floor(Date.now() / 1000)): Promise<JwtMaterial | null> {
  const existing = await resolveJwtMaterial(env, now);
  if (!existing) return null;

  const next: JwtKey = {
    kid: generateJwtKid(),
    secret: generateJwtSecret(),
    rotated_at: now,
  };

  try {
    await writeKvKey(env.CACHE, JWT_KV_PREVIOUS, existing.current);
    await writeKvKey(env.CACHE, JWT_KV_CURRENT, next);
  } catch (e) {
    console.error(JSON.stringify({ level: 'error', msg: 'jwt key rotation kv write failed', error: String(e) }));
    return existing;
  }

  return { current: next, previous: existing.current };
}

/** hourly cron 調用：距上次輪換 ≥ JWT_ROTATION_INTERVAL_SEC 才真正輪換 */
export async function maybeRotateJwtKeys(env: JwtEnv, now = Math.floor(Date.now() / 1000)): Promise<'rotated' | 'skipped' | 'unavailable'> {
  let current: JwtKey | null;
  try {
    current = await readKvKey(env.CACHE, JWT_KV_CURRENT);
  } catch (e) {
    console.error(JSON.stringify({ level: 'error', msg: 'jwt key kv read failed; skip rotation', error: String(e) }));
    return 'unavailable';
  }
  if (!current) {
    const seeded = await resolveJwtMaterial(env, now);
    return seeded ? 'skipped' : 'unavailable';
  }
  if (now - current.rotated_at < JWT_ROTATION_INTERVAL_SEC) return 'skipped';
  const out = await rotateJwtKeys(env, now);
  return out && out.current.kid !== current.kid ? 'rotated' : 'skipped';
}
