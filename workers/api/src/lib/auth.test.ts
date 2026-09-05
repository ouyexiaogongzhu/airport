// 純函數契約測試 — jwt round-trip / csrf 恆定時間比較 / cookie 屬性 / sanitizedUser
// 運行：workers/api 下 `npx vitest run src/lib/auth.test.ts`（vitest 在 repo root devDependencies）

import { describe, it, expect } from 'vitest';
import { signJwt, verifyJwt } from './jwt';
import { randomHex, constantTimeEqual } from './csrf';
import { SESSION_TTL, REFRESH_TTL, sessionCookie, refreshCookie, csrfCookie, clearAuthCookies } from './cookies';
import { sanitizedUser, type UserRow } from './user';

const SECRET = 'test-secret-at-least-16-chars';

describe('jwt round-trip', () => {
  it('sign then verify returns original claims', async () => {
    const claims = { user_id: 42, username: 'alice', role: 'user' };
    const token = await signJwt(claims, SECRET, 3600);
    const out = await verifyJwt(token, SECRET);
    expect(out).not.toBeNull();
    expect(out!.user_id).toBe(42);
    expect(out!.username).toBe('alice');
    expect(out!.role).toBe('user');
    expect(out!.exp).toBeGreaterThan(out!.iat);
  });

  it('rejects wrong secret', async () => {
    const token = await signJwt({ user_id: 1, username: 'a', role: 'user' }, SECRET, 3600);
    expect(await verifyJwt(token, 'another-secret-value-16ch')).toBeNull();
  });

  it('rejects expired token', async () => {
    const token = await signJwt({ user_id: 1, username: 'a', role: 'user' }, SECRET, -10);
    expect(await verifyJwt(token, SECRET)).toBeNull();
  });

  it('rejects tampered payload', async () => {
    const token = await signJwt({ user_id: 1, username: 'a', role: 'user' }, SECRET, 3600);
    const parts = token.split('.');
    const forged = JSON.stringify({ user_id: 999, username: 'a', role: 'admin', exp: 9999999999, iat: 1 });
    const payload = btoa(forged).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    expect(await verifyJwt(`${parts[0]}.${payload}.${parts[2]}`, SECRET)).toBeNull();
  });

  it('rejects malformed token', async () => {
    expect(await verifyJwt('not-a-jwt', SECRET)).toBeNull();
  });
});

describe('csrf', () => {
  it('randomHex(32) is 64 lowercase hex chars and unique', () => {
    const a = randomHex(32);
    expect(a).toMatch(/^[0-9a-f]{64}$/);
    expect(randomHex(32)).not.toBe(a);
  });

  it('constantTimeEqual matches only exact equality', () => {
    expect(constantTimeEqual('abc', 'abc')).toBe(true);
    expect(constantTimeEqual('abc', 'abd')).toBe(false);
    expect(constantTimeEqual('abc', 'abcd')).toBe(false);
    expect(constantTimeEqual('', '')).toBe(true);
  });
});

describe('cookies (對齊 auth.go)', () => {
  it('session cookie: 30d, HttpOnly, Secure, SameSite=Strict, Path=/', () => {
    const v = sessionCookie('session', 'tok', undefined);
    expect(v).toContain('session=tok');
    expect(v).toContain(`Max-Age=${SESSION_TTL}`);
    expect(SESSION_TTL).toBe(30 * 24 * 3600);
    expect(v).toContain('HttpOnly');
    expect(v).toContain('Secure');
    expect(v).toContain('SameSite=Strict');
    expect(v).toContain('Path=/');
    expect(v).not.toContain('Domain=');
  });

  it('refresh cookie: 90d', () => {
    const v = refreshCookie('refresh', 'tok');
    expect(v).toContain(`Max-Age=${REFRESH_TTL}`);
    expect(REFRESH_TTL).toBe(90 * 24 * 3600);
  });

  it('csrf cookie: not HttpOnly, session TTL, Domain appended when set', () => {
    const v = csrfCookie('csrf', 'tok', 'rfplay.uk');
    expect(v).not.toContain('HttpOnly');
    expect(v).toContain(`Max-Age=${SESSION_TTL}`);
    expect(v).toContain('Domain=rfplay.uk');
  });

  it('clearAuthCookies covers all 6 names', () => {
    const all = clearAuthCookies();
    expect(all).toHaveLength(6);
    for (const n of ['session', 'refresh', 'csrf', 'admin_session', 'admin_refresh', 'admin_csrf']) {
      expect(all.some((v) => v.startsWith(`${n}=`))).toBe(true);
    }
  });
});

describe('sanitizedUser', () => {
  it('exposes exactly the portal contract fields, never credentials', () => {
    const row: UserRow = {
      id: 7,
      username: 'bob',
      role: 'user',
      status: 'active',
      balance: 1.5,
      subscription_status: 'pending',
      subscription_tier: null,
      traffic_limit_bytes: 0,
      traffic_used_bytes: 0,
      expire_time: 0,
      rate_limit_bps: 0,
      traffic_period_start: 0,
      client_token: 'rf_x',
      created_at: '2026-01-01T00:00:00.000Z',
    };
    const out = sanitizedUser(row);
    expect(Object.keys(out).sort()).toEqual(
      [
        'balance',
        'client_token',
        'created_at',
        'expire_time',
        'id',
        'rate_limit_bps',
        'role',
        'status',
        'subscription_status',
        'subscription_tier',
        'traffic_limit_bytes',
        'traffic_period_start',
        'traffic_used_bytes',
        'username',
      ].sort(),
    );
    expect(JSON.stringify(out)).not.toContain('password');
    expect(JSON.stringify(out)).not.toContain('vless_uuid');
    expect(JSON.stringify(out)).not.toContain('ss_password');
    expect(JSON.stringify(out)).not.toContain('trojan_password');
  });
});
