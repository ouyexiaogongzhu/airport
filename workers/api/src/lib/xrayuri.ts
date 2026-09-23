// 對齊 manager/internal/handler/links.go — vless/vmess/ss/trojan URI 生成（逐字節契約）

export type NodeRow = {
  name: string | null;
  address: string | null;
  port: number | null;
  protocol: string | null;
  reality_public_key: string | null;
  reality_short_id: string | null;
  network?: string | null;
  security?: string | null;
  ws_path?: string | null;
  server_name?: string | null;
};

export type Transport = {
  network: string;
  security: string;
  host: string;
  path: string;
};

// 與 admin buildNodeXrayConfig 同一套默認值，保證客戶端鏈接與節點 inbound 一致
export function nodeTransport(node: NodeRow): Transport {
  const address = node.address ?? '';
  return {
    network: node.network || 'tcp',
    security: node.security || 'none',
    host: node.server_name || address,
    path: node.ws_path || '/',
  };
}

// Vision 只能跑在 TCP + TLS/REALITY 上；WS（含 Cloudflare CDN）必須留空
export function usesVision(protocol: string | null | undefined, t: Pick<Transport, 'network' | 'security'>): boolean {
  return protocol === 'vless' && t.network === 'tcp' && (t.security === 'tls' || t.security === 'reality');
}

export type UserCreds = {
  id: number;
  vless_uuid: string | null;
  ss_password: string | null;
  trojan_password: string | null;
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
    case 'shadowsocks':
      return encodeShadowsocks(node, user);
    case 'trojan':
      return encodeTrojan(node, user);
    default:
      return '';
  }
}

// v2rayN 分享格式，鍵按字母序輸出
function encodeVmess(node: NodeRow, user: UserCreds): string {
  const t = nodeTransport(node);
  const tls = t.security === 'tls';
  const data = JSON.stringify({
    add: node.address ?? '',
    aid: 0,
    host: t.network === 'ws' ? t.host : '',
    id: user.vless_uuid ?? '',
    net: t.network,
    path: t.network === 'ws' ? t.path : '',
    port: node.port ?? 0,
    ps: node.name ?? '',
    sni: tls ? t.host : '',
    tls: tls ? 'tls' : '',
    type: 'none',
    v: '2',
  });
  return 'vmess://' + b64std(data);
}

// query 參數按鍵序輸出
function encodeVless(node: NodeRow, user: UserCreds): string {
  const addr = node.address ?? '';
  const t = nodeTransport(node);
  const q = queryEscape;
  const params: [string, string][] = [];
  // 舊節點可能只填了 Reality 公鑰而 security 仍是默認 none
  const reality = t.security === 'reality' || (t.security !== 'tls' && !!node.reality_public_key);
  if (reality) {
    params.push(
      ['flow', 'xtls-rprx-vision'],
      ['fp', 'chrome'],
      ['pbk', node.reality_public_key ?? ''],
      ['security', 'reality'],
      ['sid', node.reality_short_id ?? ''],
      ['sni', t.host],
      ['type', 'tcp'],
    );
  } else {
    params.push(['encryption', 'none']);
    if (usesVision('vless', t)) params.push(['flow', 'xtls-rprx-vision']);
    if (t.security === 'tls') params.push(['fp', 'chrome']);
    if (t.network === 'ws') params.push(['host', t.host], ['path', t.path]);
    params.push(['security', t.security]);
    if (t.security === 'tls') params.push(['sni', t.host]);
    params.push(['type', t.network]);
  }
  const qs = params.map(([k, v]) => `${k}=${q(v)}`).join('&');
  return `vless://${user.vless_uuid ?? ''}@${addr}:${node.port ?? 0}?${qs}#${q(node.name ?? '')}`;
}

function encodeShadowsocks(node: NodeRow, user: UserCreds): string {
  const ssStr = `aes-256-gcm:${user.ss_password ?? ''}@${node.address ?? ''}:${node.port ?? 0}`;
  return `ss://${b64std(ssStr)}#${queryEscape(node.name ?? '')}`;
}

function encodeTrojan(node: NodeRow, user: UserCreds): string {
  const addr = node.address ?? '';
  return `trojan://${user.trojan_password ?? ''}@${addr}:${node.port ?? 0}?security=tls&sni=${addr}#${queryEscape(node.name ?? '')}`;
}
