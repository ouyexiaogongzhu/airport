import 'dart:io';

import 'package:flutter/foundation.dart';
import '../models/user.dart';
import 'api_service.dart';

class AuthService extends ChangeNotifier {
  final ApiService _api = ApiService();
  static const String _tokenFile = 'rfplay_auth_token';

  User? _currentUser;
  bool _isLoading = false;
  String? _error;

  User? get currentUser => _currentUser;
  bool get isLoading => _isLoading;
  bool get isLoggedIn => _currentUser != null;
  String? get error => _error;

  String get _tokenPath {
    // Store token in a temp file
    final dir = Directory.systemTemp.path;
    return '$dir/$_tokenFile';
  }

  Future<String?> _loadToken() async {
    try {
      final file = File(_tokenPath);
      if (await file.exists()) {
        return await file.readAsString();
      }
    } catch (_) {}
    return null;
  }

  Future<void> _saveToken(String token) async {
    try {
      final file = File(_tokenPath);
      await file.writeAsString(token);
    } catch (_) {}
  }

  Future<void> _deleteToken() async {
    try {
      final file = File(_tokenPath);
      if (await file.exists()) {
        await file.delete();
      }
    } catch (_) {}
  }

  /// Initialize: restore token from file
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
