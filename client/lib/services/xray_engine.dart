import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

/// Parsed VLESS/VMess URI components used to build a standard Xray config.
class XrayNodeConfig {
  final String name;
  final String protocol; // vless / vmess / trojan / shadowsocks
  final String uuid;
  final String password;
  final String host;
  final int port;
  final Map<String, String> params;

  XrayNodeConfig({
    required this.name,
    required this.protocol,
    required this.uuid,
    required this.password,
    required this.host,
    required this.port,
    this.params = const {},
  });

  /// Build the `outbound` object for a standard Xray JSON config.
  Map<String, dynamic> toOutbound() {
    final security = params['security'] ?? '';
    final streamSettings = <String, dynamic>{
      'network': params['type'] ?? 'tcp',
    };

    if (params['type'] == 'ws') {
      streamSettings['wsSettings'] = {
        'path': params['path'] ?? '/',
        'headers': {
          'Host': params['host'] ?? host,
        },
      };
    }

    if (security == 'tls') {
      streamSettings['security'] = 'tls';
      streamSettings['tlsSettings'] = {
        'serverName': params['sni'] ?? params['host'] ?? host,
        'allowInsecure': false,
        'fingerprint': params['fp'] ?? 'chrome',
      };
    } else if (security == 'reality') {
      streamSettings['security'] = 'reality';
      streamSettings['realitySettings'] = {
        'serverName': params['sni'] ?? host,
        'fingerprint': params['fp'] ?? 'chrome',
        'publicKey': params['pbk'] ?? '',
        'shortId': params['sid'] ?? '',
        'spiderX': params['spx'] ?? '',
      };
    } else {
      streamSettings['security'] = 'none';
    }

    final settings = <String, dynamic>{
      'vnext': [
        {
          'address': host,
          'port': port,
          'users': [
            {
              'id': uuid,
              'encryption': 'none',
              'flow': params['flow'] ?? 'xtls-rprx-vision',
            },
          ],
        },
      ],
    };

    return {
      'tag': 'proxy-$name',
      'protocol': protocol,
      'settings': settings,
      'streamSettings': streamSettings,
    };
  }
}

/// A Dart-side wrapper around the native libXray engine.
///
/// The native side (Android `RFVpnService`, iOS/macOS PacketTunnelProvider,
/// desktop TUN helpers) owns the TUN interface file descriptor. libXray
/// (since its SetTunFd removal) receives the fd through the `xray.tun.fd`
/// entry in the config root `env` object, so the native bridge injects it
/// right before calling `runXrayFromJson`.
abstract class XrayEngine {
  /// Starts the engine with the given standard Xray JSON config.
  Future<void> start(String configJson);

  /// Stops the engine and releases the TUN interface.
  Future<void> stop();

  /// Returns whether the native engine is currently running.
  Future<bool> isRunning();

  /// Cumulative traffic counters {upload, download} in bytes.
  Future<({int upload, int download})> stats();
}

/// Default engine backed by the native MethodChannel.
class NativeXrayEngine implements XrayEngine {
  static const MethodChannel _channel = MethodChannel('uk.rfplay.client/xray');
  static const MethodChannel _vpnChannel =
      MethodChannel('uk.rfplay.client/vpn');

  @override
  Future<void> start(String configJson) async {
    try {
      if (Platform.isIOS) {
        // iOS starts a NETunnelProvider extension; the config is persisted
        // to the shared App Group by the AppDelegate handler.
        await _vpnChannel.invokeMethod('startVpn', {'config': configJson});
      } else {
        await _channel.invokeMethod('xrayStart', {'config': configJson});
      }
    } on PlatformException catch (e) {
      debugPrint('[XrayEngine] start failed: ${e.message}');
      rethrow;
    }
  }

  @override
  Future<void> stop() async {
    try {
      if (Platform.isIOS) {
        await _vpnChannel.invokeMethod('stopVpn');
      } else {
        await _channel.invokeMethod('xrayStop');
      }
    } on PlatformException catch (e) {
      debugPrint('[XrayEngine] stop failed: ${e.message}');
    }
  }

  @override
  Future<bool> isRunning() async {
    try {
      if (Platform.isIOS) {
        return await _vpnChannel.invokeMethod('vpnRunning') ?? false;
      }
      return await _channel.invokeMethod('xrayRunning') ?? false;
    } catch (_) {
      return false;
    }
  }

  @override
  Future<({int upload, int download})> stats() async {
    try {
      if (Platform.isIOS) {
        // Tunnel providers don't expose live byte counters through
        // NETunnelProvider; the packet pump reports them asynchronously.
        return (upload: 0, download: 0);
      }
      final raw = await _channel.invokeMethod('xrayStats');
      if (raw is Map) {
        return (
          upload: (raw['upload'] as num?)?.toInt() ?? 0,
          download: (raw['download'] as num?)?.toInt() ?? 0,
        );
      }
    } catch (_) {}
    return (upload: 0, download: 0);
  }
}

