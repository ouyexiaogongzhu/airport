import 'dart:async';
import 'dart:io';
import 'dart:math';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/subscription.dart';

/// Connection mode for VPN
enum VpnMode {
  /// Use built-in Android VpnService (MethodChannel)
  builtin,

  /// Open external VPN app (v2rayNG, Clash, etc.)
  external,
}

/// VPN connection state
enum VpnState {
  disconnected,
  connecting,
  connected,
  disconnecting,
  error,
}

/// Node latency measurement
class NodeLatency {
  final String nodeName;
  final int latencyMs; // -1 = timeout/error
  final DateTime measuredAt;

  NodeLatency({
    required this.nodeName,
    required this.latencyMs,
    required this.measuredAt,
  });

  bool get isReachable => latencyMs >= 0;
  String get label => isReachable ? '${latencyMs}ms' : '超时';
}

/// Real VPN service with Android VpnService + external app support
class VpnService extends ChangeNotifier {
  static const MethodChannel _channel =
      MethodChannel('com.example.rfplay_client/vpn');

  // --- State ---
  VpnState _state = VpnState.disconnected;
  VpnMode _mode = VpnMode.external; // default to external (safer)
  String? _selectedNode;
  String? _connectedNode;
  String? _errorMessage;

  // Speed simulation (real speed data requires traffic accounting)
  double _downloadSpeed = 0;
  double _uploadSpeed = 0;
  Duration _connectedTime = Duration.zero;

  // Timers
  Timer? _speedTimer;
  Timer? _durationTimer;

  // Node latencies
  final Map<String, NodeLatency> _latencies = {};

  // Random for speed simulation
  final Random _random = Random();

  // --- Getters ---
  VpnState get state => _state;
  VpnMode get mode => _mode;
  bool get isConnected => _state == VpnState.connected;
  bool get isConnecting => _state == VpnState.connecting;
  String? get selectedNode => _selectedNode;
  String? get connectedNode => _connectedNode;
  String? get errorMessage => _errorMessage;
  double get downloadSpeed => _downloadSpeed;
  double get uploadSpeed => _uploadSpeed;
  Duration get connectedTime => _connectedTime;
  Map<String, NodeLatency> get latencies => Map.unmodifiable(_latencies);

  // --- Mode ---
  void setMode(VpnMode mode) {
    _mode = mode;
    notifyListeners();
  }

  // --- Node selection ---
  void selectNode(String? node) {
    _selectedNode = node;
    notifyListeners();
  }

  // --- TCP Ping (measure node latency) ---
  Future<NodeLatency> pingNode(String name, String host, int port) async {
    final stopwatch = Stopwatch()..start();
    try {
      final socket = await Socket.connect(
        host,
        port,
        timeout: const Duration(seconds: 5),
      );
      await socket.close();
      stopwatch.stop();
      final latency = NodeLatency(
        nodeName: name,
        latencyMs: stopwatch.elapsedMilliseconds,
        measuredAt: DateTime.now(),
      );
      _latencies[name] = latency;
      notifyListeners();
      return latency;
    } catch (_) {
      final latency = NodeLatency(
        nodeName: name,
        latencyMs: -1,
        measuredAt: DateTime.now(),
      );
      _latencies[name] = latency;
      notifyListeners();
      return latency;
    }
  }

  /// Ping all nodes in a subscription
  Future<void> pingAllNodes(SubscriptionInfo sub) async {
    for (final node in sub.nodes) {
      final parsed = _parseNodeAddress(node.uri);
      if (parsed != null) {
        await pingNode(node.name, parsed.$1, parsed.$2);
      }
    }
  }

  /// Parse host:port from a node URI
  (String, int)? _parseNodeAddress(String uri) {
    try {
      // V2Ray URI format: protocol://uuid@host:port?... or plain host:port
      if (uri.contains('://')) {
        final afterScheme = uri.split('://')[1];
        final afterAt = afterScheme.contains('@') ? afterScheme.split('@')[1] : afterScheme;
        final hostPort = afterAt.split('?')[0].split('/')[0];
        if (hostPort.contains(':')) {
          final parts = hostPort.split(':');
          return (parts[0], int.parse(parts[1]));
        }
        return (hostPort, 443); // default port
      }
      if (uri.contains(':')) {
        final parts = uri.split(':');
        return (parts[0], int.parse(parts[1]));
      }
      return (uri, 443);
    } catch (_) {
      return null;
    }
  }

  // --- Connect ---
  Future<bool> connect() async {
    if (_state == VpnState.connected || _state == VpnState.connecting) return false;

    _state = VpnState.connecting;
    _errorMessage = null;
    notifyListeners();

    if (_mode == VpnMode.builtin) {
      return _connectBuiltin();
    } else {
      return _connectExternal();
    }
  }

