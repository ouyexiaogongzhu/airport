import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/widgets/loading_overlay.dart';

/// Helper to wrap a LoadingOverlay in a Stack within a MaterialApp.
/// LoadingOverlay uses Positioned.fill, so it must be inside a Stack.
Widget wrapLoadingOverlay(LoadingOverlay overlay) {
  return MaterialApp(
    home: Scaffold(
      body: Stack(
        children: [
          const Center(child: Text('Content behind overlay')),
          overlay,
        ],
      ),
    ),
  );
}

void main() {
  group('LoadingOverlay', () {
    testWidgets('shows loading indicator when isLoading is true',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(const LoadingOverlay()),
      );

      // CircularProgressIndicator should be present
      expect(find.byType(CircularProgressIndicator), findsOneWidget);

      // Background content should still be there
      expect(find.text('Content behind overlay'), findsOneWidget);
    });

    testWidgets('hides when isLoading is false',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(const LoadingOverlay(isLoading: false)),
      );

      // No CircularProgressIndicator when not loading
      expect(find.byType(CircularProgressIndicator), findsNothing);

      // Background content visible
      expect(find.text('Content behind overlay'), findsOneWidget);
    });

    testWidgets('shows optional message text', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(const LoadingOverlay(message: '加载中...')),
      );

      expect(find.text('加载中...'), findsOneWidget);
    });

    testWidgets('hides message when message is null',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(const LoadingOverlay()),
      );

      expect(find.text('加载中...'), findsNothing);
    });

    testWidgets('uses custom indicator widget', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(
          const LoadingOverlay(
            indicator: Text('Custom Indicator'),
          ),
        ),
      );

      expect(find.text('Custom Indicator'), findsOneWidget);
      // Default CircularProgressIndicator should not be present
      expect(find.byType(CircularProgressIndicator), findsNothing);
    });

    testWidgets('uses custom opacity value', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapLoadingOverlay(
          const LoadingOverlay(opacity: 0.8),
        ),
      );

      // Still shows loading indicator
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
    });
  });
}
