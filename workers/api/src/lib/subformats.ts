// 對齊 manager/internal/handler/subscription.go — 三種訂閱格式（逐字節契約）
// Go json.Marshal 對 map 按鍵排序且 HTML 轉義 <>&；物件以插入序模擬，字串後處理轉義。
import { b64std, encodeNodeToURI, type NodeRow, type UserCreds } from './xrayuri';

export type FormatKind = 'v2ray' | 'clash' | 'singbox';

export type FormatOutput = { ct: string; body: string };

// Go json.Marshal 的 HTML 轉義（compact JSON 已由 JSON.stringify 給出）
export function goJSON(v: unknown): string {
  return JSON.stringify(v)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026');
}

// v2ray：URI 以 \n 相接後整體 base64；無可用 URI 回 null（上層回 204）
export function buildV2ray(user: UserCreds, nodes: NodeRow[]): FormatOutput | null {
  const lines: string[] = [];
  for (const node of nodes) {
    const uri = encodeNodeToURI(node, user);
    if (uri !== '') lines.push(uri);
  }
  if (lines.length === 0) return null;
  return { ct: 'text/plain; charset=utf-8', body: b64std(lines.join('\n')) };
}

function hexPad(n: number, width: number): string {
  return n.toString(16).padStart(width, '0');
}

// Go: fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", user.ID, 0, 0, 0, user.ID*100)
function clashUUID(user: UserCreds): string {
  return `${hexPad(user.id, 8)}-0000-0000-0000-${hexPad(user.id * 100, 12)}`;
}

export function buildClash(user: UserCreds, nodes: NodeRow[]): FormatOutput {
  const sb: string[] = [];
  sb.push('port: 7890\n');
  sb.push('socks-port: 7891\n');
  sb.push('mode: Rule\n');
  sb.push('log-level: info\n\n');

  sb.push('proxies:\n');
  for (const node of nodes) {
    const name = node.name ?? '';
    const address = node.address ?? '';
    const port = node.port ?? 0;
    switch (node.protocol) {
      case 'vmess':
        sb.push(`  - name: "${name}"\n`);
        sb.push('    type: vmess\n');
        sb.push(`    server: ${address}\n`);
        sb.push(`    port: ${port}\n`);
        sb.push(`    uuid: ${clashUUID(user)}\n`);
        sb.push('    alterId: 0\n');
        sb.push('    cipher: auto\n');
        sb.push('    tls: true\n');
        sb.push('    network: ws\n');
        sb.push('    ws-path: /ws\n');
        sb.push(`    ws-headers:\n      Host: ${address}\n\n`);
        break;
      case 'vless':
        sb.push(`  - name: "${name}"\n`);
        sb.push('    type: vless\n');
        sb.push(`    server: ${address}\n`);
        sb.push(`    port: ${port}\n`);
        sb.push(`    uuid: ${clashUUID(user)}\n`);
        sb.push('    flow: xtls-rprx-vision\n');
        sb.push('    tls: true\n');
        sb.push('    network: tcp\n\n');
        break;
      case 'shadowsocks':
        sb.push(`  - name: "${name}"\n`);
        sb.push('    type: ss\n');
        sb.push(`    server: ${address}\n`);
        sb.push(`    port: ${port}\n`);
        sb.push('    cipher: aes-256-gcm\n');
        sb.push(`    password: "rf-${user.id}-pass"\n\n`);
        break;
      case 'trojan':
        sb.push(`  - name: "${name}"\n`);
        sb.push('    type: trojan\n');
        sb.push(`    server: ${address}\n`);
        sb.push(`    port: ${port}\n`);
        sb.push(`    password: "rf-${user.id}-pass"\n`);
        sb.push('    udp: true\n\n');
        break;
    }
  }

  sb.push('proxy-groups:\n');
  sb.push('  - name: Proxy\n');
  sb.push('    type: url-test\n');
  sb.push('    proxies:\n');
  for (const node of nodes) {
    sb.push(`      - ${node.name ?? ''}\n`);
  }
  sb.push('    url: http://www.gstatic.com/generate_204\n');
  sb.push('    interval: 300\n\n');

  sb.push('rules:\n');
  sb.push('  - GEOIP,CN,DIRECT\n');
  sb.push('  - MATCH,Proxy\n');

  return { ct: 'text/yaml; charset=utf-8', body: sb.join('') };
}

export function buildSingbox(_user: UserCreds, nodes: NodeRow[]): FormatOutput {
  const outbounds = nodes.map((node) => ({
    tag: node.name ?? '',
    protocol: node.protocol ?? '',
  }));
  const config = {
    outbounds,
    route: {
      final: 'select',
      rules: [
        { geoip: 'cn', outbound: 'direct' },
        { geosite: 'cn', outbound: 'direct' },
      ],
    },
  };
  return { ct: 'application/json; charset=utf-8', body: goJSON(config) };
}

export function buildFormat(format: FormatKind, user: UserCreds, nodes: NodeRow[]): FormatOutput | null {
  switch (format) {
    case 'v2ray':
      return buildV2ray(user, nodes);
    case 'clash':
      return buildClash(user, nodes);
    case 'singbox':
      return buildSingbox(user, nodes);
  }
}
