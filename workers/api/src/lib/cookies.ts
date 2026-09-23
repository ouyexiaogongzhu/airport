// 會話 cookie — portal / admin 分開 TTL；refresh 共用較長壽命。
// 屬性：Path=/ Secure HttpOnly SameSite=None；Domain 取 COOKIE_DOMAIN（host-only 為空）。

/** Portal access session（session cookie + JWT）：2 小時 */
export const PORTAL_SESSION_TTL = 2 * 3600;
/** Admin access session（admin_session cookie + JWT）：30 天（既有行為） */
export const ADMIN_SESSION_TTL = 30 * 24 * 3600;
/** Refresh token（portal refresh + admin_refresh）：90 天，不變 */
export const REFRESH_TTL = 90 * 24 * 3600;

/** @deprecated 請改用 PORTAL_SESSION_TTL / ADMIN_SESSION_TTL；保留別名以免外部誤用 */
export const SESSION_TTL = PORTAL_SESSION_TTL;

export type CookieOptions = {
  maxAge: number;
  httpOnly?: boolean;
  domain?: string;
};

export function buildSetCookie(name: string, value: string, opts: CookieOptions): string {
  const parts = [`${name}=${value}`, 'Path=/', `Max-Age=${opts.maxAge}`, 'Secure', 'SameSite=None'];
  if (opts.httpOnly !== false) parts.push('HttpOnly');
  if (opts.domain) parts.push(`Domain=${opts.domain}`);
  return parts.join('; ');
}

function accessTtlForCookie(name: string): number {
  return name.startsWith('admin_') ? ADMIN_SESSION_TTL : PORTAL_SESSION_TTL;
}

export function sessionCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: accessTtlForCookie(name), httpOnly: true, domain });
}

export function refreshCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: REFRESH_TTL, httpOnly: true, domain });
}

export function csrfCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: accessTtlForCookie(name), httpOnly: false, domain });
}

export function clearAuthCookies(domain?: string): string[] {
  const out: string[] = [];
  for (const n of ['session', 'refresh', 'csrf', 'admin_session', 'admin_refresh', 'admin_csrf']) {
    // SameSite=None 必須帶 Secure，否則瀏覽器拒收這條 Set-Cookie
    const parts = [`${n}=`, 'Path=/', 'Max-Age=0', 'Secure', 'SameSite=None'];
    if (!n.includes('csrf')) parts.push('HttpOnly');
    if (domain) parts.push(`Domain=${domain}`);
    out.push(parts.join('; '));
  }
  return out;
}
