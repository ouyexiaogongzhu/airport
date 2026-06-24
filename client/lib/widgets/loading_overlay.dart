import 'package:flutter/material.dart';

/// A full-screen loading overlay that sits on top of current content.
///
/// Optionally shows a [message] below the spinner. The overlay is
/// semi-transparent so the content underneath is still visible.
///
/// Usage:
/// ```dart
/// Stack(
///   children: [
///     // Your content ...
///     if (isLoading)
///       const LoadingOverlay(message: '加载中...'),
///   ],
/// )
/// ```
class LoadingOverlay extends StatelessWidget {
  /// Optional message displayed below the circular progress indicator.
  final String? message;

  /// Background opacity (defaults to 0.5).
  final double opacity;

  /// Whether to show the overlay (defaults to true).
  final bool isLoading;

  /// The progress indicator widget. Defaults to [CircularProgressIndicator].
  final Widget? indicator;

  const LoadingOverlay({
    super.key,
    this.message,
    this.opacity = 0.5,
    this.isLoading = true,
    this.indicator,
  });

  @override
  Widget build(BuildContext context) {
    if (!isLoading) return const SizedBox.shrink();

    return Positioned.fill(
      child: Container(
        color: Theme.of(context).scaffoldBackgroundColor.withAlpha(
              (opacity * 255).round(),
            ),
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              indicator ??
                  CircularProgressIndicator(
                    color: Theme.of(context).colorScheme.primary,
                  ),
              if (message != null) ...[
                const SizedBox(height: 16),
                Text(
                  message!,
                  style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: Theme.of(context)
                            .colorScheme
                            .onSurface
                            .withAlpha(200),
                      ) ??
                      TextStyle(
                        color: Theme.of(context)
                            .colorScheme
                            .onSurface
                            .withAlpha(200),
                      ),
                  textAlign: TextAlign.center,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
