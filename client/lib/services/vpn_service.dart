import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:url_launcher/url_launcher.dart';
import '../config.dart';
import '../models/subscription.dart';
import 'xray_engine.dart';
import 'xray_ffi.dart';

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
// TODO(L1): Implement operator == and hashCode for value-based equality.
// Consider using package:equatable or manually overriding these.
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

/// Real VPN service with Android VpnService + libXray engine
class VpnService extends ChangeNotifier {
  static const MethodChannel _channel =
      MethodChannel('uk.rfplay.client/vpn');

  final XrayEngine _engine;

  VpnService({XrayEngine? engine}) : _engine = engine ?? _defaultEngine();

  static XrayEngine _defaultEngine() {
    if (Platform.isAndroid || Platform.isIOS) {
      return NativeXrayEngine();
    }
    // Desktop: load the libXray shared library via FFI.
    return FfiXrayEngine();
  }

  // --- State ---
  VpnState _state = VpnState.disconnected;
  VpnMode _mode = VpnMode.external; // default to external (safer)
  String? _selectedNode;
  String? _connectedNode;
  String? _errorMessage;

  // Real speed data from the engine traffic counters
  double _downloadSpeed = 0;
  double _uploadSpeed = 0;
  int _lastUpBytes = 0;
  int _lastDownBytes = 0;
  Duration _connectedTime = Duration.zero;

  // Timers
  Timer? _speedTimer;
  Timer? _durationTimer;

