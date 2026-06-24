/// Traffic utility helpers for formatting and computing byte values.
///
/// All functions use binary units (1 KB = 1024 B, 1 MB = 1024 KB, etc.).
library;

/// Format a byte count to a human-readable string with appropriate unit.
///
/// Examples:
///   `formatBytes(500)`       → `"500 B"`
///   `formatBytes(2048)`      → `"2.00 KB"`
///   `formatBytes(5_242_880)` → `"5.00 MB"`
///   `formatBytes(1_073_741_824)` → `"1.00 GB"`
///
/// [bytes] may be negative, in which case it is treated as zero.
String formatBytes(int bytes) {
  if (bytes <= 0) return '0 B';
  if (bytes < 1024) return '$bytes B';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  double value = bytes.toDouble();
  int unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }

  if (unitIndex == 0) return '$bytes B';
  return '${value.toStringAsFixed(2)} ${units[unitIndex]}';
}

/// Format bytes to gigabytes with [decimals] places.
///
/// Returns `"0.00"` for null/negative values.
String formatGb(int bytes, {int decimals = 2}) {
  if (bytes <= 0) return '0.${'0' * decimals}';
  final gb = bytes / (1024 * 1024 * 1024);
  return gb.toStringAsFixed(decimals);
}

/// Format bytes as a download/upload label with unit and rate suffix.
///
/// Example: `"15.32 MB/s"`, `"2.10 KB/s"`
String formatSpeed(double bytesPerSecond) {
  if (bytesPerSecond <= 0) return '0 B/s';
  if (bytesPerSecond < 1024) return '${bytesPerSecond.toStringAsFixed(1)} B/s';
  if (bytesPerSecond < 1024 * 1024) {
    return '${(bytesPerSecond / 1024).toStringAsFixed(1)} KB/s';
  }
  if (bytesPerSecond < 1024 * 1024 * 1024) {
    return '${(bytesPerSecond / (1024 * 1024)).toStringAsFixed(2)} MB/s';
  }
  return '${(bytesPerSecond / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB/s';
}

/// Get a short day-of-week label from a [DateTime].
///
/// Returns Chinese abbreviations: 周一, 周二, ..., 周日.
String dayLabel(DateTime dt) {
  const labels = ['周一', '周二', '周三', '周四', '周五', '周六', '周日'];
  // DateTime.weekday: 1=Monday ... 7=Sunday
  return labels[dt.weekday - 1];
}

/// Generate 7 daily traffic data entries ending at [today].
///
/// Returns a list of `(DateTime date, int bytes)` sorted oldest first.
/// Data is randomly perturbed around `[baseBytes]` to simulate realistic
/// daily variation. In production, this would come from the API.
List<(DateTime, int)> generateDailyTraffic({
  required DateTime today,
  required int totalUsedBytes,
  int days = 7,
}) {
  final results = <(DateTime, int)>[];
  // Rough daily average from total used
  final avg = (totalUsedBytes / days).round();
  // We'll use a pseudo-random distribution based on totalUsedBytes
  final seed = totalUsedBytes.isEven ? 1 : -1;

  for (int i = days - 1; i >= 0; i--) {
    final date = today.subtract(Duration(days: i));
    // Vary around the average with a deterministic spread
    final variation = ((seed * (i + 1) * 37) % 101 - 50); // -50..+50 percent
    final dailyBytes = (avg * (1.0 + variation / 100.0)).round().clamp(0, totalUsedBytes * 2);
    results.add((date, dailyBytes > 0 ? dailyBytes : avg ~/ 4));
  }

  return results;
}
