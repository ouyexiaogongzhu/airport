import 'dart:convert';
import 'dart:io';
import 'package:http/http.dart' as http;

class ApiService {
  /// Base URL for API requests.
  ///
  /// Defaults to '/api/v1' (relative path). For mobile apps, this must be
  /// overridden at build time via --dart-define=API_BASE_URL=... or at
  /// runtime via [configure].
  ///
  /// Examples:
  ///   flutter run --dart-define=API_BASE_URL=http://10.0.2.2:8080/api/v1
  ///   flutter build apk --dart-define=API_BASE_URL=https://example.com/api/v1
  static String get baseUrl => _baseUrl;
  static String _baseUrl = _resolveDefault();
  static const Duration _timeout = Duration(seconds: 10);

  String? _token;
  static final ApiService _instance = ApiService._internal();

  factory ApiService() => _instance;

  ApiService._internal();

  /// Resolve the default base URL from compile-time define or fallback.
  static String _resolveDefault() {
    const envUrl = String.fromEnvironment(
      'API_BASE_URL',
      defaultValue: '/api/v1',
    );
    return envUrl;
  }

  /// Override the API base URL at runtime (e.g., from remote config).
  static void configure({required String baseUrl}) {
    _baseUrl = baseUrl;
  }

  String? get token => _token;

  void setToken(String? token) {
    _token = token;
  }

  Map<String, String> get _headers {
    final headers = <String, String>{
      'Content-Type': 'application/json',
      'Accept': 'application/json',
    };
    if (_token != null && _token!.isNotEmpty) {
      headers['Authorization'] = 'Bearer $_token';
    }
    return headers;
  }

  Future<Map<String, dynamic>> get(String path, {Map<String, String>? queryParams}) async {
    try {
      var uri = Uri.parse('$baseUrl$path');
      if (queryParams != null && queryParams.isNotEmpty) {
        uri = uri.replace(queryParameters: queryParams);
      }
      final response = await http.get(uri, headers: _headers).timeout(_timeout);
      return _handleResponse(response);
    } on SocketException {
      throw ApiException('网络连接失败，请检查网络');
    } on HttpException {
      throw ApiException('服务器错误');
    } on FormatException {
      throw ApiException('数据格式错误');
    }
  }

  Future<Map<String, dynamic>> post(String path, {Map<String, dynamic>? body}) async {
    try {
      final response = await http
          .post(
            Uri.parse('$baseUrl$path'),
            headers: _headers,
            body: body != null ? jsonEncode(body) : null,
          )
          .timeout(_timeout);
      return _handleResponse(response);
    } on SocketException {
      throw ApiException('网络连接失败，请检查网络');
    } on HttpException {
      throw ApiException('服务器错误');
    } on FormatException {
      throw ApiException('数据格式错误');
    }
  }

  Future<Map<String, dynamic>> put(String path, {Map<String, dynamic>? body}) async {
    try {
      final response = await http
          .put(
            Uri.parse('$baseUrl$path'),
            headers: _headers,
            body: body != null ? jsonEncode(body) : null,
          )
          .timeout(_timeout);
      return _handleResponse(response);
    } on SocketException {
      throw ApiException('网络连接失败，请检查网络');
    } on HttpException {
      throw ApiException('服务器错误');
    } on FormatException {
      throw ApiException('数据格式错误');
    }
  }

  Future<Map<String, dynamic>> delete(String path) async {
    try {
      final response = await http
          .delete(Uri.parse('$baseUrl$path'), headers: _headers)
          .timeout(_timeout);
      return _handleResponse(response);
    } on SocketException {
      throw ApiException('网络连接失败，请检查网络');
    } on HttpException {
      throw ApiException('服务器错误');
    } on FormatException {
      throw ApiException('数据格式错误');
    }
  }

  Map<String, dynamic> _handleResponse(http.Response response) {
    final body = response.body.isNotEmpty ? jsonDecode(response.body) as Map<String, dynamic> : <String, dynamic>{};

    if (response.statusCode >= 200 && response.statusCode < 300) {
      return body;
    } else if (response.statusCode == 401) {
      _token = null;
      throw ApiException('未授权，请重新登录');
    } else if (response.statusCode == 403) {
      final msg = body['error'] as String? ?? '没有权限';
      throw ApiException(msg);
    } else if (response.statusCode == 404) {
      throw ApiException('请求的资源不存在');
    } else if (response.statusCode >= 500) {
      throw ApiException('服务器内部错误');
    } else {
      final msg = body['error'] as String? ?? body['message'] as String? ?? '请求失败 (${response.statusCode})';
      throw ApiException(msg);
    }
  }
}

class ApiException implements Exception {
  final String message;
  ApiException(this.message);

  @override
  String toString() => message;
}
