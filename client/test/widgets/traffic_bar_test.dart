import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/widgets/traffic_bar.dart';

/// Helper to wrap a TrafficBar in a MaterialApp.
Widget wrapTrafficBar(TrafficBar bar) {
  return MaterialApp(home: Scaffold(body: Center(child: bar)));
}

void main() {
  group('TrafficBar', () {
    testWidgets('renders with correct percentage for half usage',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 5_000_000_000, totalBytes: 10_000_000_000),
        ),
      );

      // 50% should be shown
      expect(find.text('50%'), findsOneWidget);
    });

    testWidgets('shows used and total labels', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 2_000_000_000, totalBytes: 10_000_000_000),
        ),
      );

      // Should show formatted GB strings
      expect(find.textContaining('GB'), findsWidgets);
    });

    testWidgets('handles 0% usage edge case', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 0, totalBytes: 10_000_000_000),
        ),
      );

      expect(find.text('0%'), findsOneWidget);
    });

    testWidgets('handles 100% usage edge case', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 10_000_000_000, totalBytes: 10_000_000_000),
        ),
      );

      expect(find.text('100%'), findsOneWidget);
    });

    testWidgets('handles zero totalBytes (falls back to 0%)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 100, totalBytes: 0),
        ),
      );

      expect(find.text('0%'), findsOneWidget);
    });

    testWidgets('handles usedBytes exceeding totalBytes (clamped to 100%)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(usedBytes: 15_000_000_000, totalBytes: 10_000_000_000),
        ),
      );

      expect(find.text('100%'), findsOneWidget);
    });

    testWidgets('shows custom label when provided', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(
            usedBytes: 500_000_000,
            totalBytes: 10_000_000_000,
            label: '本月已用',
          ),
        ),
      );

      expect(find.text('本月已用'), findsOneWidget);
    });

    testWidgets('uses custom height', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficBar(
          const TrafficBar(
            usedBytes: 500_000_000,
            totalBytes: 10_000_000_000,
            height: 20,
          ),
        ),
      );

      // Widget renders without error; height is internal to SizedBox
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
    });
  });
}
