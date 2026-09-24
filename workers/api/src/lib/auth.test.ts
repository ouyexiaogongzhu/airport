// 純函數契約測試 — jwt round-trip / csrf 恆定時間比較 / cookie 屬性 / sanitizedUser
// 運行：workers/api 下 `npx vitest run src/lib/auth.test.ts`（vitest 在 repo root devDependencies）

import { describe, it, expect } from 'vitest';
import { signJwt, verifyJwt } from './jwt';
import { randomHex, constantTimeEqual } from './csrf';
import { PORTAL_SESSION_TTL, ADMIN_SESSION_TTL, REFRESH_TTL, sessionCookie, refreshCookie, csrfCookie, clearAuthCookies } from './cookies';
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

  it('accepts any matching secret in a list (rotation grace)', async () => {
    const token = await signJwt({ user_id: 1, username: 'a', role: 'user' }, SECRET, 3600);
    expect(await verifyJwt(token, ['wrong-secret-16chars', SECRET])).not.toBeNull();
    expect(await verifyJwt(token, ['wrong-a-16-chars!!', 'wrong-b-16-chars!!'])).toBeNull();
  });

  it('embeds kid in header when provided', async () => {
    const token = await signJwt({ user_id: 1, username: 'a', role: 'user' }, SECRET, 3600, 'kid-1');
    const header = JSON.parse(atob(token.split('.')[0].replace(/-/g, '+').replace(/_/g, '/')));
    expect(header.kid).toBe('kid-1');
    expect(await verifyJwt(token, SECRET)).not.toBeNull();
  });

  it('rejects token without exp claim (never-expiring token)', async () => {
    const header = btoa(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    const payload = btoa(JSON.stringify({ user_id: 1, username: 'a', role: 'user', iat: 1 }))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');
    const data = `${header}.${payload}`;
    const sig = await crypto.subtle.sign('HMAC', await crypto.subtle.importKey('raw', new TextEncoder().encode(SECRET), { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']), new TextEncoder().encode(data));
    const sigB64 = btoa(String.fromCharCode(...new Uint8Array(sig))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    expect(await verifyJwt(`${data}.${sigB64}`, SECRET)).toBeNull();
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

describe('cookies (portal 2h / admin 30d / refresh 7d)', () => {
  it('portal session cookie: 2h, HttpOnly, Secure, SameSite=None, Path=/', () => {
    const v = sessionCookie('session', 'tok', undefined);
    expect(v).toContain('session=tok');
    expect(v).toContain(`Max-Age=${PORTAL_SESSION_TTL}`);
    expect(PORTAL_SESSION_TTL).toBe(2 * 3600);
    expect(v).toContain('HttpOnly');
    expect(v).toContain('Secure');
    expect(v).toContain('SameSite=None');
    expect(v).not.toContain('SameSite=Strict');
    expect(v).toContain('Path=/');
    expect(v).not.toContain('Domain=');
  });

  it('admin session cookie keeps 30d TTL', () => {
    const v = sessionCookie('admin_session', 'tok');
    expect(v).toContain(`Max-Age=${ADMIN_SESSION_TTL}`);
    expect(ADMIN_SESSION_TTL).toBe(30 * 24 * 3600);
  });

  it('refresh cookie: 7d', () => {
    const v = refreshCookie('refresh', 'tok');
    expect(v).toContain(`Max-Age=${REFRESH_TTL}`);
    expect(REFRESH_TTL).toBe(7 * 24 * 3600);
  });

  it('csrf cookie: not HttpOnly; TTL follows portal vs admin cookie name', () => {
    const portal = csrfCookie('csrf', 'tok', 'rfplay.uk');
    expect(portal).not.toContain('HttpOnly');
    expect(portal).toContain(`Max-Age=${PORTAL_SESSION_TTL}`);
    expect(portal).toContain('Domain=rfplay.uk');
    expect(csrfCookie('admin_csrf', 'tok')).toContain(`Max-Age=${ADMIN_SESSION_TTL}`);
  });

  it('clearAuthCookies covers all 6 names (host-only; +Domain pairs when set)', () => {
    const all = clearAuthCookies();
    expect(all).toHaveLength(6);
    for (const n of ['session', 'refresh', 'csrf', 'admin_session', 'admin_refresh', 'admin_csrf']) {
      expect(all.some((v) => v.startsWith(`${n}=`))).toBe(true);
    }
    const withDomain = clearAuthCookies('rfplay.uk');
    expect(withDomain).toHaveLength(12);
    expect(withDomain.filter((v) => v.includes('Domain=rfplay.uk'))).toHaveLength(6);
    expect(withDomain.filter((v) => !v.includes('Domain='))).toHaveLength(6);
  });

  it('clearAuthCookies always pairs SameSite=None with Secure (browsers reject otherwise)', () => {
    for (const v of clearAuthCookies()) {
      expect(v).toContain('SameSite=None');
      expect(v).toContain('Secure');
    }
  });
});

describe('sanitizedUser', () => {
  it('exposes portal contract fields including profile, never credentials', () => {
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
      email: 'bob@example.com',
      phone: '+15551212',
      display_name: 'Bob',
      billing_address: '1 Main St',
    };
    const out = sanitizedUser(row);
    expect(Object.keys(out).sort()).toEqual(
      [
        'balance',
        'billing_address',
        'client_token',
        'created_at',
        'display_name',
        'email',
        'expire_time',
        'id',
        'phone',
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
    expect(out.email).toBe('bob@example.com');
    expect(out.display_name).toBe('Bob');
    expect(JSON.stringify(out)).not.toContain('password');
    expect(JSON.stringify(out)).not.toContain('vless_uuid');
    expect(JSON.stringify(out)).not.toContain('ss_password');
    expect(JSON.stringify(out)).not.toContain('trojan_password');
  });
});
