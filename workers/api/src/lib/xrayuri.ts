// vless / vmess 分享鏈接生成（shadowsocks、trojan 已下線，見遷移方案 §16）
// 節點唯一形態：客戶端連 Cloudflare 邊緣，address = 節點域名，443 + tls + ws，host/sni = 域名；
// 無 Vision flow（WS 上不可用）。nodes.port 是節點本機 Xray 端口，不出現在鏈接裡。

export type NodeRow = {
  name: string | null;
  address: string | null;
  protocol: string | null;
  ws_path?: string | null;
};

export const CLIENT_PORT = 443;

export type Transport = {
  host: string;
  path: string;
};

// 與 nodeconfig 的 ws path 默認值一致
export function nodeTransport(node: NodeRow): Transport {
  return {
    host: node.address ?? '',
    path: node.ws_path || '/',
  };
}

export type UserCreds = {
  id: number;
  vless_uuid: string | null;
};

// Go base64.StdEncoding（帶 padding，UTF-8 bytes）
export function b64std(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

// Go url.QueryEscape：保留 A-Za-z0-9-_.~，空格→'+'，其餘逐 UTF-8 byte %XX 大寫
export function queryEscape(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let out = '';
  for (const b of bytes) {
    const c = String.fromCharCode(b);
    if ((b >= 0x41 && b <= 0x5a) || (b >= 0x61 && b <= 0x7a) || (b >= 0x30 && b <= 0x39) || b === 0x2d || b === 0x5f || b === 0x2e || b === 0x7e) {
      out += c;
    } else if (b === 0x20) {
      out += '+';
    } else {
      out += '%' + b.toString(16).toUpperCase().padStart(2, '0');
    }
  }
  return out;
}

export function encodeNodeToURI(node: NodeRow, user: UserCreds): string {
  switch (node.protocol) {
    case 'vmess':
      return encodeVmess(node, user);
    case 'vless':
      return encodeVless(node, user);
    default:
      return '';
  }
}

// v2rayN 分享格式，鍵按字母序輸出
function encodeVmess(node: NodeRow, user: UserCreds): string {
  const t = nodeTransport(node);
  const data = JSON.stringify({
    add: node.address ?? '',
    aid: 0,
    host: t.host,
    id: user.vless_uuid ?? '',
    net: 'ws',
    path: t.path,
    port: CLIENT_PORT,
    ps: node.name ?? '',
    sni: t.host,
    tls: 'tls',
    type: 'none',
    v: '2',
  });
  return 'vmess://' + b64std(data);
}

// query 參數按鍵序輸出
function encodeVless(node: NodeRow, user: UserCreds): string {
  const t = nodeTransport(node);
  const q = queryEscape;
  const params: [string, string][] = [
    ['encryption', 'none'],
    ['fp', 'chrome'],
    ['host', t.host],
    ['path', t.path],
    ['security', 'tls'],
    ['sni', t.host],
    ['type', 'ws'],
  ];
  const qs = params.map(([k, v]) => `${k}=${q(v)}`).join('&');
  return `vless://${user.vless_uuid ?? ''}@${node.address ?? ''}:${CLIENT_PORT}?${qs}#${q(node.name ?? '')}`;
}
