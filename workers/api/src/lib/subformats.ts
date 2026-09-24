// 對齊 manager/internal/handler/subscription.go — 三種訂閱格式（逐字節契約）
// Go json.Marshal 對 map 按鍵排序且 HTML 轉義 <>&；物件以插入序模擬，字串後處理轉義。
import { CLIENT_PORT, XHTTP_CLIENT_MODE, b64std, encodeNodeToURI, nodeTransport, type NodeRow, type UserCreds } from './xrayuri';

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

// proxy-groups 引用未輸出的節點名時 Clash 會拒絕整份配置，所以先過濾
const CLASH_PROTOCOLS = new Set(['vmess', 'vless']);

export function buildClash(user: UserCreds, allNodes: NodeRow[]): FormatOutput {
  const nodes = allNodes.filter((n) => CLASH_PROTOCOLS.has(n.protocol ?? ''));
  const sb: string[] = [];
  sb.push('port: 7890\n');
  sb.push('socks-port: 7891\n');
  sb.push('mode: Rule\n');
  sb.push('log-level: info\n\n');

  sb.push('proxies:\n');
  for (const node of nodes) {
    const name = node.name ?? '';
    const address = node.address ?? '';
    const t = nodeTransport(node);
    sb.push(`  - name: "${name}"\n`);
    sb.push(`    type: ${node.protocol}\n`);
    sb.push(`    server: ${address}\n`);
    sb.push(`    port: ${CLIENT_PORT}\n`);
    sb.push(`    uuid: ${user.vless_uuid ?? ''}\n`);
    if (node.protocol === 'vmess') {
      sb.push('    alterId: 0\n');
      sb.push('    cipher: auto\n');
    }
    sb.push('    tls: true\n');
    sb.push(`    servername: ${t.host}\n`);
    sb.push('    network: xhttp\n');
    sb.push('    alpn:\n      - h2\n');
    sb.push('    client-fingerprint: chrome\n');
    sb.push(`    xhttp-opts:\n      path: "${t.path}"\n      host: ${t.host}\n      mode: ${XHTTP_CLIENT_MODE}\n`);
    sb.push('\n');
  }

  sb.push('proxy-groups:\n');
  sb.push('  - name: Proxy\n');
  sb.push('    type: select\n');
  sb.push('    proxies:\n');
  sb.push('      - Auto\n');
  for (const node of nodes) {
    sb.push(`      - "${node.name ?? ''}"\n`);
  }
  sb.push('      - DIRECT\n');
  sb.push('  - name: Auto\n');
  sb.push('    type: url-test\n');
  sb.push('    proxies:\n');
  for (const node of nodes) {
    sb.push(`      - "${node.name ?? ''}"\n`);
  }
  sb.push('    url: http://www.gstatic.com/generate_204\n');
  sb.push('    interval: 300\n');
  sb.push('    tolerance: 50\n\n');

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
