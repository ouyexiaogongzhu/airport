import 'package:flutter/material.dart';

import '../services/vpn_service.dart';

/// A circular VPN connect/disconnect button with animated state transitions.
///
/// States:
/// - [VpnState.disconnected]: grey circle
/// - [VpnState.connecting]: spinning progress indicator
/// - [VpnState.connected]: green circle with glow
/// - [VpnState.disconnecting]: spinning with amber tint
/// - [VpnState.error]: red circle
class VpnButton extends StatefulWidget {
  final VpnState state;
  final VoidCallback? onTap;
  final double size;

  const VpnButton({
    super.key,
    required this.state,
    this.onTap,
    this.size = 120,
  });

  @override
  State<VpnButton> createState() => _VpnButtonState();
}

class _VpnButtonState extends State<VpnButton>
    with SingleTickerProviderStateMixin {
  late AnimationController _pulseController;
  late Animation<double> _pulseAnimation;

  @override
  void initState() {
    super.initState();
    _pulseController = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 1500),
    );
    _pulseAnimation = Tween<double>(begin: 1.0, end: 1.15).animate(
      CurvedAnimation(parent: _pulseController, curve: Curves.easeInOut),
    );
    _startStopPulse();
  }

  @override
  void didUpdateWidget(VpnButton oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.state != widget.state) {
      _startStopPulse();
    }
  }

  void _startStopPulse() {
    if (widget.state == VpnState.connected) {
      _pulseController.repeat(reverse: true);
    } else {
      _pulseController.stop();
      _pulseController.reset();
    }
  }

  @override
  void dispose() {
    _pulseController.dispose();
    super.dispose();
  }

  Color _backgroundColor(BuildContext context) {
    switch (widget.state) {
      case VpnState.disconnected:
        return Colors.grey;
      case VpnState.connecting:
        return Theme.of(context).colorScheme.primary.withAlpha(100);
      case VpnState.disconnecting:
        return Colors.amber.shade700;
      case VpnState.error:
        return Colors.red;
      case VpnState.connected:
        return Colors.green;
    }
  }

  IconData _icon() {
    switch (widget.state) {
      case VpnState.connected:
        return Icons.vpn_lock;
      case VpnState.connecting:
      case VpnState.disconnecting:
        return Icons.vpn_lock;
      case VpnState.disconnected:
      case VpnState.error:
        return Icons.vpn_lock_outlined;
    }
  }

  String get _labelText {
    switch (widget.state) {
      case VpnState.disconnected:
        return '已断开';
      case VpnState.connecting:
        return '连接中...';
      case VpnState.disconnecting:
        return '断开中...';
      case VpnState.connected:
        return '已连接';
      case VpnState.error:
        return '错误';
    }
  }

  Color _labelColor(BuildContext context) {
    switch (widget.state) {
      case VpnState.connected:
        return Colors.green;
      case VpnState.error:
        return Colors.red;
      case VpnState.disconnecting:
        return Colors.amber;
      case VpnState.connecting:
        return Theme.of(context).colorScheme.primary;
      case VpnState.disconnected:
        return Colors.grey;
    }
  }

  @override
  Widget build(BuildContext context) {
    final bgColor = _backgroundColor(context);

    return GestureDetector(
      onTap: widget.onTap,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          AnimatedBuilder(
            animation: _pulseAnimation,
            builder: (context, child) {
              return Transform.scale(
                scale: _pulseAnimation.value,
                child: child,
              );
            },
            child: Stack(
              alignment: Alignment.center,
              children: [
                // Glow effect for connected state
                if (widget.state == VpnState.connected)
                  Container(
                    width: widget.size + 16,
                    height: widget.size + 16,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: Colors.green.withAlpha(25),
                    ),
                  ),
                // Main circle
                Container(
                  width: widget.size,
                  height: widget.size,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: bgColor,
                    boxShadow: [
                      BoxShadow(
                        color: bgColor.withAlpha(80),
                        blurRadius: 16,
                        spreadRadius: 2,
                      ),
                    ],
                  ),
                  child: Icon(
                    _icon(),
                    size: widget.size * 0.4,
                    color: Colors.white,
                  ),
                ),
                // Loading spinner for connecting/disconnecting
                if (widget.state == VpnState.connecting ||
                    widget.state == VpnState.disconnecting)
                  SizedBox(
                    width: widget.size,
                    height: widget.size,
                    child: CircularProgressIndicator(
                      strokeWidth: 3,
                      valueColor: AlwaysStoppedAnimation<Color>(
                        widget.state == VpnState.connecting
                            ? Colors.cyanAccent.withAlpha(180)
                            : Colors.amber.withAlpha(180),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          const SizedBox(height: 16),
          // Status label
          AnimatedDefaultTextStyle(
            duration: const Duration(milliseconds: 300),
            style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                  color: _labelColor(context),
                ) ??
                TextStyle(
                  fontWeight: FontWeight.w600,
                  color: _labelColor(context),
                ),
            child: Text(_labelText),
          ),
        ],
      ),
    );
  }
}
