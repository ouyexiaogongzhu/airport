import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/models/subscription.dart';
import 'package:rfplay_client/services/vpn_service.dart';
import 'package:rfplay_client/widgets/node_card.dart';
import 'package:rfplay_client/widgets/status_badge.dart';
import 'package:rfplay_client/widgets/traffic_bar.dart';

/// Helper to wrap a NodeCard in a MaterialApp.
Widget wrapNodeCard(NodeCard card) {
  return MaterialApp(home: Scaffold(body: SingleChildScrollView(child: card)));
}

void main() {
  final sampleNode = VpnNode(name: 'Tokyo-01', uri: 'vmess://example.com:443');
  final unreachableLatency = NodeLatency(
    nodeName: 'Tokyo-01',
    latencyMs: -1,
    measuredAt: DateTime.now(),
  );
  final fastLatency = NodeLatency(
    nodeName: 'Tokyo-01',
    latencyMs: 45,
    measuredAt: DateTime.now(),
  );
  final mediumLatency = NodeLatency(
    nodeName: 'Tokyo-01',
    latencyMs: 150,
    measuredAt: DateTime.now(),
  );
  final slowLatency = NodeLatency(
    nodeName: 'Tokyo-01',
    latencyMs: 350,
    measuredAt: DateTime.now(),
  );

  group('NodeCard', () {
    testWidgets('renders node name', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(NodeCard(node: sampleNode)),
      );

      expect(find.text('Tokyo-01'), findsOneWidget);
      expect(find.text('vmess://example.com:443'), findsOneWidget);
    });

    testWidgets('shows connection status when isConnected is true',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, isConnected: true),
        ),
      );

      // Should show the "已连接" status badge
      expect(find.text('已连接'), findsWidgets);
    });

    testWidgets('shows selection highlight when isSelected is true',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, isSelected: true),
        ),
      );

      // Widget renders without errors
      expect(find.byType(NodeCard), findsOneWidget);
    });

    testWidgets('renders latency badge with fast latency (<100ms)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, latency: fastLatency),
        ),
      );

      // Latency label should be "45ms"
      expect(find.text('45ms'), findsOneWidget);
    });

    testWidgets('renders latency badge with medium latency (100-300ms)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, latency: mediumLatency),
        ),
      );

      expect(find.text('150ms'), findsOneWidget);
    });

    testWidgets('renders latency badge with slow latency (>300ms)',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, latency: slowLatency),
        ),
      );

      expect(find.text('350ms'), findsOneWidget);
    });

    testWidgets('renders latency badge with unreachable node',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, latency: unreachableLatency),
        ),
      );

      expect(find.text('超时'), findsOneWidget);
    });

    testWidgets('shows traffic bar when traffic bytes are provided',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(
            node: sampleNode,
            trafficUsedBytes: 500_000_000,
            trafficTotalBytes: 10_000_000_000,
          ),
        ),
      );

      // TrafficBar should be present
      expect(find.byType(TrafficBar), findsOneWidget);
    });

    testWidgets('does not show traffic bar when traffic bytes are null',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(NodeCard(node: sampleNode)),
      );

      expect(find.byType(TrafficBar), findsNothing);
    });

    testWidgets('fires onTap callback when tapped',
        (WidgetTester tester) async {
      int tapCount = 0;

      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(node: sampleNode, onTap: () => tapCount++),
        ),
      );

      await tester.tap(find.byType(InkWell));
      expect(tapCount, equals(1));
    });

    testWidgets('fires onPing and onCopy callbacks',
        (WidgetTester tester) async {
      int pingCount = 0;
      int copyCount = 0;

      await tester.pumpWidget(
        wrapNodeCard(
          NodeCard(
            node: sampleNode,
            onPing: () => pingCount++,
            onCopy: () => copyCount++,
          ),
        ),
      );

      // Tap the ping button (speed icon)
      await tester.tap(find.byIcon(Icons.speed));
      expect(pingCount, equals(1));

      // Tap the copy button
      await tester.tap(find.byIcon(Icons.copy));
      expect(copyCount, equals(1));
    });

    testWidgets('does not show ping/copy buttons when callbacks are null',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapNodeCard(NodeCard(node: sampleNode)),
      );

      expect(find.byIcon(Icons.speed), findsNothing);
      expect(find.byIcon(Icons.copy), findsNothing);
    });
  });
}
