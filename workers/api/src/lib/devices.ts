// Subscription-side device slots — see docs/devices.md.
// A "device" is a fingerprint bound on /client/links/:token pulls (not live Xray sessions).

/** Default concurrent device slots per user. Change here; products.max_devices may override on grant. */
export const DEFAULT_MAX_DEVICES = 5;

export type DeviceRow = {
  id: number;
  user_id: number;
  device_fingerprint: string;
  device_name: string | null;
  platform: string | null;
  user_agent: string | null;
  last_seen: number;
  created_at: number;
};

export type DeviceIdentity = {
  fingerprint: string;
  deviceName: string;
  platform: string;
  userAgent: string;
};

const FP_RE = /^[a-zA-Z0-9_-]{8,64}$/;

export async function sha256Hex32(input: string): Promise<string> {
  const buf = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(input));
  return Array.from(new Uint8Array(buf), (b) => b.toString(16).padStart(2, '0'))
    .join('')
    .slice(0, 32);
}

/** Prefer explicit client id; else hash User-Agent (weak — same app shares a slot). */
export async function resolveDeviceIdentity(opts: {
  headerId?: string | null;
  queryId?: string | null;
  userAgent?: string | null;
}): Promise<DeviceIdentity> {
  const explicit = (opts.headerId || opts.queryId || '').trim();
  const ua = (opts.userAgent ?? '').trim();
  const guessed = guessClient(ua);

  if (explicit && FP_RE.test(explicit)) {
    return {
      fingerprint: explicit.length === 32 ? explicit.toLowerCase() : await sha256Hex32(explicit),
      deviceName: guessed.name,
      platform: guessed.platform,
      userAgent: ua,
    };
  }

  const seed = ua || 'unknown';
  return {
    fingerprint: await sha256Hex32(`ua:${seed}`),
    deviceName: guessed.name,
    platform: guessed.platform,
    userAgent: ua,
  };
}

function guessClient(ua: string): { name: string; platform: string } {
  if (!ua) return { name: 'Unknown client', platform: 'unknown' };
  const u = ua.toLowerCase();
  if (u.includes('clash')) return { name: 'Clash', platform: 'clash' };
  if (u.includes('stash')) return { name: 'Stash', platform: 'ios' };
  if (u.includes('shadowrocket')) return { name: 'Shadowrocket', platform: 'ios' };
  if (u.includes('quantumult')) return { name: 'Quantumult', platform: 'ios' };
  if (u.includes('surge')) return { name: 'Surge', platform: 'ios' };
  if (u.includes('v2rayng') || u.includes('v2rayn')) return { name: 'v2rayN/NG', platform: 'android' };
  if (u.includes('v2raya')) return { name: 'v2rayA', platform: 'linux' };
  if (u.includes('sing-box') || u.includes('singbox')) return { name: 'sing-box', platform: 'other' };
  if (u.includes('cfnetwork') || u.includes('iphone') || u.includes('ipad')) {
    return { name: 'iOS client', platform: 'ios' };
  }
  if (u.includes('android')) return { name: 'Android client', platform: 'android' };
  return { name: ua.slice(0, 48), platform: 'other' };
}

export type TouchResult =
  | { ok: true; deviceId: number; isNew: boolean }
  | { ok: false; error: 'DEVICE_LIMIT_EXCEEDED' };

/**
 * Register or refresh a device on subscription pull.
 * maxDevices: 0 = unlimited; >0 enforces cap for new fingerprints only.
 */
