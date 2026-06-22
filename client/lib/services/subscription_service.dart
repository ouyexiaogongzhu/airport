import 'package:flutter/foundation.dart';
import '../models/subscription.dart';

class SubscriptionService extends ChangeNotifier {
  Subscription? _subscription;
  bool _isLoading = false;

  Subscription? get subscription => _subscription;
  bool get isLoading => _isLoading;

  /// Load subscription: mock hardcoded data
  Future<void> loadSubscription() async {
    _isLoading = true;
    notifyListeners();

    await Future.delayed(const Duration(milliseconds: 500));

    _subscription = Subscription(
      planName: 'Premium VPN',
      expiryDate: '2026-12-31',
      totalTraffic: 100,
      usedTraffic: 34.2,
      nodes: [
        VpnNode(
          name: '东京',
          location: '日本',
          latency: 80,
          isOnline: true,
        ),
        VpnNode(
          name: '新加坡',
          location: '新加坡',
          latency: 120,
          isOnline: true,
        ),
        VpnNode(
          name: '洛杉矶',
          location: '美国',
          latency: 200,
          isOnline: true,
        ),
      ],
    );

    _isLoading = false;
    notifyListeners();
  }
}
