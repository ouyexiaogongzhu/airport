// 會話 cookie — portal / admin 分開 TTL；refresh 共用較長壽命。
// 屬性：Path=/ Secure HttpOnly SameSite=None；Domain 取 COOKIE_DOMAIN（host-only 為空）。

/** Portal access session（session cookie + JWT）：2 小時 */
export const PORTAL_SESSION_TTL = 2 * 3600;
/** Admin access session（admin_session cookie + JWT）：30 天（既有行為） */
export const ADMIN_SESSION_TTL = 30 * 24 * 3600;
/** Refresh token（portal refresh + admin_refresh）：7 天，/auth/refresh 成功時換發 */
export const REFRESH_TTL = 7 * 24 * 3600;

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

function clearOne(name: string, domain?: string): string {
  // SameSite=None 必須帶 Secure，否則瀏覽器拒收這條 Set-Cookie
  const parts = [`${name}=`, 'Path=/', 'Max-Age=0', 'Secure', 'SameSite=None'];
  if (!name.includes('csrf')) parts.push('HttpOnly');
  if (domain) parts.push(`Domain=${domain}`);
  return parts.join('; ');
}

/**
 * 清掉全部 auth cookie。若配了 COOKIE_DOMAIN，同時清 host-only 與 Domain 兩份：
 * 遷移到 Domain=rfplay.uk 之前寫入的 host-only cookie 不會被 Domain 版 Set-Cookie 覆蓋，
 * 瀏覽器會同時帶上兩份同名 cookie，getCookie 可能拿到過期那份。
 */
export function clearAuthCookies(domain?: string): string[] {
  const out: string[] = [];
  for (const n of ['session', 'refresh', 'csrf', 'admin_session', 'admin_refresh', 'admin_csrf']) {
    out.push(clearOne(n)); // host-only
    if (domain) out.push(clearOne(n, domain));
  }
  return out;
}

/** 登入/發 session 前清掉 host-only 舊 cookie，避免與 Domain=… 新 cookie 並存 */
export function clearHostOnlyAuthCookies(names: string[]): string[] {
  return names.map((n) => clearOne(n));
}
