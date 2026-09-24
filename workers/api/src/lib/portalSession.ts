// Portal 登入後發 session / refresh / csrf cookies（password 與 OAuth 共用）

import { sessionCookie, refreshCookie, csrfCookie, clearHostOnlyAuthCookies } from './cookies';
import { randomHex } from './csrf';
import { resolveJwtMaterial } from './jwtkeys';
import { signTokens } from './session';
import type { Env } from '../index';

export type SessionCredentials = {
  id: number;
  username: string;
  role: string;
  token_version?: number;
};

type CookieContext = {
  env: Env;
  header: (name: 'Set-Cookie', value: string, opts?: { append?: boolean }) => void;
};

/** session(2h)+refresh(90d)+csrf(2h)；回傳 Bearer + refresh 供跨站 localStorage 兜底 */
export async function issuePortalSession(
  c: CookieContext,
  user: SessionCredentials,
): Promise<{ token: string; refresh_token: string } | null> {
  const material = await resolveJwtMaterial(c.env);
  if (!material) return null;
  const domain = c.env.COOKIE_DOMAIN;
  const t = await signTokens(user, material, 'portal');
  if (domain) {
    for (const v of clearHostOnlyAuthCookies(['session', 'refresh', 'csrf'])) {
      c.header('Set-Cookie', v, { append: true });
    }
  }
  c.header('Set-Cookie', sessionCookie('session', t.session, domain), { append: true });
  c.header('Set-Cookie', refreshCookie('refresh', t.refresh, domain), { append: true });
  c.header('Set-Cookie', csrfCookie('csrf', randomHex(32), domain), { append: true });
  return { token: t.bearer, refresh_token: t.refresh };
}

/** vless_uuid / ss_password(16B) / trojan_password(24B) */
export function ensureUserCredentials() {
  return {
    vless_uuid: crypto.randomUUID(),
    ss_password: randomHex(16),
    trojan_password: randomHex(24),
  };
}
