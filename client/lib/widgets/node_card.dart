import 'package:flutter/material.dart';

import '../models/subscription.dart';
import '../services/vpn_service.dart';
import 'status_badge.dart';
import 'traffic_bar.dart';

/// A card widget displaying a VPN node's details, including its name,
/// latency badge, an optional traffic usage bar, and a connection status
/// indicator.
///
/// Designed to be used in a list of selectable/connected nodes.
class NodeCard extends StatelessWidget {
  /// The node model providing name and URI.
  final VpnNode node;

  /// Optional latency measurement for this node.
  final NodeLatency? latency;

  /// Whether this node is currently selected.
  final bool isSelected;

  /// Whether this node is currently the active connected node.
  final bool isConnected;

  /// Called when the card is tapped.
  final VoidCallback? onTap;

  /// Called when the copy button is pressed.
  final VoidCallback? onCopy;

  /// Called when the ping/latency refresh button is pressed.
  final VoidCallback? onPing;

  /// Optional traffic usage (in bytes) to render a [TrafficBar].
  final int? trafficUsedBytes;

  /// Optional total traffic (in bytes) for the traffic bar.
  final int? trafficTotalBytes;

  const NodeCard({
    super.key,
    required this.node,
    this.latency,
    this.isSelected = false,
    this.isConnected = false,
    this.onTap,
    this.onCopy,
    this.onPing,
    this.trafficUsedBytes,
    this.trafficTotalBytes,
  });

  Color _latencyColor(int ms) {
    if (ms < 0) return Colors.grey;
    if (ms < 100) return Colors.green;
    if (ms < 300) return Colors.orange;
    return Colors.red;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final cardColor = isConnected
        ? Colors.green.withAlpha(25)
        : isSelected
            ? theme.colorScheme.primary.withAlpha(25)
            : theme.cardTheme.color;

    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: cardColor,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: isConnected
            ? BorderSide(color: Colors.green.withAlpha(120))
            : isSelected
                ? BorderSide(
                    color: theme.colorScheme.primary.withAlpha(120))
                : BorderSide.none,
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              // --- Top row: icon + name + latency badge + status ---
              Row(
                children: [
                  // Node icon with connection dot
                  Stack(
                    children: [
                      Icon(
                        Icons.public,
                        size: 28,
                        color: isConnected
                            ? Colors.green
                            : isSelected
                                ? theme.colorScheme.primary
                                : Colors.grey,
                      ),
                      if (isConnected)
                        Positioned(
                          right: -2,
                          bottom: -2,
                          child: Container(
                            width: 12,
                            height: 12,
                            decoration: const BoxDecoration(
                              color: Colors.green,
                              shape: BoxShape.circle,
                            ),
                          ),
                        ),
                    ],
                  ),
                  const SizedBox(width: 12),
                  // Node name
                  Expanded(
                    child: Text(
                      node.name,
                      style: theme.textTheme.titleSmall?.copyWith(
                        fontWeight: (isSelected || isConnected)
                            ? FontWeight.bold
                            : FontWeight.normal,
                        color: isConnected
                            ? Colors.green
                            : isSelected
                                ? theme.colorScheme.primary
                                : null,
                      ),
                    ),
                  ),
                  const SizedBox(width: 8),
                  // Latency badge
                  if (latency != null)
                    StatusBadge(
                      label: latency!.label,
                      color: _latencyColor(latency!.latencyMs),
                      showDot: latency!.isReachable,
                    ),
                  const SizedBox(width: 4),
                  // Connection status indicator
                  if (isConnected)
                    const StatusBadge(
                      label: '已连接',
                      color: Colors.green,
                      showDot: true,
                    ),
                ],
              ),
              // --- URI subtitle ---
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        node.uri,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(
                          fontSize: 11,
                          color: isConnected
                              ? Colors.green.withAlpha(180)
                              : isSelected
                                  ? theme.colorScheme.primary.withAlpha(180)
                                  : Colors.grey,
                        ),
                      ),
                    ),
                    // Ping button
                    if (onPing != null)
                      IconButton(
                        icon: Icon(
                          Icons.speed,
                          size: 18,
                          color: Colors.grey,
                        ),
                        onPressed: onPing,
                        padding: EdgeInsets.zero,
                        constraints: const BoxConstraints(),
                        tooltip: '测速',
                      ),
                    const SizedBox(width: 8),
                    // Copy button
                    if (onCopy != null)
                      IconButton(
                        icon: Icon(
                          Icons.copy,
                          size: 18,
                          color: Colors.grey,
                        ),
                        onPressed: onCopy,
                        padding: EdgeInsets.zero,
                        constraints: const BoxConstraints(),
                        tooltip: '复制',
                      ),
                  ],
                ),
              ),
              // --- Traffic bar (optional) ---
              if (trafficUsedBytes != null && trafficTotalBytes != null) ...[
                const SizedBox(height: 12),
                TrafficBar(
                  usedBytes: trafficUsedBytes!,
                  totalBytes: trafficTotalBytes!,
                  height: 10,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }
}