  /// Connect via built-in Android VpnService
  Future<bool> _connectBuiltin() async {
    // Validate node selection
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // Try to start the native VPN service via MethodChannel
    try {
      final result = await _channel.invokeMethod('startVpn', {
        'host': 'proxy.example.com', // Would use node.address
        'port': 443,
        'name': 'RFPlay - $_selectedNode',
      });

      if (result == 'PERMISSION_REQUIRED') {
        // User needs to grant VPN permission via the system dialog
        // The Flutter side shows a snackbar / dialog
        _state = VpnState.disconnected;
        _errorMessage = '需要 VPN 权限，请允许 VPN 连接请求';
        notifyListeners();
        // The permission result comes back via onActivityResult
        // For now, we fall back to external mode
        debugPrint('[VpnService] VPN permission required, falling back to external mode');
        _mode = VpnMode.external;
        return _connectExternal();
      }

      if (result == 'VPN_STARTED') {
        _state = VpnState.connected;
        _connectedNode = _selectedNode;
        _startSimulation();
        notifyListeners();
        return true;
      }

      _state = VpnState.error;
      _errorMessage = 'VPN 启动失败: $result';
      notifyListeners();
      return false;
    } on MissingPluginException {
      // Not on Android — fall back to external mode
      debugPrint('[VpnService] MethodChannel not available (not Android), using external mode');
      _mode = VpnMode.external;
      return _connectExternal();
    } on PlatformException catch (e) {
      _state = VpnState.error;
      _errorMessage = 'VPN 错误: ${e.message}';
      notifyListeners();
      return false;
    }
  }

  /// Connect by opening external VPN app (v2rayNG, Clash, etc.)
  Future<bool> _connectExternal() async {
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // Try to open subscription in external VPN apps
    // Priority: v2rayNG > Clash > Sing-box > generic
    bool launched = false;

    // v2rayNG: v2rayng://install-subscribe?url=...
    final subscriptionUrl = _buildSubscriptionUrl();
    if (subscriptionUrl != null) {
      final v2rayUri = Uri.parse(
        'v2rayng://install-subscribe?url=${Uri.encodeComponent(subscriptionUrl)}',
      );
      if (await canLaunchUrl(v2rayUri)) {
        await launchUrl(v2rayUri, mode: LaunchMode.externalApplication);
        launched = true;
      }
    }

    if (!launched) {
      // Fallback: copy subscription URL to clipboard and let user paste
      // The UI will show a dialog
      _state = VpnState.disconnected;
      _errorMessage = '复制订阅链接到剪贴板';
      notifyListeners();
      return false;
    }

    // After launching external app, we show "connected" in the app
    // but the actual VPN connection is managed by the external app
    _state = VpnState.connected;
    _connectedNode = _selectedNode;
    _startSimulation();
    notifyListeners();
    return true;
  }

  /// Build subscription URL for external apps
  String? _buildSubscriptionUrl() {
    // Use the subscription endpoint URL
    // In production, this comes from /client/config or the portal
    return null; // Placeholder — set via configure()
  }

  /// Configure subscription URL from API
  String? _configuredSubscriptionUrl;

  void configure(String? subscriptionUrl) {
    _configuredSubscriptionUrl = subscriptionUrl;
    notifyListeners();
  }

  /// Get configured subscription URL
  String? get configuredSubscriptionUrl => _configuredSubscriptionUrl;

  // --- Disconnect ---
  void disconnect() {
    if (_state == VpnState.disconnected) return;

    _state = VpnState.disconnecting;
    notifyListeners();

    // Stop native VPN
    try {
      _channel.invokeMethod('stopVpn');
    } catch (_) {}

    _stopSimulation();
    _state = VpnState.disconnected;
    _connectedNode = null;
    _downloadSpeed = 0;
    _uploadSpeed = 0;
    _connectedTime = Duration.zero;
    notifyListeners();
  }

  // --- Toggle ---
  Future<void> toggle() async {
    if (isConnected || isConnecting) {
      disconnect();
    } else {
      await connect();
    }
  }

  // --- Check VPN permission ---
  Future<bool> checkVpnPermission() async {
    try {
      return await _channel.invokeMethod('checkVpnPermission') ?? false;
    } catch (_) {
      return false;
    }
  }

  // --- Check if native VPN is running ---
  Future<bool> isVpnRunning() async {
    try {
      return await _channel.invokeMethod('isVpnRunning') ?? false;
    } catch (_) {
      return false;
    }
  }

  // --- Speed simulation (for UI demo; real speed requires TUN accounting) ---
  void _startSimulation() {
    _speedTimer?.cancel();
    _durationTimer?.cancel();

    _updateSpeeds();

    _speedTimer = Timer.periodic(const Duration(seconds: 2), (_) {
      if (isConnected) {
        _updateSpeeds();
        notifyListeners();
      }
    });

    _durationTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (isConnected) {
        _connectedTime += const Duration(seconds: 1);
        notifyListeners();
      }
    });
  }

  void _updateSpeeds() {
    _downloadSpeed = 15 + _random.nextDouble() * 35;
    _uploadSpeed = 5 + _random.nextDouble() * 10;
  }

  void _stopSimulation() {
    _speedTimer?.cancel();
    _speedTimer = null;
    _durationTimer?.cancel();
    _durationTimer = null;
  }

  @override
  void dispose() {
    _stopSimulation();
    super.dispose();
  }
}
