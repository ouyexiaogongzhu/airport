import 'package:flutter/foundation.dart';
import '../models/user.dart';
import 'api_service.dart';

class AuthService extends ChangeNotifier {
  final ApiService _api = ApiService();

  User? _currentUser;
  bool _isLoading = false;
  String? _error;

  User? get currentUser => _currentUser;
  bool get isLoading => _isLoading;
  bool get isLoggedIn => _currentUser != null;
  String? get error => _error;

  /// Login: try real API → mock fallback
  Future<bool> login(String username, String password) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _api.post('/auth/login', body: {
        'username': username,
        'password': password,
      });

      final user = User.fromJson({
        ...response['user'] as Map<String, dynamic>? ?? {},
        'token': response['token'] as String?,
        'username': response['username'] as String? ?? username,
      });

      _api.setToken(user.token);
      _currentUser = user;
      _isLoading = false;
      notifyListeners();
      return true;
    } catch (e) {
      // Mock fallback
      if (username == 'admin' && password == 'admin123') {
        _currentUser = User(
          id: 1,
          username: 'admin',
          email: 'admin@rfplay.com',
          role: 'admin',
          token: 'mock_token_admin_123',
          createdAt: '2025-01-01T00:00:00Z',
        );
        _api.setToken(_currentUser!.token);
        _isLoading = false;
        notifyListeners();
        return true;
      }

      if (username == 'user' && password == 'user123') {
        _currentUser = User(
          id: 2,
          username: 'user',
          email: 'user@rfplay.com',
          role: 'user',
          token: 'mock_token_user_456',
          createdAt: '2025-03-15T00:00:00Z',
        );
        _api.setToken(_currentUser!.token);
        _isLoading = false;
        notifyListeners();
        return true;
      }

      _error = '用户名或密码错误';
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Register: try real API → mock fallback
  Future<bool> register(String username, String email, String password) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await _api.post('/auth/register', body: {
        'username': username,
        'email': email,
        'password': password,
      });

      final user = User.fromJson({
        ...response['user'] as Map<String, dynamic>? ?? {},
        'token': response['token'] as String?,
        'username': response['username'] as String? ?? username,
      });

      _api.setToken(user.token);
      _currentUser = user;
      _isLoading = false;
      notifyListeners();
      return true;
    } catch (e) {
      // Mock fallback — accept any registration
      _currentUser = User(
        id: DateTime.now().millisecondsSinceEpoch,
        username: username,
        email: email,
        role: 'user',
        token: 'mock_token_${username}_${DateTime.now().millisecondsSinceEpoch}',
        createdAt: DateTime.now().toIso8601String(),
      );
      _api.setToken(_currentUser!.token);
      _isLoading = false;
      notifyListeners();
      return true;
    }
  }

  /// Logout
  void logout() {
    _currentUser = null;
    _api.setToken(null);
    notifyListeners();
  }

  /// Clear error
  void clearError() {
    _error = null;
    notifyListeners();
  }
}
