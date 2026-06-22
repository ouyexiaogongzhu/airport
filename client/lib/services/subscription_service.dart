import 'package:flutter/foundation.dart';
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

  SubscriptionInfo? get subscription => _subscription;
  bool get isLoading => _isLoading;
  String? get portalUrl => _portalUrl;
  String? get renewalPath => _renewalPath;
  bool get configLoaded => _configLoaded;
  String? get statusError => _statusError;

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

  /// Load subscription from real API (requires JWT)
  Future<void> loadSubscription() async {
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

  /// Get full renewal URL
  String? get renewalUrl {
    if (_portalUrl != null && _renewalPath != null) {
      return '$_portalUrl$_renewalPath';
    }
    return 'https://www.rfplay.uk/plans';
  }
}
