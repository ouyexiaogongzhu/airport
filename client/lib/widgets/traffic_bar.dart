import 'package:flutter/material.dart';

/// A horizontal bar showing used/total traffic with a color gradient and
/// percentage label.
///
/// [usedBytes] and [totalBytes] control the bar fill proportion.
/// When [totalBytes] is zero, the bar appears empty.
class TrafficBar extends StatelessWidget {
  /// Bytes consumed so far.
  final int usedBytes;

  /// Total bytes allocated. When zero, the bar shows 0%.
  final int totalBytes;

  /// Optional custom height (defaults to 12).
  final double height;

  /// Optional label text shown below the bar (e.g. "2.34 GB / 10 GB").
  final String? label;

  const TrafficBar({
    super.key,
    required this.usedBytes,
    required this.totalBytes,
    this.height = 12,
    this.label,
  });

  /// Fraction of used traffic from 0.0 to 1.0.
  double get _fraction {
    if (totalBytes <= 0) return 0.0;
    return (usedBytes / totalBytes).clamp(0.0, 1.0);
  }

  /// Determine bar colour based on usage percentage.
  Color _barColor(BuildContext context) {
    final fraction = _fraction;
    if (fraction >= 0.9) return Colors.red;
    if (fraction >= 0.7) return Colors.orange;
    if (fraction >= 0.4) return Colors.amber;
    return Theme.of(context).colorScheme.primary;
  }

  String get _percentageLabel {
    return '${(_fraction * 100).toStringAsFixed(0)}%';
  }

  /// Format bytes to a human-readable string (GB with 2 decimals).
  static String bytesToGb(int bytes) {
    final gb = bytes / (1024 * 1024 * 1024);
    return '${gb.toStringAsFixed(2)} GB';
  }

  @override
  Widget build(BuildContext context) {
    final fraction = _fraction;
    final barColor = _barColor(context);
    final usedLabel = bytesToGb(usedBytes);
    final totalLabel = bytesToGb(totalBytes);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        // Percentage + optional custom label row
        Row(
          children: [
            Text(
              _percentageLabel,
              style: TextStyle(
                fontSize: 12,
                fontWeight: FontWeight.w600,
                color: barColor,
              ),
            ),
            const Spacer(),
            if (label != null)
              Flexible(
                child: Text(
                  label!,
                  style: TextStyle(
                    fontSize: 11,
                    color: Theme.of(context).colorScheme.onSurface.withAlpha(180),
                  ),
                  overflow: TextOverflow.ellipsis,
                ),
              ),
          ],
        ),
        const SizedBox(height: 4),
        // Bar track
        ClipRRect(
          borderRadius: BorderRadius.circular(height / 2),
          child: SizedBox(
            height: height,
            child: LinearProgressIndicator(
              value: fraction,
              backgroundColor:
                  Theme.of(context).colorScheme.surfaceContainerHighest,
              valueColor: AlwaysStoppedAnimation<Color>(barColor),
            ),
          ),
        ),
        const SizedBox(height: 4),
        // Used / Total label
        Row(
          children: [
            Text(
              usedLabel,
              style: TextStyle(
                fontSize: 10,
                color: Theme.of(context).colorScheme.onSurface.withAlpha(150),
              ),
            ),
            const Spacer(),
            Text(
              totalLabel,
              style: TextStyle(
                fontSize: 10,
                color: Theme.of(context).colorScheme.onSurface.withAlpha(150),
              ),
            ),
          ],
        ),
      ],
    );
  }
}
