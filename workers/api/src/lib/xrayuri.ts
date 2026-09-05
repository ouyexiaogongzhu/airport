// 對齊 manager/internal/handler/links.go — vless/vmess/ss/trojan URI 生成（逐字節契約）

export type NodeRow = {
  name: string | null;
  address: string | null;
  port: number | null;
  protocol: string | null;
  reality_public_key: string | null;
  reality_short_id: string | null;
};

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

// Go json.Marshal(map[string]any) 按鍵排序輸出：add,aid,id,net,ps,port,type,v
function encodeVmess(node: NodeRow, user: UserCreds): string {
  const data = JSON.stringify({
    add: node.address ?? '',
    aid: 0,
    id: user.vless_uuid ?? '',
    net: 'ws',
    ps: node.name ?? '',
    port: node.port ?? 0,
    type: 'none',
    v: '2',
  });
  return 'vmess://' + b64std(data);
}

// Go url.Values.Encode() 按鍵排序：flow,fp,pbk,security,sid,sni,type
function encodeVless(node: NodeRow, user: UserCreds): string {
  const addr = node.address ?? '';
  const q = queryEscape;
  const params =
    `flow=xtls-rprx-vision` +
    `&fp=chrome` +
    `&pbk=${q(node.reality_public_key ?? '')}` +
    `&security=reality` +
    `&sid=${q(node.reality_short_id ?? '')}` +
    `&sni=${q(addr)}` +
    `&type=tcp`;
  return `vless://${user.vless_uuid ?? ''}@${addr}:${node.port ?? 0}?${params}#${q(node.name ?? '')}`;
}

function encodeShadowsocks(node: NodeRow, user: UserCreds): string {
  const ssStr = `aes-256-gcm:${user.ss_password ?? ''}@${node.address ?? ''}:${node.port ?? 0}`;
  return `ss://${b64std(ssStr)}#${queryEscape(node.name ?? '')}`;
}

function encodeTrojan(node: NodeRow, user: UserCreds): string {
  const addr = node.address ?? '';
  return `trojan://${user.trojan_password ?? ''}@${addr}:${node.port ?? 0}?security=tls&sni=${addr}#${queryEscape(node.name ?? '')}`;
}
