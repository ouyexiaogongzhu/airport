// 節點請求簽名 — 對齊 daemon internal/sync/sync.go signRequest / nodeHMACSecret：
// key = sha256("rfplay-node-hmac-v1:" + token)；msg = method \n path \n ts \n body；hex(HMAC-SHA256)
import { constantTimeEqual } from './csrf';

const enc = new TextEncoder();

// 允許的時鐘偏差（秒）
export const NODE_SIG_SKEW = 300;

function hex(buf: ArrayBuffer): string {
  return Array.from(new Uint8Array(buf), (x) => x.toString(16).padStart(2, '0')).join('');
}

export async function nodeSignature(token: string, method: string, path: string, ts: string, body: string): Promise<string> {
  const secret = await crypto.subtle.digest('SHA-256', enc.encode('rfplay-node-hmac-v1:' + token));
  const key = await crypto.subtle.importKey('raw', secret, { name: 'HMAC', hash: 'SHA-256' }, false, ['sign']);
  const sig = await crypto.subtle.sign('HMAC', key, enc.encode(`${method}\n${path}\n${ts}\n${body}`));
  return hex(sig);
}

export async function verifyNodeSignature(
  token: string,
  method: string,
  path: string,
  ts: string | undefined,
  sig: string | undefined,
  body: string,
  now: number,
): Promise<boolean> {
  if (!ts || !sig || !/^\d+$/.test(ts)) return false;
  if (Math.abs(now - Number(ts)) > NODE_SIG_SKEW) return false;
  return constantTimeEqual(sig.toLowerCase(), await nodeSignature(token, method, path, ts, body));
}
