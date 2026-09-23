// 會話吊銷（token_version）、封禁、降權、refresh、退出 — 真實 SQLite 上跑全部路由
import { describe, expect, it } from 'vitest';
import bcrypt from 'bcryptjs';
import { createApp, type Env } from '../index';
import { signJwt } from '../lib/jwt';
import { signTokens } from '../lib/session';
import { createTestD1 } from '../testing/d1';

const SECRET = 'test-secret';
const PASSWORD = 'password123';

function setup() {
  const { db, raw } = createTestD1();
  const hash = bcrypt.hashSync(PASSWORD, 4);
  raw.prepare("INSERT INTO users (id, username, password_hash, role) VALUES (1, 'admin', ?, 'admin')").run(hash);
  raw
    .prepare(
      "INSERT INTO users (id, username, password_hash, client_token, vless_uuid, subscription_status, expire_time) " +
        "VALUES (2, 'alice', ?, 'rf_alice', '11111111-2222-4333-8444-555555555555', 'active', 4102444800)",
    )
    .run(hash);
  const env = {
    DB: db,
    CACHE: { get: async () => null, put: async () => {} } as unknown as KVNamespace,
    JWT_SECRET: SECRET,
    TURNSTILE_DISABLED: '1',
  } as unknown as Env;
  const app = createApp();
  const req = (path: string, init: RequestInit = {}) => app.request(`/api/v1${path}`, init, env);
  const bearer = (token: string, extra: Record<string, string> = {}) => ({
    headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', ...extra },
  });
  const tokens = (id: number) => {
    const u = raw.prepare('SELECT id, username, role, token_version FROM users WHERE id = ?').get(id) as {
      id: number;
      username: string;
      role: string;
      token_version: number;
    };
    return signTokens(u, SECRET);
  };
  return { raw, req, bearer, tokens };
}

function setCookies(res: Response): string[] {
  return res.headers.getSetCookie();
}

describe('token_version 吊銷', () => {
  it('版本號不符的 token 在 portal / admin / client 三處都被拒', async () => {
    const { raw, req, bearer, tokens } = setup();
    const a = await tokens(1);
    const u = await tokens(2);
    expect((await req('/auth/validate', bearer(u.bearer))).status).toBe(200);
    expect((await req('/admin/users', bearer(a.bearer))).status).toBe(200);
    expect((await req('/client/subscription', bearer(u.bearer))).status).toBe(200);

    raw.exec('UPDATE users SET token_version = token_version + 1');
    expect((await req('/auth/validate', bearer(u.bearer))).status).toBe(401);
    expect((await req('/user/profile', bearer(u.bearer))).status).toBe(401);
    expect((await req('/admin/users', bearer(a.bearer))).status).toBe(401);
    expect((await req('/admin/auth/validate', bearer(a.bearer))).status).toBe(401);
    const sub = await req('/client/subscription', bearer(u.bearer));
    expect(sub.status).toBe(401);
    expect(await sub.json()).toEqual({ error: 'invalid or expired token' });
  });

  it('不帶 tv 的舊 token 視為版本 0', async () => {
    const { raw, req, bearer } = setup();
    const legacy = await signJwt({ user_id: 2, username: 'alice', role: 'user' }, SECRET, 3600);
    expect((await req('/auth/validate', bearer(legacy))).status).toBe(200);
    raw.exec('UPDATE users SET token_version = 1 WHERE id = 2');
    expect((await req('/auth/validate', bearer(legacy))).status).toBe(401);
  });

  it('refresh token 不能當 access token 用', async () => {
    const { req, bearer, tokens } = setup();
    const u = await tokens(2);
    expect((await req('/auth/validate', bearer(u.refresh))).status).toBe(401);
    expect((await req('/client/subscription', bearer(u.refresh))).status).toBe(401);
  });
});

