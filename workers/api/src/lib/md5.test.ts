// RFC 1321 官方测试向量 + 常见已知值（覆盖空串、单块、多块、满 55/56 字节边界）
import { describe, expect, it } from 'vitest';
import { md5Hex } from './md5';
import { bepusdtSign } from './payments';

describe('md5Hex', () => {
  it.each([
    ['', 'd41d8cd98f00b204e9800998ecf8427e'],
    ['a', '0cc175b9c0f1b6a831c399e269772661'],
    ['abc', '900150983cd24fb0d6963f7d28e17f72'],
    ['message digest', 'f96b697d7cb7938d525a2f31aaf161d0'],
    ['abcdefghijklmnopqrstuvwxyz', 'c3fcd3d76192e4007dfb496cca67e13b'],
    [
      'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789',
      'd174ab98d277d9f5a5611c2c9f419d9f',
    ],
    [
      '12345678901234567890123456789012345678901234567890123456789012345678901234567890',
      '57edf4a22be3c955ac49da2e2107b67a',
    ],
    ['The quick brown fox jumps over the lazy dog', '9e107d9d372bb6826bd81d3542a419d6'],
    // 填充边界（55/56 字节，经 python3 hashlib 核对）：55→单块放得下 0x80+长度；56→被迫双块
    ['A'.repeat(55), 'e38a93ffe074a99b3fed47dfbe37db21'],
    ['A'.repeat(56), 'a2f3e2024931bd470555002aa5ccc010'],
    ['A'.repeat(100), '8adc5937e635f6c9af646f0b23560fae'],
  ])('md5(%j) = %s', (input, expected) => {
    expect(md5Hex(input)).toBe(expected);
  });

  it('输出恒为 32 位小写 hex', () => {
    const h = md5Hex('RFPlay');
    expect(h).toMatch(/^[0-9a-f]{32}$/);
  });
});

// bepusdtSign 对拍：期望值由 manager/internal/handler/payment_provider.go 的 bepusdtSign 原样
// 跑 Go 得出（含空参数剔除、signature 剔除两个边界）
describe('bepusdtSign', () => {
  it('建单参数（剔除 signature/空值，ASCII 排序）', () => {
    expect(
      bepusdtSign(
        {
          order_id: '42',
          amount: '19.9',
          notify_url: 'https://api.rfplay.uk/x',
          redirect_url: 'https://rfplay.uk/y',
          signature: 'ignored',
          empty: '',
        },
        'test-token-123',
      ),
    ).toBe('962fcc4b88d016b072461798f2bfd84b');
  });

  it('回调 8 字段', () => {
    expect(
      bepusdtSign(
        {
          order_id: 'ORD-9',
          amount: '100',
          actual_amount: '99.5',
          token: 'TRC20',
          status: '2',
          block_transaction_id: 'abc123',
          created_at: '1700000000',
          expired_at: '1700086400',
        },
        'secret-secret',
      ),
    ).toBe('c27cd4fae2859afa14a0fee7274c957f');
  });
});
