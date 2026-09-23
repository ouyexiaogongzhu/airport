// Turnstile siteverify — canonical form-encoded POST（對齊 turnstile-spin skill 的 canonical idiom）。
// 只有 TURNSTILE_DISABLED="1" 時跳過；未配 secret 視為配置錯誤 → fail closed。
// 校驗失敗 / siteverify 不可達 → fail closed。

import type { Env } from '../index';

export type TurnstileResult = { ok: true } | { ok: false; status: 400 | 403 | 500; error: string };

const SITEVERIFY_URL = 'https://challenges.cloudflare.com/turnstile/v0/siteverify';

export async function verifyTurnstile(
  token: unknown,
  ip: string | undefined,
  env: Pick<Env, 'TURNSTILE_SECRET' | 'TURNSTILE_DISABLED'>,
): Promise<TurnstileResult> {
  if (env.TURNSTILE_DISABLED === '1') return { ok: true };
  const secret = env.TURNSTILE_SECRET;
  if (!secret) return { ok: false, status: 500, error: 'TURNSTILE_NOT_CONFIGURED' };
  if (typeof token !== 'string' || token.length === 0 || token.length > 2048) {
    return { ok: false, status: 400, error: 'TURNSTILE_TOKEN_REQUIRED' };
  }
  const body = new URLSearchParams({ secret, response: token });
  if (ip) body.set('remoteip', ip);
  try {
    const res = await fetch(SITEVERIFY_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
      signal: AbortSignal.timeout(10_000),
    });
    if (!res.ok) return { ok: false, status: 403, error: 'TURNSTILE_FAILED' };
    const data = (await res.json()) as { success?: boolean };
    if (data.success !== true) return { ok: false, status: 403, error: 'TURNSTILE_FAILED' };
    return { ok: true };
  } catch {
    // 網路錯誤 / 非 2xx / 非 JSON：fail closed
    return { ok: false, status: 403, error: 'TURNSTILE_FAILED' };
  }
}
