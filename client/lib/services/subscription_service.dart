import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:http/http.dart' as http;
import '../config.dart';
import '../models/subscription.dart';
import 'api_service.dart';

class SubscriptionService extends ChangeNotifier {
  final ApiService _api = ApiService();

  SubscriptionInfo? _subscription;
  bool _isLoading = false;

  // Config from /client/config
  String? _portalUrl;
  String? _renewalPath;
  bool _configLoaded = false;

  // Subscription status (parsed from 403 errors)
  String? _statusError; // 'SUBSCRIPTION_PENDING', 'SUBSCRIPTION_EXPIRED', or null

  // Imported subscription (from manual URL / QR code)
  List<VpnNode> _importedNodes = [];
  bool _importing = false;
  String? _importError;

  SubscriptionInfo? get subscription => _subscription;
  bool get isLoading => _isLoading;
  String? get portalUrl => _portalUrl;
  String? get renewalPath => _renewalPath;
  bool get configLoaded => _configLoaded;
  String? get statusError => _statusError;
  List<VpnNode> get importedNodes => _importedNodes;
  bool get importing => _importing;
  String? get importError => _importError;

  /// 有效节点列表：优先使用服务器下发的订阅节点；
  /// store 模式（无账号体系）下使用手动导入的节点。
  List<VpnNode> get effectiveNodes => _subscription?.nodes ?? _importedNodes;

  /// Load client config (public, no auth needed)
  Future<void> loadConfig() async {
    try {
      final data = await _api.get('/client/config');
      _portalUrl = data['portal_url'] as String?;
      _renewalPath = data['renewal_path'] as String?;
      _configLoaded = true;
      notifyListeners();
    } catch (_) {
      // Default fallback
      _portalUrl = 'https://www.rfplay.uk';
      _renewalPath = '/plans';
      _configLoaded = true;
      notifyListeners();
    }
  }

  /// Load subscription from real API (requires JWT).
  ///
  /// store 模式没有账号体系，直接跳过，避免发起无意义的认证请求。
  Future<void> loadSubscription() async {
    if (AppConfig.storeMode) return;

    _isLoading = true;
    _statusError = null;
    notifyListeners();

    try {
      final data = await _api.get('/client/subscription');
      _subscription = SubscriptionInfo.fromJson(data);
      _statusError = null;
    } on ApiException catch (e) {
      _subscription = null;
      final msg = e.message;
      if (msg == 'SUBSCRIPTION_PENDING') {
        _statusError = 'SUBSCRIPTION_PENDING';
      } else if (msg == 'SUBSCRIPTION_EXPIRED') {
        _statusError = 'SUBSCRIPTION_EXPIRED';
      } else {
        _statusError = msg;
      }
    } catch (_) {
      _subscription = null;
      _statusError = '网络错误';
    }

    _isLoading = false;
    notifyListeners();
  }

  /// Clear error
  void clearError() {
    _statusError = null;
    notifyListeners();
  }

  /// Import subscription from a URL (manual paste or QR scan).
  ///
  /// Fetches the URL, decodes base64 response, and parses node URIs.
  /// On success, [_importedNodes] is populated and [importing] = false.
  /// On error, [importError] is set.
  Future<bool> importFromUrl(String url) async {
    _importing = true;
    _importError = null;
    notifyListeners();

    // SECURITY: reject plain HTTP so subscription tokens and node credentials
    // are never transmitted (or fetched) in cleartext / subject to MITM.
    if (!url.trim().startsWith('https://')) {
      _importError = '订阅链接必须使用 HTTPS';
      _importing = false;
      notifyListeners();
      return false;
    }

    try {
      final response = await http
          .get(Uri.parse(url), headers: {'Accept': 'text/plain'})
          .timeout(const Duration(seconds: 15));

      if (response.statusCode != 200) {
        _importError = '服务器返回 ${response.statusCode}';
        _importing = false;
        notifyListeners();
        return false;
      }

      final body = response.body.trim();
      return _parseSubscriptionData(body);
    } catch (e) {
      _importError = '导入失败: ${e.toString()}';
      _importing = false;
      notifyListeners();
      return false;
    }
  }

  /// Import subscription from decoded text (QR scanned text that is the URL itself).
  Future<bool> importFromLink(String link) async {
    if (link.startsWith('http://') || link.startsWith('https://')) {
      return importFromUrl(link);
    }
    // Maybe it's already decoded subscription data
    return _parseSubscriptionData(link);
  }

  /// Parse raw subscription data (base64-decoded or plain text with node URIs).
  bool _parseSubscriptionData(String body) {
    String decoded;

    // Try base64 decode first (standard subscription format)
    try {
      decoded = utf8.decode(base64.decode(body));
    } catch (_) {
      // Not base64 — use raw body
      decoded = body;
    }

    // Parse node URIs: one per line, or comma-separated, or JSON array
    final List<String> uris = [];

    // Try JSON array
    if (decoded.trimLeft().startsWith('[')) {
      try {
        final parsed = jsonDecode(decoded) as List;
        for (final e in parsed) {
          if (e is String && e.isNotEmpty) uris.add(e);
        }
      } catch (_) {
        // Fall through
      }
    }

    // Try line-by-line
    if (uris.isEmpty) {
      for (final line in decoded.split('\n')) {
        final trimmed = line.trim();
        if (trimmed.isNotEmpty) uris.add(trimmed);
      }
    }

    if (uris.isEmpty) {
      _importError = '未找到有效的节点信息';
      _importing = false;
      notifyListeners();
      return false;
    }

    _importedNodes = uris
        .asMap()
        .entries
        .map((e) => VpnNode.fromUri(e.value, e.key))
        .toList();
    _importError = null;
    _importing = false;
    notifyListeners();
    return true;
  }

  /// Clear imported nodes
  void clearImport() {
    _importedNodes = [];
    _importError = null;
    _importing = false;
    notifyListeners();
  }

  /// Get full renewal URL.
  ///
  /// store 模式为通用代理客户端，禁止携带/跳转官网续费地址，一律返回 null。
  String? get renewalUrl {
    if (AppConfig.storeMode) return null;
    if (_portalUrl != null && _renewalPath != null) {
      return '$_portalUrl$_renewalPath';
    }
    return 'https://www.rfplay.uk/plans';
  }
}
