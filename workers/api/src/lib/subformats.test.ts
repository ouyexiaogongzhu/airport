// 純函數契約測試 — 節點為 Cloudflare Tunnel 形態：ws|xhttp + tls + 443，host/sni = 節點域名
import { describe, expect, it } from 'vitest';
import { buildClash, buildSingbox, buildV2ray, goJSON } from './subformats';
import { encodeNodeToURI, queryEscape } from './xrayuri';
import type { NodeRow, UserCreds } from './xrayuri';

const user: UserCreds = {
  id: 1,
  vless_uuid: '11111111-2222-3333-4444-555555555555',
};

const vmessNode: NodeRow = {
  name: 'HK-01',
  address: 'hk.example.com',
  protocol: 'vmess',
  ws_path: '/vcheck/',
};

const vlessNode: NodeRow = {
  name: 'SG-CF',
  address: 'sg.example.com',
  protocol: 'vless',
  ws_path: '/vcheck/',
};

const vlessXhttp: NodeRow = {
  name: 'w2',
  address: 'w2.example.com',
  protocol: 'vless',
  ws_path: '/rfhttp/',
  network: 'xhttp',
};

const ssNode: NodeRow = {
  name: 'JP-01',
  address: 'jp.example.com',
  protocol: 'shadowsocks',
};

const trojanNode: NodeRow = {
  name: 'TW 01',
  address: 'tw.example.com',
  protocol: 'trojan',
};

describe('queryEscape（Go url.QueryEscape 語義）', () => {
  it('空格編碼為 +，保留 unreserved', () => {
    expect(queryEscape('TW 01')).toBe('TW+01');
    expect(queryEscape('a-b_c.d~e')).toBe('a-b_c.d~e');
    expect(queryEscape('a&b<c')).toBe('a%26b%3Cc');
  });
});

describe('encodeNodeToURI', () => {
  it('vmess：ws + tls + 443，host/sni 為節點域名', () => {
    const uri = encodeNodeToURI(vmessNode, user);
    expect(uri.startsWith('vmess://')).toBe(true);
    expect(atob(uri.slice('vmess://'.length))).toBe(
      '{"add":"hk.example.com","aid":0,"host":"hk.example.com","id":"11111111-2222-3333-4444-555555555555","net":"ws","path":"/vcheck/","port":443,"ps":"HK-01","sni":"hk.example.com","tls":"tls","type":"none","v":"2"}',
    );
  });

  it('vless：ws + tls + 443，無 flow', () => {
    expect(encodeNodeToURI(vlessNode, user)).toBe(
      'vless://11111111-2222-3333-4444-555555555555@sg.example.com:443?encryption=none&fp=chrome&host=sg.example.com&path=%2Fvcheck%2F&security=tls&sni=sg.example.com&type=ws#SG-CF',
    );
  });

  it('vless xhttp：type=xhttp + mode=packet-up + alpn=h2', () => {
    expect(encodeNodeToURI(vlessXhttp, user)).toBe(
      'vless://11111111-2222-3333-4444-555555555555@w2.example.com:443?alpn=h2&encryption=none&fp=chrome&host=w2.example.com&mode=packet-up&path=%2Frfhttp%2F&security=tls&sni=w2.example.com&type=xhttp#w2',
    );
  });

  it('vmess xhttp：net=xhttp', () => {
    const uri = encodeNodeToURI({ ...vmessNode, network: 'xhttp' }, user);
    expect(atob(uri.slice('vmess://'.length))).toContain('"net":"xhttp"');
  });

  it('ws_path 為空時默認 /', () => {
    expect(encodeNodeToURI({ ...vlessNode, ws_path: null }, user)).toContain('&path=%2F&');
  });

  it('已下線的 shadowsocks / trojan 回空字串', () => {
    expect(encodeNodeToURI(ssNode, user)).toBe('');
    expect(encodeNodeToURI(trojanNode, user)).toBe('');
  });

  it('未知協議回空字串', () => {
    expect(encodeNodeToURI({ ...vmessNode, protocol: 'wireguard' }, user)).toBe('');
  });
});

describe('buildV2ray', () => {
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

describe('buildClash', () => {
  it('vmess 節點 YAML 逐字一致；已下線協議不進 proxies 與 proxy-groups', () => {
    const out = buildClash(user, [vmessNode, ssNode]);
    expect(out.ct).toBe('text/yaml; charset=utf-8');
    expect(out.body).toContain(
      '  - name: "HK-01"\n    type: vmess\n    server: hk.example.com\n    port: 443\n    uuid: 11111111-2222-3333-4444-555555555555\n    alterId: 0\n    cipher: auto\n    tls: true\n    servername: hk.example.com\n    network: ws\n    ws-opts:\n      path: "/vcheck/"\n      headers:\n        Host: hk.example.com\n\n',
    );
    expect(out.body).not.toContain('JP-01');
    expect(out.body.endsWith('rules:\n  - GEOIP,CN,DIRECT\n  - MATCH,Proxy\n')).toBe(true);
    expect(out.body).toContain(
      'proxy-groups:\n  - name: Proxy\n    type: select\n    proxies:\n      - Auto\n      - "HK-01"\n      - DIRECT\n  - name: Auto\n    type: url-test\n    proxies:\n      - "HK-01"\n    url:',
    );
  });

  it('vless 節點：無 flow / reality-opts', () => {
    const out = buildClash(user, [vlessNode]);
    expect(out.body).toContain(
      '  - name: "SG-CF"\n    type: vless\n    server: sg.example.com\n    port: 443\n    uuid: 11111111-2222-3333-4444-555555555555\n    tls: true\n    servername: sg.example.com\n    network: ws\n',
    );
    expect(out.body).not.toContain('flow');
    expect(out.body).not.toContain('reality');
  });

  it('vless xhttp：xhttp-opts + alpn h2 + packet-up', () => {
    const out = buildClash(user, [vlessXhttp]);
    expect(out.body).toContain(
      '  - name: "w2"\n    type: vless\n    server: w2.example.com\n    port: 443\n    uuid: 11111111-2222-3333-4444-555555555555\n    tls: true\n    servername: w2.example.com\n    network: xhttp\n    alpn:\n      - h2\n    client-fingerprint: chrome\n    xhttp-opts:\n      path: "/rfhttp/"\n      host: w2.example.com\n      mode: packet-up\n\n',
    );
    expect(out.body).not.toContain('ws-opts');
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
