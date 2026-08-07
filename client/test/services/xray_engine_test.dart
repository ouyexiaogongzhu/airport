import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:rfplay_client/services/xray_engine.dart';

void main() {
  group('XrayConfigBuilder.parseUri', () {
    test('parses a standard VLESS+REALITY URI', () {
      const uri =
          'vless://6b0f1c2e-9d43-4f7a-9b1c-abcdef123456@203.0.113.10:443'
          '?type=tcp&security=reality&sni=cdn.example.com&fp=chrome'
          '&pbk=REALITYPUBKEY&sid=abcdef01&flow=xtls-rprx-vision#MyNode';
      final node = XrayConfigBuilder.parseUri(uri);

      expect(node, isNotNull);
      expect(node!.protocol, 'vless');
      expect(node.uuid, '6b0f1c2e-9d43-4f7a-9b1c-abcdef123456');
      expect(node.host, '203.0.113.10');
      expect(node.port, 443);
      expect(node.name, 'MyNode');
      expect(node.params['security'], 'reality');
      expect(node.params['sni'], 'cdn.example.com');
      expect(node.params['flow'], 'xtls-rprx-vision');
    });

    test('parses a VLESS+WS URI', () {
      const uri =
          'vless://6b0f1c2e-9d43-4f7a-9b1c-abcdef123456@example.com:443'
          '?type=ws&security=tls&sni=example.com&path=%2Fapi%2Fws'
          '&host=example.com#WSNode';
      final node = XrayConfigBuilder.parseUri(uri);

      expect(node, isNotNull);
      expect(node!.protocol, 'vless');
      expect(node.params['type'], 'ws');
      expect(node.params['path'], '/api/ws');
      expect(node.params['security'], 'tls');
    });

    test('returns null for a non-URI string', () {
      expect(XrayConfigBuilder.parseUri('plain text'), isNull);
    });

    test('parses a vmess base64-encoded payload', () {
      final payload = jsonEncode({
        'v': '2',
        'ps': 'VmessNode',
        'add': '198.51.100.7',
        'port': '8443',
        'id': '9c14c3e2-8a5d-4c1a-99a0-112233445566',
        'net': 'ws',
        'tls': 'tls',
        'path': '/vmess',
        'sni': 'vmess.example.com',
      });
      final b64 = base64UrlEncode(utf8.encode(payload));
      final uri = 'vmess://$b64';

      final node = XrayConfigBuilder.parseUri(uri);

      expect(node, isNotNull);
      expect(node!.protocol, 'vmess');
      expect(node.host, '198.51.100.7');
      expect(node.port, 8443);
      expect(node.uuid, '9c14c3e2-8a5d-4c1a-99a0-112233445566');
      expect(node.name, 'VmessNode');
    });
  });

  group('XrayConfigBuilder.buildTunClientConfig', () {
    test('builds a tun inbound config with the proxy outbound', () {
      const uri =
          'vless://6b0f1c2e-9d43-4f7a-9b1c-abcdef123456@203.0.113.10:443'
          '?type=tcp&security=reality&sni=cdn.example.com&fp=chrome'
          '&pbk=REALITYPUBKEY&sid=abcdef01&flow=xtls-rprx-vision#MyNode';
      final config = XrayConfigBuilder.buildTunClientConfig(uri);

      expect(config['inbounds'], isA<List<dynamic>>());
      final inbounds = config['inbounds'] as List<dynamic>;
      expect(inbounds.first['protocol'], 'tun');

      final outbounds = config['outbounds'] as List<dynamic>;
      final proxy = outbounds.first as Map<String, dynamic>;
      expect(proxy['protocol'], 'vless');
      expect(proxy['tag'], 'proxy-MyNode');

      final settings = proxy['settings'] as Map<String, dynamic>;
      final vnext = (settings['vnext'] as List<dynamic>).first
          as Map<String, dynamic>;
      expect(vnext['address'], '203.0.113.10');

      final stream = proxy['streamSettings'] as Map<String, dynamic>;
      expect(stream['security'], 'reality');
      final reality = stream['realitySettings'] as Map<String, dynamic>;
      expect(reality['serverName'], 'cdn.example.com');
      expect(reality['publicKey'], 'REALITYPUBKEY');
    });
  });

  group('XrayConfigBuilder.buildLocalPortConfig', () {
    test('builds socks/http inbounds for desktop', () {
      const uri =
          'vless://6b0f1c2e-9d43-4f7a-9b1c-abcdef123456@203.0.113.10:443'
          '?type=ws&security=tls&sni=example.com&path=%2Fapi%2Fws'
          '&host=example.com#WSNode';
      final config = XrayConfigBuilder.buildLocalPortConfig(uri);

      final inbounds = config['inbounds'] as List<dynamic>;
      expect(inbounds.map((e) => e['protocol']), contains('socks'));
      expect(inbounds.map((e) => e['protocol']), contains('http'));

      final outbounds = config['outbounds'] as List<dynamic>;
      final proxy = outbounds.first as Map<String, dynamic>;
      final stream = proxy['streamSettings'] as Map<String, dynamic>;
      expect(stream['security'], 'tls');
    });
  });
}