  // Node latencies
  final Map<String, NodeLatency> _latencies = {};

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
    } catch (e) {
      debugPrint('[VpnService] Ping failed for $name: $e');
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
    await pingAllNodesList(sub.nodes);
  }

  /// Ping an arbitrary node list (store 模式下对导入节点测速)。
  Future<void> pingAllNodesList(List<VpnNode> nodes) async {
    for (final node in nodes) {
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
  Future<bool> connect({SubscriptionInfo? subscription, List<VpnNode>? nodes}) async {
    if (_state == VpnState.connected || _state == VpnState.connecting) return false;

    _activeSubscription = subscription;
    _activeNodes = nodes;

    final activeNodes =
        _activeSubscription?.nodes ?? _activeNodes ?? const <VpnNode>[];

    // Pre-connection checks
    if (subscription == null && (nodes == null || nodes.isEmpty)) {
      _state = VpnState.error;
      _errorMessage = AppConfig.storeMode
          ? '未找到可用节点，请先导入订阅'
          : '未找到订阅信息，请先购买套餐';
      notifyListeners();
      return false;
    }

    if (activeNodes.isEmpty) {
      _state = VpnState.error;
      _errorMessage = AppConfig.storeMode
          ? '暂无可用的节点，请先导入订阅'
          : '套餐暂无可用节点，请联系管理员';
      notifyListeners();
      return false;
    }

    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    _state = VpnState.connecting;
    _errorMessage = null;
    notifyListeners();

    // Desktop platforms (Windows/macOS/Linux) use the FFI engine.
    if (!Platform.isAndroid && !Platform.isIOS) {
      return _connectDesktop();
    }

    if (_mode == VpnMode.builtin) {
      return _connectBuiltin();
    } else {
      return _connectExternal();
    }
  }

  /// 当前可用的节点列表（服务器订阅节点或 store 模式导入节点）。
  List<VpnNode> get _activeNodeList =>
      _activeSubscription?.nodes ?? _activeNodes ?? const <VpnNode>[];

  /// Desktop: run libXray via FFI with a local SOCKS/HTTP inbound.
  Future<bool> _connectDesktop() async {
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // Find the selected node URI and build a local-port config.
    final nodeUri = _activeNodeList
        .where((n) => n.name == _selectedNode)
        .map((n) => n.uri)
        .firstOrNull;

    if (nodeUri == null) {
      _state = VpnState.error;
      _errorMessage = '未找到所选节点信息';
      notifyListeners();
      return false;
    }

    try {
      final configJson =
          jsonEncode(XrayConfigBuilder.buildLocalPortConfig(nodeUri));
      await _engine.start(configJson);
    } catch (e) {
      debugPrint('[VpnService] desktop engine start failed: $e');
      _state = VpnState.error;
      _errorMessage = '启动 Xray 引擎失败（请确认已安装 libXray）: $e';
      notifyListeners();
      return false;
    }

    _state = VpnState.connected;
    _connectedNode = _selectedNode;
    _startTrafficMonitor();
    _errorMessage = '已通过本地代理 127.0.0.1:10808 连接';
    notifyListeners();
    return true;
  }

  /// Connect via built-in Android VpnService + libXray engine
  Future<bool> _connectBuiltin() async {
    // Validate node selection
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // Find the selected node's URI from the active node list.
    final nodeUri = _activeNodeList
        .where((n) => n.name == _selectedNode)
        .map((n) => n.uri)
        .firstOrNull;

    if (nodeUri == null) {
      _state = VpnState.error;
      _errorMessage = '未找到所选节点信息';
      notifyListeners();
      return false;
    }

    // Build a standard Xray config from the node URI.
    String? configJson;
    try {
      configJson = jsonEncode(XrayConfigBuilder.buildTunClientConfig(nodeUri));
    } catch (e) {
      debugPrint('[VpnService] failed to build xray config: $e');
    }

    // Try to start the native VPN service via MethodChannel
    try {
      final result = await _channel.invokeMethod('startVpn', {
        'host': _proxyHostFromUri(nodeUri) ?? 'proxy.example.com',
        'port': _proxyPortFromUri(nodeUri) ?? 443,
        'name': 'RFPlay - $_selectedNode',
        'config': configJson,
      });

      if (result == 'PERMISSION_REQUIRED') {
        // User needs to grant VPN permission via the system dialog
        _state = VpnState.disconnected;
        _errorMessage = '需要 VPN 权限，请允许 VPN 连接请求';
        notifyListeners();
        // The permission result comes back via onActivityResult
        debugPrint('[VpnService] VPN permission required, waiting for grant');
        return true;
      }

      if (result == 'VPN_STARTED') {
        _state = VpnState.connected;
        _connectedNode = _selectedNode;
        _startTrafficMonitor();
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

  SubscriptionInfo? _activeSubscription;

  /// store 模式下手动导入的节点（无 SubscriptionInfo 时使用）。
  List<VpnNode>? _activeNodes;

  String? _proxyHostFromUri(String uri) {
    final node = XrayConfigBuilder.parseUri(uri);
    return node?.host;
  }

  int? _proxyPortFromUri(String uri) {
    final node = XrayConfigBuilder.parseUri(uri);
    return node?.port;
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
    _startTrafficMonitor();
    notifyListeners();
    return true;
  }

  // TODO(L6): This method always returns null, causing external VPN launch to
  // always fall back to clipboard copy. Implement by returning
  // _configuredSubscriptionUrl or fetching from the API.
  String? _buildSubscriptionUrl() {
    return _configuredSubscriptionUrl;
  }

  /// Configure subscription URL from API
  String? _configuredSubscriptionUrl;

  void configure(String? subscriptionUrl) {
    _configuredSubscriptionUrl = subscriptionUrl;
    notifyListeners();
  }

  /// Set subscription URL (no notify, for use after loading subscription data)
  void setSubscriptionUrl(String url) {
    _configuredSubscriptionUrl = url;
  }

  /// Get configured subscription URL
  String? get configuredSubscriptionUrl => _configuredSubscriptionUrl;

  // --- Disconnect ---
  Future<void> disconnect() async {
    if (_state == VpnState.disconnected) return;

    _state = VpnState.disconnecting;
    notifyListeners();

    // Stop the native VPN / engine
    if (Platform.isAndroid || Platform.isIOS) {
      try {
        await _channel.invokeMethod('stopVpn');
      } catch (e) {
        debugPrint('[VpnService] Failed to stop native VPN: $e');
      }
    } else {
      // Desktop FFI engine
      try {
        await _engine.stop();
      } catch (e) {
        debugPrint('[VpnService] Failed to stop desktop engine: $e');
      }
    }

    _stopSimulation();
    _state = VpnState.disconnected;
    _connectedNode = null;
    _downloadSpeed = 0;
    _uploadSpeed = 0;
    _connectedTime = Duration.zero;
    notifyListeners();
  }

  // --- Toggle ---
  Future<void> toggle({SubscriptionInfo? subscription, List<VpnNode>? nodes}) async {
    if (isConnected || isConnecting) {
      await disconnect();
    } else {
      await connect(subscription: subscription, nodes: nodes);
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

  // --- Real traffic monitor (libXray stats polling) ---
  void _startTrafficMonitor() {
    _speedTimer?.cancel();
    _durationTimer?.cancel();

    _lastUpBytes = 0;
    _lastDownBytes = 0;
    _downloadSpeed = 0;
    _uploadSpeed = 0;

    _speedTimer = Timer.periodic(const Duration(seconds: 2), (_) async {
      if (!isConnected) return;
      try {
        final stats = await _engine.stats();
        final upDelta = stats.upload - _lastUpBytes;
        final downDelta = stats.download - _lastDownBytes;
        if (_lastUpBytes != 0 || _lastDownBytes != 0) {
          // bytes per 2s -> bytes per second
          _uploadSpeed = (upDelta / 2).clamp(0, double.infinity);
          _downloadSpeed = (downDelta / 2).clamp(0, double.infinity);
        }
        _lastUpBytes = stats.upload;
        _lastDownBytes = stats.download;
      } catch (_) {
        // Engine stats unavailable — keep last speeds
      }
      notifyListeners();
    });

    _durationTimer = Timer.periodic(const Duration(seconds: 1), (_) {
      if (isConnected) {
        _connectedTime += const Duration(seconds: 1);
        notifyListeners();
      }
    });
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
