import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:provider/provider.dart';

import 'package:rfplay_client/main.dart';
import 'package:rfplay_client/services/subscription_service.dart';
import 'package:rfplay_client/services/vpn_service.dart';

void main() {
  testWidgets('RFPlayApp smoke test — renders the URL-only client',
      (WidgetTester tester) async {
    // Build our app with required providers
    await tester.pumpWidget(
      MultiProvider(
        providers: [
          ChangeNotifierProvider(create: (_) => SubscriptionService()),
          ChangeNotifierProvider(create: (_) => VpnService()),
        ],
        child: const RFPlayApp(initialRoute: '/subscription/input'),
      ),
    );

    // Verify that the app renders without crashing
    expect(find.byType(MaterialApp), findsOneWidget);
  });
}
