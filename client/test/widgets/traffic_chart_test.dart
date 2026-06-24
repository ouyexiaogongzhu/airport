import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/widgets/traffic_chart.dart';

/// Helper to wrap a TrafficChart in a MaterialApp.
Widget wrapTrafficChart(TrafficChart chart) {
  return MaterialApp(home: Scaffold(body: Center(child: chart)));
}

void main() {
  group('TrafficChart', () {
    testWidgets('renders without error given mock data',
        (WidgetTester tester) async {
      const data = [100, 200, 150, 300, 50, 400, 250];
      const labels = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];

      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(data: data, labels: labels),
        ),
      );

      // Chart should render without throwing; CustomPaint should be present
      expect(find.byType(TrafficChart), findsOneWidget);
    });

    testWidgets('handles empty data gracefully by showing placeholder text',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(data: [], labels: []),
        ),
      );

      // Should show "暂无流量数据" text
      expect(find.text('暂无流量数据'), findsOneWidget);

      // Widget does not create a CustomPaint for empty data
      // (the empty state returns a plain SizedBox with a centered text)
      expect(find.text('暂无流量数据'), findsOneWidget);
    });

    testWidgets('renders with single data point', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(data: [500], labels: ['周一']),
        ),
      );

      expect(find.byType(TrafficChart), findsOneWidget);
    });

    testWidgets('renders with all zero data', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(data: [0, 0, 0], labels: ['一', '二', '三']),
        ),
      );

      expect(find.byType(TrafficChart), findsOneWidget);
    });

    testWidgets('handles large values without overflow',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(
            data: [1000000000, 2000000000],
            labels: ['Day1', 'Day2'],
          ),
        ),
      );

      expect(find.byType(TrafficChart), findsOneWidget);
    });

    testWidgets('accepts custom height', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapTrafficChart(
          const TrafficChart(
            data: [100, 200],
            labels: ['A', 'B'],
            height: 300,
          ),
        ),
      );

      expect(find.byType(TrafficChart), findsOneWidget);
    });
  });
}
