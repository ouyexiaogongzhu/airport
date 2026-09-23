// 節點 Xray 服務端配置（唯一形態：cloudflared Tunnel 回源）
// inbound 只監聽 127.0.0.1，ws + security none；TLS 由 Cloudflare 邊緣終結，不需要證書。
// 每用戶 client 帶 email "u{id}"，daemon 經 StatsService 讀 user>>>u{id}>>>traffic>>>uplink/downlink。
// 已知限制：Xray 不支持按用戶限速（policy 只有超時與統計開關），speed_limit_bps / rate_limit_bps 暫不下發。
import { SERVICEABLE_SQL } from './entitlement';

export type NodeConfigRow = {
  id: number;
  port: number;
  protocol: string;
  ws_path: string | null;
  status?: string | null;
};

// StatsService 監聽端口（僅 127.0.0.1）；與節點端口衝突時換一個
const API_PORT = 10085;

// 不依賴 geoip.dat
const PRIVATE_CIDRS = [
  '0.0.0.0/8', '10.0.0.0/8', '100.64.0.0/10', '127.0.0.0/8', '169.254.0.0/16', '172.16.0.0/12', '192.168.0.0/16',
  '::1/128', 'fc00::/7', 'fe80::/10',
];

// 配置結構變化時遞增，強制所有節點重新應用
const SCHEMA = 'a2-1';

export function nodeApiPort(nodePort: number): number {
  return nodePort === API_PORT ? API_PORT + 1 : API_PORT;
}

export function nodeWsPath(node: Pick<NodeConfigRow, 'ws_path'>): string {
  return node.ws_path || '/';
}

// FNV-1a 64 截成 53 位：JSON 往返（daemon 以 float64 解析）不丟精度
export function configVersion(input: string): number {
  let h = 14695981039346656037n;
  for (const b of new TextEncoder().encode(input)) {
    h ^= BigInt(b);
    h = (h * 1099511628211n) & 0xffffffffffffffffn;
  }
  return Number(h & ((1n << 53n) - 1n));
}

// 節點非 active 時下發空用戶列表：daemon 應用後即斷開所有連接
export async function buildNodeXrayConfig(db: D1Database, node: NodeConfigRow, now: number): Promise<Record<string, unknown>> {
  const users =
    node.status === undefined || node.status === 'active'
      ? (
          await db
            .prepare(`SELECT id, vless_uuid FROM users WHERE ${SERVICEABLE_SQL} ORDER BY id`)
            .bind(now)
            .all<{ id: number; vless_uuid: string | null }>()
        ).results
      : [];

  const protocol = node.protocol;
  const path = nodeWsPath(node);
  const apiPort = nodeApiPort(node.port);
  const inTag = `in-${protocol}`;

  const clients: Record<string, unknown>[] = [];
  const userIDs: number[] = [];
  const parts: string[] = [SCHEMA, protocol, String(node.port), path, String(apiPort)];
  for (const u of users) {
    // 無 UUID 的用戶訂閱端也無法生成鏈接，跳過
    if (!u.vless_uuid) continue;
    clients.push({ id: u.vless_uuid, email: `u${u.id}`, level: 0 });
    userIDs.push(u.id);
    parts.push(`${u.id}:${u.vless_uuid}`);
  }

  const settings: Record<string, unknown> = protocol === 'vless' ? { clients, decryption: 'none' } : { clients };

  return {
    log: { loglevel: 'warning', access: '/var/log/xray/access.log', error: '/var/log/xray/error.log' },
    api: { tag: 'api', services: ['StatsService'] },
    stats: {},
    policy: {
      levels: {
        '0': { handshake: 4, connIdle: 300, uplinkOnly: 1, downlinkOnly: 1, statsUserUplink: true, statsUserDownlink: true },
      },
      system: { statsInboundUplink: true, statsInboundDownlink: true },
    },
    inbounds: [
      {
        tag: inTag,
        listen: '127.0.0.1',
        port: node.port,
        protocol,
        settings,
        streamSettings: { network: 'ws', security: 'none', wsSettings: { path } },
      },
      { tag: 'api', listen: '127.0.0.1', port: apiPort, protocol: 'dokodemo-door', settings: { address: '127.0.0.1' } },
    ],
    outbounds: [
      { protocol: 'freedom', tag: 'direct' },
      { protocol: 'blackhole', tag: 'block' },
    ],
    // 禁止經代理訪問節點本機與內網（否則用戶可連 127.0.0.1 的 StatsService 重置流量計數）；
    // 不加 inboundTag→direct 規則，讓域名目標走 IPIfNonMatch 解析後再匹配 IP 規則，默認出站為 direct
    routing: {
      domainStrategy: 'IPIfNonMatch',
      rules: [
        { type: 'field', inboundTag: ['api'], outboundTag: 'api' },
        { type: 'field', ip: PRIVATE_CIDRS, outboundTag: 'block' },
      ],
    },
    _meta: { node_id: node.id, user_ids: userIDs, version: configVersion(parts.join('|')), api_port: apiPort },
  };
}
