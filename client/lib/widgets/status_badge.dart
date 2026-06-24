import 'package:flutter/material.dart';

/// A small colored badge for displaying subscription status, connection status,
/// or any short label that needs a visual indicator.
///
/// The badge renders as a rounded pill with an optional dot indicator.
class StatusBadge extends StatelessWidget {
  /// The short text label (e.g. "Active", "Expired", "Connected").
  final String label;

  /// The badge colour. Defaults to the primary colour if null.
  final Color? color;

  /// Text colour. Defaults to [color] for a monochrome look.
  final Color? textColor;

  /// Whether to show a small dot indicator before the label.
  final bool showDot;

  /// Optional icon shown before the label (replaces the dot if both given).
  final IconData? icon;

  /// Font size for the label (defaults to 11).
  final double fontSize;

  /// Horizontal padding inside the badge (defaults to 10).
  final double horizontalPadding;

  /// Vertical padding inside the badge (defaults to 4).
  final double verticalPadding;

  const StatusBadge({
    super.key,
    required this.label,
    this.color,
    this.textColor,
    this.showDot = false,
    this.icon,
    this.fontSize = 11,
    this.horizontalPadding = 10,
    this.verticalPadding = 4,
  });

  /// Create a green "Active" badge.
  factory StatusBadge.active({String label = 'Active'}) =>
      StatusBadge(label: label, color: Colors.green, showDot: true);

  /// Create a red "Expired" badge.
  factory StatusBadge.expired({String label = 'Expired'}) =>
      StatusBadge(label: label, color: Colors.red, showDot: true);

  /// Create an orange "Warning" badge.
  factory StatusBadge.warning({String label = 'Warning'}) =>
      StatusBadge(label: label, color: Colors.orange, showDot: true);

  /// Create a grey "Offline" badge.
  factory StatusBadge.offline({String label = 'Offline'}) =>
      StatusBadge(label: label, color: Colors.grey, showDot: true);

  @override
  Widget build(BuildContext context) {
    final effectiveColor =
        color ?? Theme.of(context).colorScheme.primary;
    final effectiveTextColor =
        textColor ?? effectiveColor;

    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: horizontalPadding,
        vertical: verticalPadding,
      ),
      decoration: BoxDecoration(
        color: effectiveColor.withAlpha(30),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: effectiveColor.withAlpha(80),
          width: 1,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          if (icon != null) ...[
            Icon(
              icon,
              size: fontSize + 2,
              color: effectiveTextColor,
            ),
            const SizedBox(width: 4),
          ] else if (showDot) ...[
            Container(
              width: 6,
              height: 6,
              decoration: BoxDecoration(
                color: effectiveTextColor,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: 4),
          ],
          Text(
            label,
            style: TextStyle(
              fontSize: fontSize,
              fontWeight: FontWeight.w500,
              color: effectiveTextColor,
            ),
          ),
        ],
      ),
    );
  }
}
