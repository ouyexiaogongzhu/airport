import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import '../models/user.dart';
import 'api_service.dart';

class AuthService extends ChangeNotifier {
  final ApiService _api = ApiService();
  static const String _tokenKey = 'rfplay_auth_token';

  final FlutterSecureStorage _secureStorage = const FlutterSecureStorage();

  User? _currentUser;
  bool _isLoading = false;
  String? _error;

  User? get currentUser => _currentUser;
  bool get isLoading => _isLoading;
  bool get isLoggedIn => _currentUser != null;
  String? get error => _error;

  Future<String?> _loadToken() async {
    try {
      return await _secureStorage.read(key: _tokenKey);
    } catch (_) {}
    return null;
  }

  Future<void> _saveToken(String token) async {
    try {
      await _secureStorage.write(key: _tokenKey, value: token);
    } catch (_) {}
  }

  Future<void> _deleteToken() async {
    try {
      await _secureStorage.delete(key: _tokenKey);
    } catch (_) {}
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
      final response = await _api.post('/auth/token-login', body: {
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
