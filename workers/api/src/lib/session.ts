// 會話校驗與簽發：JWT 只證明身份，每次請求回庫比對 token_version 與 status，role 以庫為準。
// 吊銷 = token_version + 1（退出、改密、封禁），該用戶所有已簽發的 access/refresh 立即失效。
// 簽名密鑰輪換見 jwtkeys.ts：簽發用 current；校驗 current→previous，不因輪換全局登出。

import { signJwt, verifyJwt, type Claims } from './jwt';
import { verifySecrets, type JwtMaterial } from './jwtkeys';
import { PORTAL_SESSION_TTL, ADMIN_SESSION_TTL, REFRESH_TTL } from './cookies';

/** Admin / 通用 Bearer 兜底：24h（對齊既有 Go generateToken） */
export const BEARER_TTL = 24 * 3600;
/** Portal Bearer（跨站 localStorage 兜底）與 portal session 同壽：2h */
export const PORTAL_BEARER_TTL = PORTAL_SESSION_TTL;

/** 字串 = 單密鑰（測試）；JwtMaterial = KV 輪換材料 */
export type JwtSecrets = string | JwtMaterial;

function signingOf(secrets: JwtSecrets): { secret: string; kid?: string } {
  if (typeof secrets === 'string') return { secret: secrets };
  return { secret: secrets.current.secret, kid: secrets.current.kid };
}

function verifyList(secrets: JwtSecrets): string[] {
  return typeof secrets === 'string' ? [secrets] : verifySecrets(secrets);
}

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
  secrets: JwtSecrets,
  kind: 'access' | 'refresh' = 'access',
): Promise<SessionResult> {
  if (!token) return { error: 'invalid' };
  const claims = await verifyJwt(token, verifyList(secrets));
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
  secrets: JwtSecrets | undefined,
  candidates: Array<string | undefined | null>,
): Promise<SessionResult> {
  if (!secrets) return { error: 'invalid' };
  let last: SessionResult = { error: 'invalid' };
  const seen = new Set<string>();
  for (const raw of candidates) {
    if (!raw || seen.has(raw)) continue;
    seen.add(raw);
    last = await authenticate(db, raw, secrets, 'access');
    if ('user' in last) return last;
  }
  return last;
}

type Signable = { id: number; username: string; role: string; token_version?: number | null };

function baseClaims(user: Signable) {
  return { user_id: user.id, username: user.username, role: user.role, tv: user.token_version ?? 0 };
}

export type SessionKind = 'portal' | 'admin';

export async function signTokens(user: Signable, secrets: JwtSecrets, kind: SessionKind = 'portal') {
  const { secret, kid } = signingOf(secrets);
  const base = baseClaims(user);
  const accessTtl = kind === 'admin' ? ADMIN_SESSION_TTL : PORTAL_SESSION_TTL;
  const bearerTtl = kind === 'admin' ? BEARER_TTL : PORTAL_BEARER_TTL;
  return {
    session: await signJwt(base, secret, accessTtl, kid),
    refresh: await signJwt({ ...base, typ: 'refresh' }, secret, REFRESH_TTL, kid),
    bearer: await signJwt(base, secret, bearerTtl, kid),
  };
}

export function signAccess(user: Signable, secrets: JwtSecrets, ttl: number) {
  const { secret, kid } = signingOf(secrets);
  return signJwt(baseClaims(user), secret, ttl, kid);
}

// 僅當庫中版本仍等於 expected 時加一：舊 token 無法觸發，並發吊銷只生效一次
export function bumpTokenVersion(db: D1Database, userId: number, expected: number) {
  return db
    .prepare('UPDATE users SET token_version = token_version + 1 WHERE id = ? AND token_version = ?')
    .bind(userId, expected);
}
