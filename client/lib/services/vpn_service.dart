import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
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

  /// Ping an arbitrary node list（对导入节点测速）。
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
  Future<bool> connect({List<VpnNode>? nodes}) async {
    if (_state == VpnState.connected || _state == VpnState.connecting) return false;

    _activeNodes = nodes;
    _activeNodesByName = _indexNodesByName(_activeNodes);

    final activeNodes = _activeNodeList;

    // Pre-connection checks
    if (nodes == null || nodes.isEmpty) {
      _state = VpnState.error;
      _errorMessage = '未找到可用节点，请先导入订阅';
      notifyListeners();
      return false;
    }

    if (activeNodes.isEmpty) {
      _state = VpnState.error;
      _errorMessage = '暂无可用的节点，请先导入订阅';
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

  /// 当前可用的节点列表（手动导入的节点）。
  List<VpnNode> get _activeNodeList => _activeNodes ?? const <VpnNode>[];

  /// name → node lookup for the currently active node list. Rebuilt once per
  /// [connect] instead of re-scanning the list for the selected node on every
  /// connect path.
  Map<String, VpnNode>? _activeNodesByName;

  /// Index a node list by name. Duplicate names keep the first occurrence to
  /// match the previous `where((n) => n.name == selected).first` linear-scan
  /// semantics.
  static Map<String, VpnNode>? _indexNodesByName(List<VpnNode>? nodes) {
    if (nodes == null || nodes.isEmpty) return null;
    final index = <String, VpnNode>{};
    for (final node in nodes) {
      index.putIfAbsent(node.name, () => node);
    }
    return index;
  }

  /// Desktop: run libXray via FFI with a local SOCKS/HTTP inbound.
  Future<bool> _connectDesktop() async {
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // Find the selected node URI and build a local-port config.
    final nodeUri = _activeNodesByName?[_selectedNode]?.uri;

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
    final nodeUri = _activeNodesByName?[_selectedNode]?.uri;

    if (nodeUri == null) {
      _state = VpnState.error;
      _errorMessage = '未找到所选节点信息';
      notifyListeners();
      return false;
    }

    // Parse the node URI once and reuse it for both the Xray config build and
    // the native bridge host/port (previously parsed up to three times).
    XrayNodeConfig? parsed;
    String? configJson;
    try {
      parsed = XrayConfigBuilder.parseUri(nodeUri);
      configJson =
          jsonEncode(XrayConfigBuilder.buildTunClientConfig(nodeUri, node: parsed));
    } catch (e) {
      debugPrint('[VpnService] failed to build xray config: $e');
    }

    // Try to start the native VPN service via MethodChannel
    try {
      final result = await _channel.invokeMethod('startVpn', {
        'host': parsed?.host ?? 'proxy.example.com',
        'port': parsed?.port ?? 443,
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

  /// 手动导入的节点列表。
  List<VpnNode>? _activeNodes;

  /// Connect by opening external VPN app (v2rayNG, Clash, etc.)
  Future<bool> _connectExternal() async {
    if (_selectedNode == null) {
      _state = VpnState.error;
      _errorMessage = '请先选择一个节点';
      notifyListeners();
      return false;
    }

    // 无内置引擎的平台：复制订阅链接到剪贴板，交由外部代理 App 处理。
    _state = VpnState.disconnected;
    _errorMessage = '复制订阅链接到剪贴板';
    notifyListeners();
    return false;
  }

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
  Future<void> toggle({List<VpnNode>? nodes}) async {
    if (isConnected || isConnecting) {
      await disconnect();
    } else {
      await connect(nodes: nodes);
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
