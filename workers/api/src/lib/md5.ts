// 纯 JS MD5（RFC 1321）— WebCrypto 不支持 MD5；BEpusdt 是遗留 MD5 签名方案，无法更换只能重现
// （cloudflare_migration_plan.md §5.1）。输入按 UTF-8 编码（等价 Go md5.Sum([]byte(s))），输出小写 hex。

const SHIFTS = [
  7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22,
  5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20, 5, 9, 14, 20,
  4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23,
  6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
];

const K = new Uint32Array(64);
for (let i = 0; i < 64; i++) {
  K[i] = Math.floor(Math.abs(Math.sin(i + 1)) * 4294967296);
}

export function md5Hex(input: string): string {
  const msg = new TextEncoder().encode(input);
  const len = msg.length;

  // 填充：追加 0x80、零字节，末 8 字节放比特长度（小端），总长为 64 字节倍数
  const total = (((len + 8) >> 6) + 1) << 6;
  const words = new Uint32Array(total >> 2);
  for (let i = 0; i < len; i++) {
    words[i >> 2] |= msg[i] << ((i & 3) * 8);
  }
  words[len >> 2] |= 0x80 << ((len & 3) * 8);
  const bits = len * 8;
  words[words.length - 2] = bits >>> 0;
  words[words.length - 1] = Math.floor(bits / 4294967296);

  let a = 0x67452301;
  let b = 0xefcdab89;
  let c = 0x98badcfe;
  let d = 0x10325476;

  for (let off = 0; off < words.length; off += 16) {
    let A = a;
    let B = b;
    let C = c;
    let D = d;
    for (let i = 0; i < 64; i++) {
      let f: number;
      let g: number;
      if (i < 16) {
        f = (B & C) | (~B & D);
        g = i;
      } else if (i < 32) {
        f = (D & B) | (~D & C);
        g = (5 * i + 1) & 15;
      } else if (i < 48) {
        f = B ^ C ^ D;
        g = (3 * i + 5) & 15;
      } else {
        f = C ^ (B | ~D);
        g = (7 * i) & 15;
      }
      const sum = (A + f + K[i] + words[off + g]) | 0;
      const s = SHIFTS[i];
      const rotated = ((sum << s) | (sum >>> (32 - s))) | 0;
      const temp = D;
      D = C;
      C = B;
      B = (B + rotated) | 0;
      A = temp;
    }
    a = (a + A) | 0;
    b = (b + B) | 0;
    c = (c + C) | 0;
    d = (d + D) | 0;
  }

  let hex = '';
  for (const w of [a, b, c, d]) {
    for (let i = 0; i < 4; i++) {
      hex += ((w >>> (i * 8)) & 0xff).toString(16).padStart(2, '0');
    }
  }
  return hex;
}
