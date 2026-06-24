import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/services/vpn_service.dart';
import 'package:rfplay_client/widgets/vpn_button.dart';

/// Helper to wrap a VpnButton in a MaterialApp so themes and text styles work.
Widget wrapVpnButton(VpnButton button) {
  return MaterialApp(home: Scaffold(body: Center(child: button)));
}

void main() {
  group('VpnButton', () {
    testWidgets('renders disconnected state (grey circle, "已断开" label)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.disconnected)),
      );

      // The button contains a GestureDetector wrapper
      expect(find.byType(GestureDetector), findsOneWidget);

      // Should show the disconnected label text
      expect(find.text('已断开'), findsOneWidget);

      // Grey color icon (vpn_lock_outlined for disconnected)
      // The icon widget should be present
      expect(find.byIcon(Icons.vpn_lock_outlined), findsOneWidget);
    });

    testWidgets('renders connected state (green circle, "已连接" label)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.connected)),
      );

      // Should show the connected label
      expect(find.text('已连接'), findsOneWidget);

      // Should show the vpn_lock icon (filled)
      expect(find.byIcon(Icons.vpn_lock), findsOneWidget);
    });

    testWidgets('renders connecting state with progress indicator and label',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.connecting)),
      );

      expect(find.text('连接中...'), findsOneWidget);

      // Should show a CircularProgressIndicator
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });

    testWidgets('renders disconnecting state with progress indicator',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.disconnecting)),
      );

      expect(find.text('断开中...'), findsOneWidget);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });

    testWidgets('renders error state', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.error)),
      );

      expect(find.text('错误'), findsOneWidget);
      expect(find.byIcon(Icons.vpn_lock_outlined), findsOneWidget);
    });

    testWidgets('fires onTap callback when tapped',
        (WidgetTester tester) async {
      int tapCount = 0;

      await tester.pumpWidget(
        wrapVpnButton(
          VpnButton(
            state: VpnState.disconnected,
            onTap: () => tapCount++,
          ),
        ),
      );

      await tester.tap(find.byType(GestureDetector));
      expect(tapCount, equals(1));
    });

    testWidgets('does not fire onTap when onTap is null',
        (WidgetTester tester) async {
      // Should not crash
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.disconnected)),
      );

      // Tap should be a no-op
      await tester.tap(find.byType(GestureDetector));
      // No crash = success
    });

    testWidgets('responds to VpnState change from disconnected to connected',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.disconnected)),
      );

      expect(find.text('已断开'), findsOneWidget);
      expect(find.text('已连接'), findsNothing);

      // Rebuild with connected state
      await tester.pumpWidget(
        wrapVpnButton(const VpnButton(state: VpnState.connected)),
      );
      await tester.pump(); // Let animation settle

      expect(find.text('已连接'), findsOneWidget);
      expect(find.text('已断开'), findsNothing);
    });

    testWidgets('accepts custom size', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapVpnButton(
          const VpnButton(state: VpnState.disconnected, size: 200),
        ),
      );

      // Should render without errors; the container size is driven by the size param
      expect(find.byType(VpnButton), findsOneWidget);
    });
  });
}
