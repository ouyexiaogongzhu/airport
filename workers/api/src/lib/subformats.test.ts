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
  it('vmess：鍵序排序 JSON + 標準 base64', () => {
    expect(encodeNodeToURI(vmessNode, user)).toBe(
      'vmess://eyJhZGQiOiJoay5leGFtcGxlLmNvbSIsImFpZCI6MCwiaWQiOiIxMTExMTExMS0yMjIyLTMzMzMtNDQ0NC01NTU1NTU1NTU1NTUiLCJuZXQiOiJ3cyIsInBzIjoiSEstMDEiLCJwb3J0Ijo0NDMsInR5cGUiOiJub25lIiwidiI6IjIifQ==',
    );
  });

  it('vless：query 參數按鍵序 flow,fp,pbk,security,sid,sni,type', () => {
    expect(encodeNodeToURI(vlessNode, user)).toBe(
      'vless://11111111-2222-3333-4444-555555555555@us.example.com:8443?flow=xtls-rprx-vision&fp=chrome&pbk=PBK&security=reality&sid=abcd1234&sni=us.example.com&type=tcp#US-01',
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
    expect(out?.body).toBe(
      'dm1lc3M6Ly9leUpoWkdRaU9pSm9heTVsZUdGdGNHeGxMbU52YlNJc0ltRnBaQ0k2TUN3aWFXUWlPaUl4TVRFeE1URXhNUzB5TWpJeUxUTXpNek10TkRRME5DMDFOVFUxTlRVMU5UVTFOVFVpTENKdVpYUWlPaUozY3lJc0luQnpJam9pU0VzdE1ERWlMQ0p3YjNKMElqbzBORE1zSW5SNWNHVWlPaUp1YjI1bElpd2lkaUk2SWpJaWZRPT0Kdmxlc3M6Ly8xMTExMTExMS0yMjIyLTMzMzMtNDQ0NC01NTU1NTU1NTU1NTVAdXMuZXhhbXBsZS5jb206ODQ0Mz9mbG93PXh0bHMtcnByeC12aXNpb24mZnA9Y2hyb21lJnBiaz1QQksmc2VjdXJpdHk9cmVhbGl0eSZzaWQ9YWJjZDEyMzQmc25pPXVzLmV4YW1wbGUuY29tJnR5cGU9dGNwI1VTLTAx',
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
    // uuid = %08x-%04x-%04x-%04x-%012x of (1,0,0,0,100)
    expect(out.body).toContain(
      '  - name: "HK-01"\n    type: vmess\n    server: hk.example.com\n    port: 443\n    uuid: 00000001-0000-0000-0000-000000000064\n    alterId: 0\n    cipher: auto\n    tls: true\n    network: ws\n    ws-path: /ws\n    ws-headers:\n      Host: hk.example.com\n\n',
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