export async function touchDevice(
  db: D1Database,
  userId: number,
  maxDevices: number,
  identity: DeviceIdentity,
  now: number,
): Promise<TouchResult> {
  const existing = await db
    .prepare('SELECT id FROM user_devices WHERE user_id = ? AND device_fingerprint = ? LIMIT 1')
    .bind(userId, identity.fingerprint)
    .first<{ id: number }>();

  if (existing) {
    await db
      .prepare(
        'UPDATE user_devices SET last_seen = ?, device_name = COALESCE(?, device_name), ' +
          'platform = COALESCE(?, platform), user_agent = COALESCE(?, user_agent) WHERE id = ?',
      )
      .bind(now, identity.deviceName || null, identity.platform || null, identity.userAgent || null, existing.id)
      .run();
    return { ok: true, deviceId: existing.id, isNew: false };
  }

  if (maxDevices > 0) {
    const row = await db
      .prepare('SELECT COUNT(*) AS n FROM user_devices WHERE user_id = ?')
      .bind(userId)
      .first<{ n: number }>();
    if ((row?.n ?? 0) >= maxDevices) {
      return { ok: false, error: 'DEVICE_LIMIT_EXCEEDED' };
    }
  }

  // Re-check limit inside INSERT to shrink the concurrent-insert race window.
  let insert: D1Result | null = null;
  try {
    insert = await db
      .prepare(
        'INSERT INTO user_devices (user_id, device_fingerprint, device_name, platform, user_agent, last_seen, created_at) ' +
          'SELECT ?, ?, ?, ?, ?, ?, ? WHERE ? = 0 OR (SELECT COUNT(*) FROM user_devices WHERE user_id = ?) < ?',
      )
      .bind(
        userId,
        identity.fingerprint,
        identity.deviceName,
        identity.platform,
        identity.userAgent || null,
        now,
        now,
        maxDevices,
        userId,
        maxDevices,
      )
      .run();
  } catch {
    // max_devices = 0 時 WHERE 恆真：並發同指紋首拉會撞 UNIQUE(user_id, fingerprint)
    // 拋異常而非 changes=0。視同「已存在」，走下方回查。
  }

  if (!insert || (insert.meta.changes ?? 0) === 0) {
    // Lost race, or limit hit — confirm whether fingerprint landed anyway.
    const again = await db
      .prepare('SELECT id FROM user_devices WHERE user_id = ? AND device_fingerprint = ? LIMIT 1')
      .bind(userId, identity.fingerprint)
      .first<{ id: number }>();
    if (again) return { ok: true, deviceId: again.id, isNew: false };
    return { ok: false, error: 'DEVICE_LIMIT_EXCEEDED' };
  }

  return { ok: true, deviceId: Number(insert.meta.last_row_id), isNew: true };
}

export async function listDevices(
  db: D1Database,
  userId: number,
  currentFingerprint?: string | null,
): Promise<{ max_devices: number; used: number; devices: Record<string, unknown>[] }> {
  const user = await db
    .prepare('SELECT max_devices FROM users WHERE id = ?')
    .bind(userId)
    .first<{ max_devices: number }>();
  const maxDevices = user?.max_devices ?? DEFAULT_MAX_DEVICES;

  const { results } = await db
    .prepare(
      'SELECT id, device_fingerprint, device_name, platform, user_agent, last_seen, created_at ' +
        'FROM user_devices WHERE user_id = ? ORDER BY last_seen DESC',
    )
    .bind(userId)
    .all<DeviceRow>();

  const devices = (results ?? []).map((d) => ({
    id: d.id,
    device_name: d.device_name ?? 'Unknown',
    platform: d.platform ?? 'unknown',
    last_seen: d.last_seen,
    created_at: d.created_at,
    is_current: currentFingerprint ? d.device_fingerprint === currentFingerprint : false,
  }));

  return { max_devices: maxDevices, used: devices.length, devices };
}

export async function deleteDevice(
  db: D1Database,
  userId: number,
  deviceId: number,
): Promise<'ok' | 'not_found'> {
  const r = await db
    .prepare('DELETE FROM user_devices WHERE id = ? AND user_id = ?')
    .bind(deviceId, userId)
    .run();
  return (r.meta.changes ?? 0) > 0 ? 'ok' : 'not_found';
}

export async function clearUserDevices(db: D1Database, userId: number): Promise<void> {
  await db.prepare('DELETE FROM user_devices WHERE user_id = ?').bind(userId).run();
}
