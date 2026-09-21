// CSRF double-submit — 對齊 Go middleware.WebCSRF（恆定時間比較）+ randomHex(32)

export function randomHex(bytes = 32): string {
  const b = new Uint8Array(bytes);
  crypto.getRandomValues(b);
  return Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
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
