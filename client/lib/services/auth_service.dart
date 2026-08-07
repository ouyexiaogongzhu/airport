import 'package:flutter/foundation.dart' show kIsWeb, ChangeNotifier, debugPrint;
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../models/user.dart';
import 'api_service.dart';

class AuthService extends ChangeNotifier {
  final ApiService _api = ApiService();

  /// Key used to persist the auth token in platform secure storage
  /// (Keychain on iOS/macOS, Keystore-backed encryption on Android).
  static const String _tokenKey = 'auth_token';

  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  /// In-memory fallback for web (secure storage uses WebCrypto there) and
  /// for platforms where the secure storage backend is unavailable
  /// (e.g. Linux without libsecret, or unit tests without platform channels).
  String? _memoryToken;

  User? _currentUser;
  bool _isLoading = false;
  String? _error;

  User? get currentUser => _currentUser;
  bool get isLoading => _isLoading;
  bool get isLoggedIn => _currentUser != null;
  String? get error => _error;

  Future<String?> _loadToken() async {
    if (kIsWeb) return _memoryToken;
    try {
      return await _storage.read(key: _tokenKey) ?? _memoryToken;
    } catch (e) {
      debugPrint('[AuthService] Failed to load token: $e');
      return _memoryToken;
    }
  }

  Future<void> _saveToken(String token) async {
    _memoryToken = token;
    if (kIsWeb) return;
    try {
      await _storage.write(key: _tokenKey, value: token);
    } catch (e) {
      debugPrint('[AuthService] Failed to save token: $e');
    }
  }

  Future<void> _deleteToken() async {
    _memoryToken = null;
    if (kIsWeb) return;
    try {
      await _storage.delete(key: _tokenKey);
    } catch (e) {
      debugPrint('[AuthService] Failed to delete token: $e');
    }
  }

  /// Initialize: restore token from secure storage
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
