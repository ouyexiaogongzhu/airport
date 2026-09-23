import { describe, expect, it } from 'vitest';
import { verifyTurnstile } from './turnstile';

describe('verifyTurnstile', () => {
  it('TURNSTILE_DISABLED=1 時放行', async () => {
    expect(await verifyTurnstile(undefined, undefined, { TURNSTILE_DISABLED: '1' })).toEqual({ ok: true });
  });

  it('未配置 secret 且未顯式關閉時 fail closed', async () => {
    expect(await verifyTurnstile('tok', undefined, {})).toEqual({
      ok: false,
      status: 500,
      error: 'TURNSTILE_NOT_CONFIGURED',
    });
  });

  it('有 secret 但缺 token 回 400', async () => {
    expect(await verifyTurnstile('', undefined, { TURNSTILE_SECRET: 's' })).toEqual({
      ok: false,
      status: 400,
      error: 'TURNSTILE_TOKEN_REQUIRED',
    });
  });
});
