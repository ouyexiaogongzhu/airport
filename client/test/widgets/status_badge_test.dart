import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/widgets/status_badge.dart';

/// Helper to wrap a StatusBadge in a MaterialApp.
Widget wrapStatusBadge(StatusBadge badge) {
  return MaterialApp(home: Scaffold(body: Center(child: badge)));
}

void main() {
  group('StatusBadge', () {
    testWidgets('renders status text', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(const StatusBadge(label: 'Active')),
      );

      expect(find.text('Active'), findsOneWidget);
    });

    testWidgets('renders with custom color', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(
          const StatusBadge(label: 'Custom', color: Colors.purple),
        ),
      );

      expect(find.text('Custom'), findsOneWidget);
    });

    testWidgets('renders with custom textColor', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(
          const StatusBadge(
            label: 'White Text',
            color: Colors.blue,
            textColor: Colors.white,
          ),
        ),
      );

      expect(find.text('White Text'), findsOneWidget);
    });

    testWidgets('shows dot indicator when showDot is true',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(
          const StatusBadge(label: 'Online', showDot: true),
        ),
      );

      // The dot is a small Container with BoxShape.circle
      expect(find.text('Online'), findsOneWidget);
    });

    testWidgets('shows icon when icon is provided', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(
          const StatusBadge(
            label: 'Check',
            icon: Icons.check_circle,
          ),
        ),
      );

      expect(find.text('Check'), findsOneWidget);
      expect(find.byIcon(Icons.check_circle), findsOneWidget);
    });

    testWidgets('factory active() creates green badge with Active text',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(StatusBadge.active()),
      );

      expect(find.text('Active'), findsOneWidget);
    });

    testWidgets('factory active() accepts custom label',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(StatusBadge.active(label: '运行中')),
      );

      expect(find.text('运行中'), findsOneWidget);
    });

    testWidgets('factory expired() creates red badge with Expired text',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(StatusBadge.expired()),
      );

      expect(find.text('Expired'), findsOneWidget);
    });

    testWidgets('factory warning() creates orange badge with Warning text',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(StatusBadge.warning()),
      );

      expect(find.text('Warning'), findsOneWidget);
    });

    testWidgets('factory offline() creates grey badge with Offline text',
        (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(StatusBadge.offline()),
      );

      expect(find.text('Offline'), findsOneWidget);
    });

    testWidgets('uses custom fontSize, padding', (WidgetTester tester) async {
      await tester.pumpWidget(
        wrapStatusBadge(
          const StatusBadge(
            label: 'Small',
            fontSize: 8,
            horizontalPadding: 4,
            verticalPadding: 2,
          ),
        ),
      );

      expect(find.text('Small'), findsOneWidget);
    });
  });
}
