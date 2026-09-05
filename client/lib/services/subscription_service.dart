import 'dart:convert';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;
import '../models/subscription.dart';

/// 无账号的订阅导入服务。
///
/// 客户端只认「订阅 URL / 节点链接」：粘贴或扫码导入后解析节点列表，
/// 并把原始链接持久化到系统安全存储，重启后自动重新导入。
class SubscriptionService extends ChangeNotifier {
  /// Key used to persist the imported subscription/node source string.
  static const String _sourceKey = 'imported_source';

  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  /// In-memory fallback when the secure storage backend is unavailable
  /// (e.g. Linux without libsecret, or unit tests without platform channels).
  String? _memorySource;

  List<VpnNode> _nodes = [];
  bool _importing = false;
  String? _importError;
  String? _savedSource;

  List<VpnNode> get nodes => _nodes;
  bool get importing => _importing;
  String? get importError => _importError;
  bool get hasSavedSource => _savedSource != null && _savedSource!.isNotEmpty;

  /// 启动时读取已保存的链接（不联网）。
  Future<void> init() async {
    _savedSource = await _loadSource();
    notifyListeners();
  }

  /// 重新导入已保存的链接（联网刷新节点）。
  Future<void> restore() async {
    final source = _savedSource;
    if (source == null || source.isEmpty) return;
    if (source.startsWith('http://') || source.startsWith('https://')) {
      await importFromUrl(source);
    } else {
      await importFromLink(source);
    }
  }

  /// 从 URL 导入订阅。
  ///
  /// 抓取内容、base64 解码并解析节点 URI。成功时把 URL 持久化。
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

      final ok = _parseSubscriptionData(response.body.trim());
      if (ok) await _saveSource(url);
      return ok;
    } catch (e) {
      _importError = '导入失败: ${e.toString()}';
      _importing = false;
      notifyListeners();
      return false;
    }
  }

  /// 从文本导入（节点链接，或已是解码后的订阅内容）。
  Future<bool> importFromLink(String link) async {
    if (link.startsWith('http://') || link.startsWith('https://')) {
      return importFromUrl(link);
    }
    final ok = _parseSubscriptionData(link);
    if (ok) await _saveSource(link);
    return ok;
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

    _nodes = uris
        .asMap()
        .entries
        .map((e) => VpnNode.fromUri(e.value, e.key))
        .toList();
    _importError = null;
    _importing = false;
    notifyListeners();
    return true;
  }

  Future<String?> _loadSource() async {
    if (kIsWeb) return _memorySource;
    try {
      return await _storage.read(key: _sourceKey) ?? _memorySource;
    } catch (e) {
      debugPrint('[SubscriptionService] Failed to load source: $e');
      return _memorySource;
    }
  }

  Future<void> _saveSource(String source) async {
    _memorySource = source;
    _savedSource = source;
    if (kIsWeb) return;
    try {
      await _storage.write(key: _sourceKey, value: source);
    } catch (e) {
      debugPrint('[SubscriptionService] Failed to save source: $e');
    }
  }
}
