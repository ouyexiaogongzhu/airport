// 會話校驗與簽發：JWT 只證明身份，每次請求回庫比對 token_version 與 status，role 以庫為準。
// 吊銷 = token_version + 1（退出、改密、封禁），該用戶所有已簽發的 access/refresh 立即失效。

import { signJwt, verifyJwt, type Claims } from './jwt';
import { SESSION_TTL, REFRESH_TTL } from './cookies';

export const BEARER_TTL = 24 * 3600; // Go generateToken: 24h

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

type Signable = { id: number; username: string; role: string; token_version?: number | null };

function baseClaims(user: Signable) {
  return { user_id: user.id, username: user.username, role: user.role, tv: user.token_version ?? 0 };
}

export async function signTokens(user: Signable, secret: string) {
  const base = baseClaims(user);
  return {
    session: await signJwt(base, secret, SESSION_TTL),
    refresh: await signJwt({ ...base, typ: 'refresh' }, secret, REFRESH_TTL),
    bearer: await signJwt(base, secret, BEARER_TTL),
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
