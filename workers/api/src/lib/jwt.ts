// HS256 JWT — 對齊 Go golang-jwt/v5（claims: user_id/username/role/exp/iat）
// 用途：httpOnly cookie 會話 + Flutter Bearer，同一 JWT_SECRET 下可互相驗證。

const enc = new TextEncoder();

function b64url(bytes: ArrayBuffer | Uint8Array): string {
  const b = bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes);
  let s = '';
  for (const x of b) s += String.fromCharCode(x);
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// Uint8Array<ArrayBuffer>：TS≥5.7 typed-array 泛型下 crypto.subtle 需要精確 buffer 型別（行為不變）
function b64urlDecode(s: string): Uint8Array<ArrayBuffer> {
  s = s.replace(/-/g, '+').replace(/_/g, '/');
  while (s.length % 4) s += '=';
  const bin = atob(s);
  return Uint8Array.from(bin, (c) => c.charCodeAt(0));
}

async function key(secret: string): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', enc.encode(secret), { name: 'HMAC', hash: 'SHA-256' }, false, [
    'sign',
    'verify',
  ]);
}

export type Claims = {
  user_id: number;
  username: string;
  role: string;
  exp: number;
  iat: number;
};

export async function signJwt(
  claims: Omit<Claims, 'exp' | 'iat'>,
  secret: string,
  ttlSeconds: number,
): Promise<string> {
  const now = Math.floor(Date.now() / 1000);
  const header = b64url(enc.encode(JSON.stringify({ alg: 'HS256', typ: 'JWT' })));
  const payload = b64url(enc.encode(JSON.stringify({ ...claims, exp: now + ttlSeconds, iat: now })));
  const data = `${header}.${payload}`;
  const sig = await crypto.subtle.sign('HMAC', await key(secret), enc.encode(data));
  return `${data}.${b64url(sig)}`;
}

export async function verifyJwt(token: string, secret: string): Promise<Claims | null> {
  const parts = token.split('.');
  if (parts.length !== 3) return null;
  const [header, payload, sig] = parts;
  try {
    const ok = await crypto.subtle.verify(
      'HMAC',
      await key(secret),
      b64urlDecode(sig),
      enc.encode(`${header}.${payload}`),
    );
    if (!ok) return null;
    const claims = JSON.parse(new TextDecoder().decode(b64urlDecode(payload))) as Claims;
    if (typeof claims.exp === 'number' && Math.floor(Date.now() / 1000) >= claims.exp) return null;
    return claims;
  } catch {
    return null;
  }
}
