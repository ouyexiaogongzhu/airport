import { describe, expect, it } from 'vitest';
import { bearerJwt, csrfExemptForVerifiedBearer } from './csrf';

describe('csrfExemptForVerifiedBearer', () => {
  it('parses a bearer jwt and ignores garbage schemes', () => {
    expect(bearerJwt('Bearer abc.def')).toBe('abc.def');
    expect(bearerJwt('bearer abc.def')).toBe('abc.def');
    expect(bearerJwt('Token abc')).toBeUndefined();
    expect(bearerJwt('Bearer')).toBeUndefined();
  });

  it('exempts only when the verified jwt matches the bearer', () => {
    expect(csrfExemptForVerifiedBearer('Bearer good', 'good')).toBe(true);
    expect(csrfExemptForVerifiedBearer('Bearer dummy', 'good')).toBe(false);
    expect(csrfExemptForVerifiedBearer('Bearer good', undefined)).toBe(false);
    expect(csrfExemptForVerifiedBearer(undefined, 'good')).toBe(false);
  });
});
