// 會話校驗與簽發：JWT 只證明身份，每次請求回庫比對 token_version 與 status，role 以庫為準。
// 吊銷 = token_version + 1（退出、改密、封禁），該用戶所有已簽發的 access/refresh 立即失效。

import { signJwt, verifyJwt, type Claims } from './jwt';
import { PORTAL_SESSION_TTL, ADMIN_SESSION_TTL, REFRESH_TTL } from './cookies';

/** Admin / 通用 Bearer 兜底：24h（對齊既有 Go generateToken） */
export const BEARER_TTL = 24 * 3600;
/** Portal Bearer（跨站 localStorage 兜底）與 portal session 同壽：2h */
export const PORTAL_BEARER_TTL = PORTAL_SESSION_TTL;

export type SessionUser = {
  id: number;
  username: string;
  role: string;
  status: string;
  token_version: number;
};

export type SessionResult = { user: SessionUser } | { error: 'invalid' | 'revoked' | 'disabled' };

export async function checkClaims(db: D1Database, claims: Claims): Promise<SessionResult> {
  const user = await db
    .prepare('SELECT id, username, role, status, token_version FROM users WHERE id = ?')
    .bind(claims.user_id)
    .first<SessionUser>();
  if (!user) return { error: 'invalid' };
  if ((claims.tv ?? 0) !== user.token_version) return { error: 'revoked' };
  if (user.status !== 'active') return { error: 'disabled' };
  return { user };
}

// kind='access' 拒絕 refresh token（防 90 天 refresh 被當 access 用）；kind='refresh' 只收 refresh token
export async function authenticate(
  db: D1Database,
  token: string | undefined,
  secret: string,
  kind: 'access' | 'refresh' = 'access',
): Promise<SessionResult> {
  if (!token) return { error: 'invalid' };
  const claims = await verifyJwt(token, secret);
  if (!claims || typeof claims.user_id !== 'number') return { error: 'invalid' };
  if ((claims.typ === 'refresh') !== (kind === 'refresh')) return { error: 'invalid' };
  return checkClaims(db, claims);
}

/**
 * Cookie 優先、Bearer 兜底；但 cookie 無效（過期 / typ=refresh / token_version 不符）時必須繼續試 Bearer。
 * 舊邏輯 `cookie || bearer` 在「過期 Domain cookie + 有效 localStorage Bearer」時會直接 401，
 * 表現為登入成功後立刻被前端 401 interceptor 踢回登入頁。
 */
export async function authenticateAccessCandidates(
  db: D1Database,
  secret: string | undefined,
  candidates: Array<string | undefined | null>,
): Promise<SessionResult> {
  if (!secret) return { error: 'invalid' };
  let last: SessionResult = { error: 'invalid' };
  const seen = new Set<string>();
  for (const raw of candidates) {
    if (!raw || seen.has(raw)) continue;
    seen.add(raw);
    last = await authenticate(db, raw, secret, 'access');
    if ('user' in last) return last;
  }
  return last;
}

type Signable = { id: number; username: string; role: string; token_version?: number | null };

function baseClaims(user: Signable) {
  return { user_id: user.id, username: user.username, role: user.role, tv: user.token_version ?? 0 };
}

export type SessionKind = 'portal' | 'admin';

export async function signTokens(user: Signable, secret: string, kind: SessionKind = 'portal') {
  const base = baseClaims(user);
  const accessTtl = kind === 'admin' ? ADMIN_SESSION_TTL : PORTAL_SESSION_TTL;
  const bearerTtl = kind === 'admin' ? BEARER_TTL : PORTAL_BEARER_TTL;
  return {
    session: await signJwt(base, secret, accessTtl),
    refresh: await signJwt({ ...base, typ: 'refresh' }, secret, REFRESH_TTL),
    bearer: await signJwt(base, secret, bearerTtl),
  };
}

export function signAccess(user: Signable, secret: string, ttl: number) {
  return signJwt(baseClaims(user), secret, ttl);
}

// 僅當庫中版本仍等於 expected 時加一：舊 token 無法觸發，並發吊銷只生效一次
export function bumpTokenVersion(db: D1Database, userId: number, expected: number) {
  return db
    .prepare('UPDATE users SET token_version = token_version + 1 WHERE id = ? AND token_version = ?')
    .bind(userId, expected);
}
