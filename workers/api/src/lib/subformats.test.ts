// 純函數契約測試 — 期望值由 Go 邏輯手工推導（base64 以標準編碼核算）
// 注意：本檔不在 tsc --noEmit 範圍驗收內（vitest 由主線統一跑）
import { describe, expect, it } from 'vitest';
import { buildClash, buildSingbox, buildV2ray, goJSON } from './subformats';
import { encodeNodeToURI, queryEscape } from './xrayuri';
import type { NodeRow, UserCreds } from './xrayuri';

const user: UserCreds = {
  id: 1,
  vless_uuid: '11111111-2222-3333-4444-555555555555',
  ss_password: 'sptest',
  trojan_password: 'tjpass',
};

const vmessNode: NodeRow = {
  name: 'HK-01',
  address: 'hk.example.com',
  port: 443,
  protocol: 'vmess',
  reality_public_key: null,
  reality_short_id: null,
  network: 'ws',
  security: 'tls',
  ws_path: '/vcheck/',
  server_name: null,
};

const vlessWsNode: NodeRow = {
  name: 'SG-CF',
  address: 'sg.example.com',
  port: 443,
  protocol: 'vless',
  reality_public_key: null,
  reality_short_id: null,
  network: 'ws',
  security: 'tls',
  ws_path: '/vcheck/',
  server_name: 'cdn.example.com',
};

const vlessNode: NodeRow = {
  name: 'US-01',
  address: 'us.example.com',
  port: 8443,
  protocol: 'vless',
  reality_public_key: 'PBK',
  reality_short_id: 'abcd1234',
};

const ssNode: NodeRow = {
  name: 'JP-01',
  address: 'jp.example.com',
  port: 8388,
  protocol: 'shadowsocks',
  reality_public_key: null,
  reality_short_id: null,
};

const trojanNode: NodeRow = {
  name: 'TW 01', // 帶空格：驗證 Go QueryEscape 的 '+' 行為
  address: 'tw.example.com',
  port: 443,
  protocol: 'trojan',
  reality_public_key: null,
  reality_short_id: null,
};

describe('queryEscape（Go url.QueryEscape 語義）', () => {
  it('空格編碼為 +，保留 unreserved', () => {
    expect(queryEscape('TW 01')).toBe('TW+01');
    expect(queryEscape('a-b_c.d~e')).toBe('a-b_c.d~e');
    expect(queryEscape('a&b<c')).toBe('a%26b%3Cc');
  });
});

describe('encodeNodeToURI（對齊 links.go）', () => {
  it('vmess ws+tls：帶 host/path/sni/tls，host 默認為 address', () => {
    const uri = encodeNodeToURI(vmessNode, user);
    expect(uri.startsWith('vmess://')).toBe(true);
    expect(atob(uri.slice('vmess://'.length))).toBe(
      '{"add":"hk.example.com","aid":0,"host":"hk.example.com","id":"11111111-2222-3333-4444-555555555555","net":"ws","path":"/vcheck/","port":443,"ps":"HK-01","sni":"hk.example.com","tls":"tls","type":"none","v":"2"}',
    );
  });

  it('vless reality：舊節點只填公鑰、security 未設也按 Reality 生成', () => {
    expect(encodeNodeToURI(vlessNode, user)).toBe(
      'vless://11111111-2222-3333-4444-555555555555@us.example.com:8443?flow=xtls-rprx-vision&fp=chrome&pbk=PBK&security=reality&sid=abcd1234&sni=us.example.com&type=tcp#US-01',
    );
  });

  it('vless reality：sni 用 server_name（偽裝域名）', () => {
    expect(encodeNodeToURI({ ...vlessNode, security: 'reality', server_name: 'www.microsoft.com' }, user)).toContain(
      '&sni=www.microsoft.com&type=tcp',
    );
  });

  it('vless ws+tls（Cloudflare）：無 flow，帶 host/path/sni', () => {
    expect(encodeNodeToURI(vlessWsNode, user)).toBe(
      'vless://11111111-2222-3333-4444-555555555555@sg.example.com:443?encryption=none&fp=chrome&host=cdn.example.com&path=%2Fvcheck%2F&security=tls&sni=cdn.example.com&type=ws#SG-CF',
    );
  });

  it('shadowsocks：aes-256-gcm:pass@addr:port 整體 base64', () => {
    expect(encodeNodeToURI(ssNode, user)).toBe(
      'ss://YWVzLTI1Ni1nY206c3B0ZXN0QGpwLmV4YW1wbGUuY29tOjgzODg=#JP-01',
    );
  });

  it('trojan：密碼不轉義，名稱 QueryEscape', () => {
    expect(encodeNodeToURI(trojanNode, user)).toBe(
      'trojan://tjpass@tw.example.com:443?security=tls&sni=tw.example.com#TW+01',
    );
  });

  it('未知協議回空字串', () => {
    expect(encodeNodeToURI({ ...vmessNode, protocol: 'wireguard' }, user)).toBe('');
  });
});

describe('buildV2ray（對齊 handleV2rayFormat）', () => {
  it('URI 以 \\n 相接後整體 base64', () => {
    const out = buildV2ray(user, [vmessNode, vlessNode]);
    expect(out?.ct).toBe('text/plain; charset=utf-8');
    expect(atob(out?.body ?? '')).toBe(
      `${encodeNodeToURI(vmessNode, user)}\n${encodeNodeToURI(vlessNode, user)}`,
    );
  });

  it('全節點不可編碼時回 null（上層 204）', () => {
    expect(buildV2ray(user, [{ ...vmessNode, protocol: 'http' }])).toBeNull();
  });
});

describe('buildClash（對齊 handleClashFormat）', () => {
  it('vmess+ss 節點的 YAML 片段逐字一致', () => {
    const out = buildClash(user, [vmessNode, ssNode]);
    expect(out.ct).toBe('text/yaml; charset=utf-8');
    expect(out.body).toContain(
      '  - name: "HK-01"\n    type: vmess\n    server: hk.example.com\n    port: 443\n    uuid: 11111111-2222-3333-4444-555555555555\n    alterId: 0\n    cipher: auto\n    tls: true\n    servername: hk.example.com\n    network: ws\n    ws-opts:\n      path: "/vcheck/"\n      headers:\n        Host: hk.example.com\n\n',
    );
    expect(out.body).toContain(
      '    cipher: aes-256-gcm\n    password: "rf-1-pass"\n\n',
    );
    expect(out.body.endsWith('rules:\n  - GEOIP,CN,DIRECT\n  - MATCH,Proxy\n')).toBe(true);
    expect(out.body).toContain('proxy-groups:\n  - name: Proxy\n    type: url-test\n    proxies:\n      - HK-01\n      - JP-01\n');
  });
});

describe('buildSingbox（對齊 handleSingboxFormat）', () => {
  it('Go map 鍵序（outbounds,route.final,rules）+ 結構鍵序 tag,protocol', () => {
    const out = buildSingbox(user, [vmessNode]);
    expect(out.ct).toBe('application/json; charset=utf-8');
    expect(out.body).toBe(
      '{"outbounds":[{"tag":"HK-01","protocol":"vmess"}],"route":{"final":"select","rules":[{"geoip":"cn","outbound":"direct"},{"geosite":"cn","outbound":"direct"}]}}',
    );
  });
});

describe('goJSON（Go json.Marshal HTML 轉義）', () => {
  it('轉義 < > &', () => {
    expect(goJSON({ a: '<b>&' })).toBe('{"a":"\\u003cb\\u003e\\u0026"}');
  });
});
