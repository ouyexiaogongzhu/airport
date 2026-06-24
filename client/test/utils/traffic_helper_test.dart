import 'package:flutter_test/flutter_test.dart';

import 'package:rfplay_client/utils/traffic_helper.dart';

void main() {
  group('formatBytes', () {
    test('returns "0 B" for zero bytes', () {
      expect(formatBytes(0), equals('0 B'));
    });

    test('returns "0 B" for negative bytes', () {
      // Negative is treated as 0 per the docs
      expect(formatBytes(-100), equals('0 B'));
    });

    test('returns bytes without unit suffix for < 1024', () {
      expect(formatBytes(500), equals('500 B'));
    });

    test('returns "1.00 KB" for 1024 bytes', () {
      expect(formatBytes(1024), equals('1.00 KB'));
    });

    test('returns "1.00 MB" for 1048576 bytes', () {
      expect(formatBytes(1048576), equals('1.00 MB'));
    });

    test('returns "1.00 GB" for 1073741824 bytes', () {
      expect(formatBytes(1073741824), equals('1.00 GB'));
    });

    test('returns "2.50 MB" for 2621440 bytes', () {
      expect(formatBytes(2621440), equals('2.50 MB'));
    });

    test('handles large values in TB range', () {
      // 2 TB = 2 * 1024 * 1024 * 1024 * 1024 = 2199023255552
      expect(formatBytes(2199023255552), equals('2.00 TB'));
    });
  });

  group('formatGb', () {
    test('returns "0.00" for zero bytes', () {
      expect(formatGb(0), equals('0.00'));
    });

    test('returns "0.00" for negative bytes', () {
      expect(formatGb(-1), equals('0.00'));
    });

    test('converts 1073741824 bytes to "1.00"', () {
      expect(formatGb(1073741824), equals('1.00'));
    });

    test('converts 5368709120 bytes to "5.00"', () {
      // 5 * 1024^3 = 5368709120
      expect(formatGb(5368709120), equals('5.00'));
    });

    test('respects decimals parameter', () {
      // 1536000000 bytes ≈ 1.4305 GB
      // with 3 decimals: "1.430"
      const gb = 1536000000;
      expect(formatGb(gb, decimals: 3), equals('1.431'));
    });
  });

  group('formatSpeed', () {
    test('returns "0 B/s" for zero or negative', () {
      expect(formatSpeed(0), equals('0 B/s'));
      expect(formatSpeed(-5), equals('0 B/s'));
    });

    test('returns B/s for values < 1024', () {
      expect(formatSpeed(500), equals('500.0 B/s'));
    });

    test('returns KB/s for values in KB range', () {
      expect(formatSpeed(2048), equals('2.0 KB/s'));
    });

    test('returns MB/s for values in MB range', () {
      expect(formatSpeed(5242880), equals('5.00 MB/s')); // 5 * 1024 * 1024
    });

    test('returns GB/s for large values', () {
      expect(formatSpeed(10737418240), equals('10.00 GB/s')); // 10 * 1024^3
    });
  });

  group('dayLabel', () {
    test('returns "周一" for Monday', () {
      final monday = DateTime(2024, 1, 1); // Jan 1, 2024 is Monday
      expect(dayLabel(monday), equals('周一'));
    });

    test('returns "周日" for Sunday', () {
      final sunday = DateTime(2024, 1, 7); // Jan 7, 2024 is Sunday
      expect(dayLabel(sunday), equals('周日'));
    });

    test('returns correct label for Wednesday', () {
      final wednesday = DateTime(2024, 1, 3); // Jan 3, 2024 is Wednesday
      expect(dayLabel(wednesday), equals('周三'));
    });

    test('returns correct label for Saturday', () {
      final saturday = DateTime(2024, 1, 6); // Jan 6, 2024 is Saturday
      expect(dayLabel(saturday), equals('周六'));
    });
  });

  group('generateDailyTraffic', () {
    test('returns 7 entries by default', () {
      final result = generateDailyTraffic(
        today: DateTime(2024, 6, 15),
        totalUsedBytes: 10_000_000_000,
      );
      expect(result.length, equals(7));
    });

    test('returns entries sorted oldest first', () {
      final result = generateDailyTraffic(
        today: DateTime(2024, 6, 15),
        totalUsedBytes: 10_000_000_000,
      );
      for (int i = 1; i < result.length; i++) {
        expect(result[i].$1.isAfter(result[i - 1].$1), isTrue);
      }
    });

    test('last entry is the today date', () {
      final today = DateTime(2024, 6, 15);
      final result = generateDailyTraffic(
        today: today,
        totalUsedBytes: 10_000_000_000,
      );
      expect(result.last.$1, equals(today));
    });

    test('handles zero totalUsedBytes without crash', () {
      final result = generateDailyTraffic(
        today: DateTime(2024, 6, 15),
        totalUsedBytes: 0,
      );
      expect(result.length, equals(7));
    });
  });
}
