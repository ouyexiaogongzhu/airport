// admin 自建 at_ token（免註冊發放）：每個 token = 一行 account_type='token_only' 的合成用戶
//（username 'at_'+hex、password_hash 不可登入、client_token 'at_'+hex），訂閱 / 流量記賬 /
// 設備槽 / 節點用戶列表全部走 users 表既有鏈路，零改動生效。掛載點：/api/v1。
// guard/adminCsrf 與 admin.ts 內部版本相同（未導出，此處複製）；訂閱 403 判定在 lib/entitlement 不動。

import { Hono } from 'hono';
import { createMiddleware } from 'hono/factory';
import { getCookie } from 'hono/cookie';
import bcrypt from 'bcryptjs';
import { authenticateAccessCandidates, bumpTokenVersion } from '../lib/session';
import { resolveJwtMaterial } from '../lib/jwtkeys';
import { constantTimeEqual, randomHex } from '../lib/csrf';
import { ensureUserCredentials } from '../lib/portalSession';
import type { Env } from '../index';

type AppEnv = { Bindings: Env; Variables: { userId: number; username: string; role: string } };

const BCRYPT_COST = 10;
const GB_BYTES = 1073741824;

// note 復用 display_name；client_token 只出前 11 字符（明文僅在簽發/續期響應出現一次）
const TOKEN_COLS =
  'id, username, display_name AS note, subscription_status, expire_time, ' +
  'traffic_used_bytes, traffic_limit_bytes, substr(client_token, 1, 11) AS client_token_prefix, created_at';

async function allocateTokenUsername(db: D1Database): Promise<string> {
  for (let i = 0; i < 8; i++) {
    const candidate = 'at_' + randomHex(6);
    const taken = await db.prepare('SELECT id FROM users WHERE username = ?').bind(candidate).first();
    if (!taken) return candidate;
  }
  return 'at_' + randomHex(16);
}

type IssueInput = { duration_days: number; traffic_limit_gb: number; note: string | null };

function parseIssue(body: Record<string, unknown>): IssueInput | { error: string } {
  const d = body.duration_days;
  if (typeof d !== 'number' || !Number.isSafeInteger(d) || d <= 0) {
    return { error: 'duration_days must be a positive integer' };
  }
  const gb = body.traffic_limit_gb;
  if (typeof gb !== 'number' || !Number.isFinite(gb) || gb <= 0) {
    return { error: 'traffic_limit_gb must be a positive number' };
  }
  let note: string | null = null;
  if (body.note !== undefined && body.note !== null) {
    if (typeof body.note !== 'string') return { error: 'note must be a string' };
    note = body.note.trim();
    if (note.length > 200) return { error: 'note must be at most 200 characters' };
    if (note === '') note = null;
  }
  return { duration_days: d, traffic_limit_gb: gb, note };
}

// 建合成用戶（照 oauth.ts 新建用戶段）；expire_time = Unix 秒（與 grant/stats 一致）
async function createTokenUser(db: D1Database, input: IssueInput): Promise<{ id: number; token: string } | null> {
  const hash = await bcrypt.hash(randomHex(32), BCRYPT_COST);
  const creds = ensureUserCredentials();
  const token = 'at_' + randomHex(32);
  const nowIso = new Date().toISOString();
  const expireTime = Math.floor(Date.now() / 1000) + input.duration_days * 86400;
  const username = await allocateTokenUsername(db);
  const r = await db
    .prepare(
      `INSERT INTO users (username, password_hash, role, status, subscription_status, traffic_limit_bytes,
         expire_time, client_token, vless_uuid, ss_password, trojan_password, email, display_name, account_type,
         created_at, updated_at)
       VALUES (?, ?, 'user', 'active', 'active', ?, ?, ?, ?, ?, ?, NULL, ?, 'token_only', ?, ?)`,
    )
    .bind(
      username,
      hash,
      Math.round(input.traffic_limit_gb * GB_BYTES),
      expireTime,
      token,
      creds.vless_uuid,
      creds.ss_password,
      creds.trojan_password,
      input.note,
      nowIso,
      nowIso,
    )
    .run();
  const id = Number(r.meta.last_row_id);
  return id ? { id, token } : null;
}

async function suspendTokenUser(db: D1Database, id: number, tokenVersion: number): Promise<void> {
  await db
    .prepare("UPDATE users SET subscription_status = 'suspended', updated_at = ? WHERE id = ?")
    .bind(new Date().toISOString(), id)
    .run();
  await bumpTokenVersion(db, id, tokenVersion).run();
}

