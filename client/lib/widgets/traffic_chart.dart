import 'dart:math' as math;
import 'package:flutter/material.dart';

/// A simple bar chart for traffic usage built with [CustomPaint].
///
/// Takes [data] (list of byte values) and [labels] (day-of-week strings),
/// both must have the same length (typically 7).
///
/// Bars are coloured with a gradient:
///   - green  when the value is ≤ 33 % of the max
///   - yellow when the value is ≤ 66 % of the max
///   - red    when the value is > 66 % of the max
///
/// No external chart packages are used.
class TrafficChart extends StatelessWidget {
  /// Traffic byte values for each day (ordered oldest first).
  final List<int> data;

  /// Short labels for each bar (e.g. "周一", "周二", …).
  final List<String> labels;

  /// Total height of the chart widget.
  final double height;

  const TrafficChart({
    super.key,
    required this.data,
    required this.labels,
    this.height = 200,
  });

  @override
  Widget build(BuildContext context) {
    if (data.isEmpty) {
      return SizedBox(
        height: height,
        child: Center(
          child: Text(
            '暂无流量数据',
            style: TextStyle(color: Colors.grey[500]),
          ),
        ),
      );
    }

    return SizedBox(
      height: height,
      child: CustomPaint(
        painter: _BarChartPainter(
          data: data,
          labels: labels,
          barColor: Theme.of(context).colorScheme.primary,
          gridColor: Theme.of(context).colorScheme.surfaceContainerHighest,
          textColor: Theme.of(context).colorScheme.onSurface.withAlpha(180),
        ),
        size: Size.infinite,
      ),
    );
  }
}

/// Custom painter that draws the bar chart.
class _BarChartPainter extends CustomPainter {
  final List<int> data;
  final List<String> labels;
  final Color barColor;
  final Color gridColor;
  final Color textColor;

  _BarChartPainter({
    required this.data,
    required this.labels,
    required this.barColor,
    required this.gridColor,
    required this.textColor,
  });

  @override
  void paint(Canvas canvas, Size size) {
    if (data.isEmpty) return;

    final maxValue = data.reduce(math.max).toDouble();
    final effectiveMax = maxValue > 0 ? maxValue : 1.0;

    // Layout constants
    const leftMargin = 48.0; // space for Y-axis labels
    const rightMargin = 8.0;
    const topMargin = 12.0;
    const bottomMargin = 28.0; // space for X-axis labels

    final chartWidth = size.width - leftMargin - rightMargin;
    final chartHeight = size.height - topMargin - bottomMargin;

    if (chartWidth <= 0 || chartHeight <= 0) return;

    final barCount = data.length;
    final barAreaWidth = chartWidth / barCount;
    const barWidthRatio = 0.55;
    final barWidth = barAreaWidth * barWidthRatio;

    // --- Draw horizontal grid lines + Y-axis labels ---
    const gridLineCount = 4;
    final gridPaint = Paint()
      ..color = gridColor.withAlpha(60)
      ..strokeWidth = 0.5;

    final labelStyle = TextStyle(
      color: textColor,
      fontSize: 10,
      fontWeight: FontWeight.w400,
    );

    for (int i = 0; i <= gridLineCount; i++) {
      final y = topMargin + chartHeight * (1.0 - i / gridLineCount);

      // Grid line
      canvas.drawLine(
        Offset(leftMargin, y),
        Offset(size.width - rightMargin, y),
        gridPaint,
      );

      // Y-axis label
      final labelValue = (effectiveMax * i / gridLineCount).round();
      final labelText = _formatChartY(labelValue);
      final textSpan = TextSpan(text: labelText, style: labelStyle);
      final textPainter = TextPainter(
        text: textSpan,
        textDirection: TextDirection.ltr,
      )..layout(maxWidth: leftMargin - 6);

      textPainter.paint(
        canvas,
        Offset(leftMargin - textPainter.width - 4, y - textPainter.height / 2),
      );
    }

    // --- Draw bars ---
    final barRadius = Radius.circular(math.min(barWidth / 2, 4.0));

    for (int i = 0; i < barCount; i++) {
      final value = data[i].toDouble();
      final barHeight = (value / effectiveMax) * chartHeight;

      final x = leftMargin + barAreaWidth * i + (barAreaWidth - barWidth) / 2;
      final y = topMargin + chartHeight - barHeight;

      final fraction = value / effectiveMax;
      final barColor = _barColor(fraction);
      final barPaint = Paint()..color = barColor;

      final rrect = RRect.fromRectAndCorners(
        Rect.fromLTWH(x, y, barWidth, barHeight),
        topLeft: barRadius,
        topRight: barRadius,
      );
      canvas.drawRRect(rrect, barPaint);
    }

    // --- Draw X-axis labels ---
    for (int i = 0; i < barCount; i++) {
      final x = leftMargin + barAreaWidth * i + barAreaWidth / 2;
      final y = size.height - bottomMargin + 8;

      final textSpan = TextSpan(
        text: i < labels.length ? labels[i] : '',
        style: labelStyle.copyWith(fontSize: 11),
      );
      final textPainter = TextPainter(
        text: textSpan,
        textDirection: TextDirection.ltr,
      )..layout();

      textPainter.paint(
        canvas,
        Offset(x - textPainter.width / 2, y),
      );
    }
  }

  /// Determine bar colour based on usage fraction (0..1).
  Color _barColor(double fraction) {
    if (fraction > 0.66) {
      return Colors.redAccent;
    } else if (fraction > 0.33) {
      return Colors.amber;
    } else {
      return Colors.greenAccent;
    }
  }

  /// Format a byte value for the Y-axis label (simplified).
  String _formatChartY(int bytes) {
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(0)} KB';
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(0)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(1)} GB';
  }

  @override
  bool shouldRepaint(covariant _BarChartPainter oldDelegate) {
    return data != oldDelegate.data || labels != oldDelegate.labels;
  }
}
