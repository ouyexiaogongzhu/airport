import 'dart:io';

import 'package:flutter/foundation.dart' show kIsWeb, ChangeNotifier;
import '../models/user.dart';
import 'api_service.dart';

/// Simple file-based storage path for auth token.
///
/// Uses the home directory on Linux/macOS, system temp as fallback.
/// On web, returns a dummy path (file storage is not available).
///
/// SECURITY: For production, replace file-based token storage with
/// `flutter_secure_storage` (Keychain on iOS, Keystore on Android,
/// libsecret on Linux).
String get _defaultTokenPath {
  if (kIsWeb) {
    // Web platform: file storage not available; token stays in memory only.
    return '/tmp/rfplay_auth_token_web';
  }
  final home = Platform.environment['HOME'] ??
      Platform.environment['USERPROFILE'];
  if (home != null) {
    final dir = Directory('$home/.rfplay');
    if (!dir.existsSync()) {
      try {
        dir.createSync(recursive: true);
      } catch (e) {
        debugPrint('[AuthService] Failed to create .rfplay dir: $e');
      }
    }
    return '$home/.rfplay/auth_token';
  }
  return '${Directory.systemTemp.path}/rfplay_auth_token';
}

class AuthService extends ChangeNotifier {
  final ApiService _api = ApiService();

  /// The file path used to persist the auth token.
  /// Overridable for testing.
  String _tokenPath = _defaultTokenPath;

  User? _currentUser;
  bool _isLoading = false;
  String? _error;

  User? get currentUser => _currentUser;
  bool get isLoading => _isLoading;
  bool get isLoggedIn => _currentUser != null;
  String? get error => _error;

  /// Allows overriding the token path (useful for testing).
  set tokenPath(String path) => _tokenPath = path;

  Future<String?> _loadToken() async {
    try {
      final file = File(_tokenPath);
      if (await file.exists()) {
        return await file.readAsString();
      }
    } catch (e) {
      debugPrint('[AuthService] Failed to load token: $e');
    }
    return null;
  }

  /// SECURITY: Writes token to disk in plaintext. For production, migrate
  /// to `flutter_secure_storage` which uses platform-native secure enclaves.
  Future<void> _saveToken(String token) async {
    try {
      final file = File(_tokenPath);
      await file.writeAsString(token);
      // Restrict file permissions to owner-only (Linux/macOS).
      if (!kIsWeb && !Platform.isWindows) {
        try {
          await Process.run('chmod', ['600', _tokenPath]);
        } catch (e) {
          debugPrint('[AuthService] Failed to chmod token file: $e');
        }
      }
    } catch (e) {
      debugPrint('[AuthService] Failed to save token: $e');
    }
  }

  Future<void> _deleteToken() async {
    try {
      final file = File(_tokenPath);
      if (await file.exists()) {
        await file.delete();
      }
    } catch (e) {
      debugPrint('[AuthService] Failed to delete token: $e');
    }
  }

  /// Initialize: restore token from file storage
  Future<void> init() async {
    final savedToken = await _loadToken();
    if (savedToken != null && savedToken.isNotEmpty) {
      _api.setToken(savedToken);
      // Try to load profile to verify token
      try {
        final data = await _api.get('/user/profile');
        _currentUser = User.fromJson(data);
        notifyListeners();
      } catch (_) {
        // Token expired or invalid — clear
        _api.setToken(null);
        await _deleteToken();
      }
    }
  }

  /// Login via real API
  Future<bool> login(String username, String password) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _api.post('/public/login', body: {
        'username': username,
        'password': password,
      });

      final token = response['token'] as String;
      final userData = response['user'] as Map<String, dynamic>;

      final user = User.fromJson({
        ...userData,
        'token': token,
      });

      _api.setToken(token);
      _currentUser = user;

      // Persist token
      await _saveToken(token);

      _isLoading = false;
      notifyListeners();
      return true;
    } catch (e) {
      _error = e.toString();
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Login via token import (rf_ or at_ token)
  Future<bool> tokenLogin(String token) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _api.post('/public/token-login', body: {
        'token': token,
      });

      final jwtToken = response['token'] as String;
      final userData = response['user'] as Map<String, dynamic>;

      final user = User.fromJson({
        ...userData,
        'token': jwtToken,
      });

      _api.setToken(jwtToken);
      _currentUser = user;

      // Persist token
      await _saveToken(jwtToken);

      _isLoading = false;
      notifyListeners();
      return true;
    } catch (e) {
      _error = e.toString();
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Register via real API
  Future<bool> register(String username, String email, String password) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _api.post('/public/register', body: {
        'username': username,
        'password': password,
      });

      final token = response['token'] as String;
      final userData = response['user'] as Map<String, dynamic>;

      final user = User.fromJson({
        ...userData,
        'token': token,
      });

      _api.setToken(token);
      _currentUser = user;

      await _saveToken(token);

      _isLoading = false;
      notifyListeners();
      return true;
    } catch (e) {
      _error = e.toString();
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Logout
  Future<void> logout() async {
    _currentUser = null;
    _api.setToken(null);
    await _deleteToken();
    notifyListeners();
  }

  /// Clear error
  void clearError() {
    _error = null;
    notifyListeners();
  }
}
