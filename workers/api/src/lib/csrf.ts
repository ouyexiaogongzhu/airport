// CSRF double-submit — 對齊 Go middleware.WebCSRF（恆定時間比較）+ randomHex(32)

export function randomHex(bytes = 32): string {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
}

/** `Authorization: Bearer <jwt>` 的 jwt；格式不对则 undefined。 */
export function bearerJwt(authorization: string | undefined): string | undefined {
  if (!authorization) return undefined;
  const idx = authorization.indexOf(' ');
  if (idx <= 0) return undefined;
  if (authorization.slice(0, idx).toLowerCase() !== 'bearer') return undefined;
  const token = authorization.slice(idx + 1).trim();
  return token || undefined;
}

/**
 * CSRF 豁免仅当本次通过校验的 access JWT 就是 Authorization Bearer 本身。
 * 仅仅“有 Authorization 头”不够：cookie 验过、头是假 Bearer 时仍要双提交。
 */
export function csrfExemptForVerifiedBearer(
  authorization: string | undefined,
  acceptedJwt: string | undefined,
): boolean {
  const token = bearerJwt(authorization);
  if (!token || !acceptedJwt) return false;
  return constantTimeEqual(token, acceptedJwt);
}

// 恆定時間比較（對齊 subtle.ConstantTimeCompare 語義）
export function constantTimeEqual(a: string, b: string): boolean {
  // 無長度早退：長度差也進 Mix，避免 length oracle
  const maxLen = Math.max(a.length, b.length);
  let diff = a.length ^ b.length;
  for (let i = 0; i < maxLen; i++) {
    diff |= (a.charCodeAt(i) || 0) ^ (b.charCodeAt(i) || 0);
  }
  return diff === 0;
}