/// Builds a standard Xray client JSON config from a subscription node URI.
///
/// The server is standard XTLS/Xray-core (Phase 1), so a standard
/// client-side Xray JSON config works unchanged. On mobile the config uses a
/// `tun` inbound; the native bridge injects `env["xray.tun.fd"]` at runtime.
class XrayConfigBuilder {
  /// Parse a subscription URI string into structured fields.
  static XrayNodeConfig? parseUri(String uri) {
    if (!uri.contains('://')) return null;
    final idx = uri.indexOf('://');
    final protocol = uri.substring(0, idx);
    final rest = uri.substring(idx + 3);

    // Strip fragment (#name) used as a display label.
    String payload = rest;
    String name = '';
    if (payload.contains('#')) {
      final hashIdx = payload.indexOf('#');
      name = Uri.decodeComponent(payload.substring(hashIdx + 1));
      payload = payload.substring(0, hashIdx);
    }

    final params = <String, String>{};
    String authority = payload;
    if (payload.contains('?')) {
      final qIdx = payload.indexOf('?');
      final query = payload.substring(qIdx + 1);
      authority = payload.substring(0, qIdx);
      for (final pair in query.split('&')) {
        final kv = pair.split('=');
        if (kv.length == 2) params[kv[0]] = Uri.decodeComponent(kv[1]);
      }
    }

    String userInfo = '';
    String hostPort = authority;
    if (authority.contains('@')) {
      final atIdx = authority.indexOf('@');
      userInfo = authority.substring(0, atIdx);
      hostPort = authority.substring(atIdx + 1);
    }

    String host = hostPort;
    int port = 443;
    if (hostPort.contains(':')) {
      final colonIdx = hostPort.lastIndexOf(':');
      host = hostPort.substring(0, colonIdx);
      port = int.tryParse(hostPort.substring(colonIdx + 1)) ?? 443;
    }

    String uuid = '';
    String password = '';
    if (protocol == 'vless' || protocol == 'vmess') {
      uuid = userInfo.split(':').first;
    } else {
      password = userInfo.split(':').first;
    }

    // vmess uses a base64-encoded JSON payload rather than userinfo.
    if (protocol == 'vmess' && userInfo.isEmpty) {
      try {
        final decoded =
            utf8.decode(base64Url.decode(base64Url.normalize(authority)));
        final data = json.decode(decoded) as Map<String, dynamic>;
        uuid = (data['id'] as String?) ?? '';
        host = (data['add'] as String?) ?? host;
        final portVal = data['port'];
        port = (portVal is num)
            ? portVal.toInt()
            : (int.tryParse(portVal?.toString() ?? '') ?? port);
        params['type'] = (data['net'] as String?) ?? params['type'] ?? 'tcp';
        params['security'] =
            (data['tls'] as String?) ?? params['security'] ?? 'none';
        params['path'] = (data['path'] as String?) ?? params['path'] ?? '/';
        params['sni'] = (data['sni'] as String?) ?? params['sni'] ?? '';
        params['host'] = (data['host'] as String?) ?? params['host'] ?? '';
        name = name.isEmpty ? ((data['ps'] as String?) ?? '') : name;
      } catch (_) {
        return null;
      }
    }

    return XrayNodeConfig(
      name: name.isEmpty ? '$protocol-$host' : name,
      protocol: protocol,
      uuid: uuid,
      password: password,
      host: host,
      port: port,
      params: params,
    );
  }

  /// Build a mobile Xray config using a `tun` inbound. The native bridge
  /// injects `env["xray.tun.fd"]` with the VpnService file descriptor.
  static Map<String, dynamic> buildTunClientConfig(String nodeUri) {
    final node = parseUri(nodeUri);
    if (node == null) {
      throw ArgumentError('Cannot parse node URI');
    }

    final outbound = node.toOutbound();
    outbound['mux'] = {
      'enabled': true,
      'concurrency': 8,
    };

    return {
      'log': {
        'loglevel': 'warning',
      },
      'env': <String, dynamic>{},
      'inbounds': [
        {
          'tag': 'tun-in',
          'protocol': 'tun',
          'settings': {
            'fd': 0, // placeholder; replaced by native bridge with the real fd
            'stack': 'gvisor',
            'mtu': 1500,
          },
          'sniffing': {
            'enabled': true,
            'destOverride': ['http', 'tls', 'quic'],
          },
        },
      ],
      'outbounds': [
        outbound,
        {
          'tag': 'direct',
          'protocol': 'freedom',
        },
      ],
      'routing': {
        'domainStrategy': 'IPIfNonMatch',
        'rules': [
          {
            'type': 'field',
            'inboundTag': ['tun-in'],
            'outboundTag': 'proxy-${node.name}',
          },
        ],
      },
    };
  }

  /// Build a desktop config that listens on a local SOCKS/HTTP port; the OS
  /// routes traffic to it via system proxy or the user's manual setup.
  static Map<String, dynamic> buildLocalPortConfig(String nodeUri) {
    final node = parseUri(nodeUri);
    if (node == null) {
      throw ArgumentError('Cannot parse node URI');
    }

    final outbound = node.toOutbound();
    outbound['mux'] = {
      'enabled': true,
      'concurrency': 8,
    };

    return {
      'log': {
        'loglevel': 'warning',
      },
      'inbounds': [
        {
          'tag': 'socks-in',
          'listen': '127.0.0.1',
          'port': 10808,
          'protocol': 'socks',
          'settings': {'udp': true},
        },
        {
          'tag': 'http-in',
          'listen': '127.0.0.1',
          'port': 10809,
          'protocol': 'http',
        },
      ],
      'outbounds': [
        outbound,
        {
          'tag': 'direct',
          'protocol': 'freedom',
        },
      ],
      'routing': {
        'domainStrategy': 'IPIfNonMatch',
        'rules': [
          {
            'type': 'field',
            'inboundTag': ['socks-in', 'http-in'],
            'outboundTag': 'proxy-${node.name}',
          },
        ],
      },
    };
  }
}