export function tokenRoutes() {
  const app = new Hono<AppEnv>();

  // ── guard：與 admin.ts 內部中間件逐字相同（該處未導出）──

  const adminAuth = createMiddleware<AppEnv>(async (c, next) => {
    const material = await resolveJwtMaterial(c.env);
    const bearer = c.req.header('Authorization')?.replace(/^Bearer /i, '');
    const r = await authenticateAccessCandidates(c.env.DB, material ?? undefined, [
      material ? getCookie(c, 'admin_session') : undefined,
      bearer,
    ]);
    if (!('user' in r)) return c.json({ error: 'SESSION_EXPIRED' }, 401);
    c.set('userId', r.user.id);
    c.set('username', r.user.username);
    c.set('role', r.user.role);
    await next();
  });

  const adminOnly = createMiddleware<AppEnv>(async (c, next) => {
    if (c.get('role') !== 'admin') return c.json({ error: 'admin access required' }, 403);
    await next();
  });

  const adminCsrf = createMiddleware<AppEnv>(async (c, next) => {
    if (c.req.method === 'GET' || c.req.method === 'HEAD' || c.req.method === 'OPTIONS') {
      await next();
      return;
    }
    if (c.req.header('Authorization')) {
      await next();
      return;
    }
    const header = c.req.header('X-CSRF-Token');
    const cookie = getCookie(c, 'admin_csrf');
    if (!header || !cookie || !constantTimeEqual(header, cookie)) {
      return c.json({ error: 'CSRF_INVALID' }, 403);
    }
    await next();
  });

  const guard = [adminAuth, adminOnly] as const;

  // ── 簽發 ────────────────────────────────────────────────────────────────────

  app.post('/admin/tokens', ...guard, adminCsrf, async (c) => {
    const body = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);
    const input = parseIssue(body);
    if ('error' in input) return c.json({ error: input.error }, 400);
    const created = await createTokenUser(c.env.DB, input);
    if (!created) return c.json({ error: 'failed to create token' }, 500);
    const row = await c.env.DB.prepare(`SELECT ${TOKEN_COLS} FROM users WHERE id = ?`)
      .bind(created.id)
      .first<Record<string, unknown>>();
    return c.json({ token: created.token, ...(row ?? {}) }, 201);
  });

  // ── 列表：account_type='token_only'，id DESC ───────────────────────────────

  app.get('/admin/tokens', ...guard, async (c) => {
    const rs = await c.env.DB.prepare(
      `SELECT ${TOKEN_COLS} FROM users WHERE account_type = 'token_only' ORDER BY id DESC`,
    ).all<Record<string, unknown>>();
    return c.json({ data: rs.results });
  });

  // ── 吊銷：suspended + bump token_version（訂閱即 403，鏈路零改動）───────────

  app.post('/admin/tokens/:id/revoke', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid token id' }, 400);
    const db = c.env.DB;
    const row = await db
      .prepare("SELECT id, token_version FROM users WHERE id = ? AND account_type = 'token_only'")
      .bind(id)
      .first<{ id: number; token_version: number }>();
    if (!row) return c.json({ error: 'token not found' }, 404);
    await suspendTokenUser(db, id, row.token_version);
    return c.json({ ok: true });
  });

  // ── 續期：舊行作廢 + 新建一行（note 沿用），返回新 token 明文 ───────────────

  app.post('/admin/tokens/:id/renew', ...guard, adminCsrf, async (c) => {
    const id = Number(c.req.param('id'));
    if (!Number.isInteger(id) || id <= 0) return c.json({ error: 'invalid token id' }, 400);
    const body = await c.req.json<Record<string, unknown>>().catch(() => null);
    if (body === null) return c.json({ error: 'invalid request body' }, 400);
    const input = parseIssue(body);
    if ('error' in input) return c.json({ error: input.error }, 400);
    const db = c.env.DB;
    const old = await db
      .prepare("SELECT id, token_version, display_name FROM users WHERE id = ? AND account_type = 'token_only'")
      .bind(id)
      .first<{ id: number; token_version: number; display_name: string | null }>();
    if (!old) return c.json({ error: 'token not found' }, 404);
    await suspendTokenUser(db, id, old.token_version);
    const created = await createTokenUser(db, { ...input, note: old.display_name });
    if (!created) return c.json({ error: 'failed to create token' }, 500);
    const row = await db.prepare(`SELECT ${TOKEN_COLS} FROM users WHERE id = ?`)
      .bind(created.id)
      .first<Record<string, unknown>>();
    return c.json({ token: created.token, ...(row ?? {}) }, 201);
  });

  return app;
}