describe('封禁與降權', () => {
  it('banned 用戶即使版本號相同也被拒', async () => {
    const { raw, req, bearer, tokens } = setup();
    const u = await tokens(2);
    raw.exec("UPDATE users SET status = 'banned' WHERE id = 2");
    expect((await req('/auth/validate', bearer(u.bearer))).status).toBe(401);
    const sub = await req('/client/subscription', bearer(u.bearer));
    expect(sub.status).toBe(403);
    expect(await sub.json()).toEqual({ error: 'ACCOUNT_DISABLED' });
    const r = await req('/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: u.refresh }) });
    expect(r.status).toBe(401);
  });

  it('後台封禁會吊銷會話：解封後舊 token 仍失效', async () => {
    const { req, bearer, tokens } = setup();
    const a = await tokens(1);
    const u = await tokens(2);
    const put = (status: string) =>
      req('/admin/users/2', { method: 'PUT', body: JSON.stringify({ status }), ...bearer(a.bearer) });
    expect((await put('banned')).status).toBe(200);
    expect((await put('active')).status).toBe(200);
    expect((await req('/auth/validate', bearer(u.bearer))).status).toBe(401);
    expect((await req('/client/subscription', bearer(u.bearer))).status).toBe(401);
    // 改回 active 不加版本：重新簽發的 token 可用
    expect((await req('/auth/validate', bearer((await tokens(2)).bearer))).status).toBe(200);
  });

  it('管理員被降權後下一次請求即失去後台權限', async () => {
    const { raw, req, bearer, tokens } = setup();
    const a = await tokens(1);
    expect((await req('/admin/users', bearer(a.bearer))).status).toBe(200);
    raw.exec("UPDATE users SET role = 'user' WHERE id = 1");
    expect((await req('/admin/users', bearer(a.bearer))).status).toBe(403);
    expect((await req('/admin/auth/validate', bearer(a.bearer))).status).toBe(403);
    const r = await req('/admin/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: a.refresh }) });
    expect(r.status).toBe(403);
  });

  it('token 裡的 role 不被信任：user 自簽 role=admin 的 token 仍是 user', async () => {
    const { req, bearer } = setup();
    const forged = await signJwt({ user_id: 2, username: 'alice', role: 'admin', tv: 0 }, SECRET, 3600);
    expect((await req('/admin/users', bearer(forged))).status).toBe(403);
  });
});

describe('/auth/refresh', () => {
  it('跨站：只帶 body.refresh_token（Bearer 已過期）即可續期', async () => {
    const { req, bearer } = setup();
    const login = await req('/public/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'alice', password: PASSWORD }),
    });
    expect(login.status).toBe(200);
    const { refresh_token } = (await login.json()) as { refresh_token: string };
    const expired = await signJwt({ user_id: 2, username: 'alice', role: 'user', tv: 0 }, SECRET, -10);

    const r = await req('/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token }), ...bearer(expired) });
    expect(r.status).toBe(200);
    const { token } = (await r.json()) as { token: string };
    expect((await req('/auth/validate', bearer(token))).status).toBe(200);
  });

  it('同站：只帶 refresh cookie 即可續期並重簽 session cookie', async () => {
    const { req, tokens } = setup();
    const u = await tokens(2);
    const r = await req('/auth/refresh', { method: 'POST', headers: { Cookie: `refresh=${u.refresh}` } });
    expect(r.status).toBe(200);
    const session = setCookies(r).find((v) => v.startsWith('session='))!;
    const value = session.split(';')[0].slice('session='.length);
    expect((await req('/auth/validate', { headers: { Cookie: `session=${value}` } })).status).toBe(200);
  });

  it('access token 不能用來續期', async () => {
    const { req, tokens } = setup();
    const u = await tokens(2);
    const r = await req('/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: u.session }) });
    expect(r.status).toBe(401);
    expect((await req('/auth/refresh', { method: 'POST' })).status).toBe(401);
  });

  it('admin refresh：只讀 admin_refresh / body', async () => {
    const { req, bearer, tokens } = setup();
    const a = await tokens(1);
    const r = await req('/admin/auth/refresh', { method: 'POST', headers: { Cookie: `admin_refresh=${a.refresh}` } });
    expect(r.status).toBe(200);
    const { token } = (await r.json()) as { token: string };
    expect((await req('/admin/users', bearer(token))).status).toBe(200);
    expect(setCookies(r).some((v) => v.startsWith('admin_session='))).toBe(true);
    // portal 的 refresh cookie 不能續後台
    const p = await req('/admin/auth/refresh', { method: 'POST', headers: { Cookie: `refresh=${a.refresh}` } });
    expect(p.status).toBe(401);
  });
});

describe('退出', () => {
  it('portal 退出後 access 與 refresh 全部失效（含其他設備）', async () => {
    const { req, bearer, tokens } = setup();
    const u = await tokens(2);
    const other = await tokens(2);
    const out = await req('/auth/logout', { method: 'POST', ...bearer(u.bearer) });
    expect(out.status).toBe(200);
    expect(setCookies(out)).toHaveLength(6);
    expect((await req('/auth/validate', bearer(u.bearer))).status).toBe(401);
    expect((await req('/auth/validate', bearer(other.bearer))).status).toBe(401);
    const r = await req('/auth/refresh', { method: 'POST', body: JSON.stringify({ refresh_token: u.refresh }) });
    expect(r.status).toBe(401);
  });

  it('會話已失效時退出仍清 cookie；舊 token 不會再次加版本', async () => {
    const { raw, req, bearer, tokens } = setup();
    const u = await tokens(2);
    await req('/auth/logout', { method: 'POST', ...bearer(u.bearer) });
    const fresh = await tokens(2);
    const out = await req('/auth/logout', { method: 'POST', ...bearer(u.bearer) });
    expect(out.status).toBe(200);
    expect(setCookies(out)).toHaveLength(6);
    expect(raw.prepare('SELECT token_version v FROM users WHERE id = 2').get()).toEqual({ v: 1 });
    expect((await req('/auth/validate', bearer(fresh.bearer))).status).toBe(200);
  });

  it('後台退出吊銷 admin 會話', async () => {
    const { req, bearer, tokens } = setup();
    const a = await tokens(1);
    const out = await req('/admin/auth/logout', { method: 'POST', headers: { Cookie: `admin_session=${a.session}` } });
    expect(out.status).toBe(200);
    expect(setCookies(out).every((v) => v.startsWith('admin_'))).toBe(true);
    expect((await req('/admin/users', bearer(a.bearer))).status).toBe(401);
  });
});

describe('/admin/auth/validate', () => {
  it('只認 admin_session / Bearer，不接受 portal 的 session cookie', async () => {
    const { req, tokens } = setup();
    const a = await tokens(1);
    expect((await req('/admin/auth/validate', { headers: { Cookie: `session=${a.session}` } })).status).toBe(401);
    const ok = await req('/admin/auth/validate', { headers: { Cookie: `admin_session=${a.session}` } });
    expect(ok.status).toBe(200);
    expect(await ok.json()).toMatchObject({ role: 'admin', user: { id: 1, username: 'admin' } });
  });

  it('普通用戶 403', async () => {
    const { req, bearer, tokens } = setup();
    expect((await req('/admin/auth/validate', bearer((await tokens(2)).bearer))).status).toBe(403);
  });

  it('admin 登入回傳 refresh_token', async () => {
    const { req } = setup();
    const r = await req('/admin/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username: 'admin', password: PASSWORD }),
    });
    expect(r.status).toBe(200);
    expect(await r.json()).toMatchObject({ role: 'admin', token: expect.any(String), refresh_token: expect.any(String) });
  });
});
