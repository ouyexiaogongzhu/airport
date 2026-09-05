// 會話 cookie — 逐項對齊 manager/internal/handler/auth.go
// session/admin_session: 30d；refresh/admin_refresh: 90d；csrf/admin_csrf: 30d 非 httpOnly。
// 屬性：Path=/ Secure HttpOnly SameSite=Strict；Domain 取 COOKIE_DOMAIN（host-only 為空）。

export const SESSION_TTL = 30 * 24 * 3600; // 秒，與 Go sessionCookieTTL 一致
export const REFRESH_TTL = 90 * 24 * 3600; // 秒，與 Go refreshCookieTTL 一致

export type CookieOptions = {
  maxAge: number;
  httpOnly?: boolean;
  domain?: string;
};

export function buildSetCookie(name: string, value: string, opts: CookieOptions): string {
  const parts = [`${name}=${value}`, 'Path=/', `Max-Age=${opts.maxAge}`, 'Secure', 'SameSite=Strict'];
  if (opts.httpOnly !== false) parts.push('HttpOnly');
  if (opts.domain) parts.push(`Domain=${opts.domain}`);
  return parts.join('; ');
}

export function sessionCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: SESSION_TTL, httpOnly: true, domain });
}

export function refreshCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: REFRESH_TTL, httpOnly: true, domain });
}

export function csrfCookie(name: string, token: string, domain?: string): string {
  return buildSetCookie(name, token, { maxAge: SESSION_TTL, httpOnly: false, domain });
}

export function clearAuthCookies(domain?: string): string[] {
  const out: string[] = [];
  for (const n of ['session', 'refresh', 'csrf', 'admin_session', 'admin_refresh', 'admin_csrf']) {
    const parts = [`${n}=`, 'Path=/', 'Max-Age=0', 'Secure', 'SameSite=Strict'];
    if (n.includes('csrf')) {
      parts.splice(parts.indexOf('Secure'), 1); // csrf 非 HttpOnly 也非必須 Secure 清除差異，保持一致性
    } else {
      parts.push('HttpOnly');
    }
    if (domain) parts.push(`Domain=${domain}`);
    out.push(parts.join('; '));
  }
  return out;
}
