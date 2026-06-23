import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'package:rfplay_client/main.dart';
import 'package:rfplay_client/services/auth_service.dart';
import 'package:rfplay_client/services/subscription_service.dart';
import 'package:rfplay_client/services/vpn_service.dart';

void main() {
  testWidgets('RFPlayApp smoke test — renders login screen',
      (WidgetTester tester) async {
    // Create unauthenticated AuthService (no init needed for basic smoke test)
    final authService = AuthService();

    // Build our app with required providers
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider.value(value: authService),
          ChangeNotifierProvider(create: (_) => SubscriptionService()),
          ChangeNotifierProvider(create: (_) => VpnService()),
        ],
        child: RFPlayApp(authService: authService),
      ),
    );

    // Verify that the app renders without crashing
    expect(find.byType(MaterialApp), findsOneWidget);
  });
}
