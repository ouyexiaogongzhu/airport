import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../../models/subscription.dart';
import '../../services/subscription_service.dart';
import '../../services/vpn_service.dart';
import '../../widgets/loading_overlay.dart';
import '../../widgets/node_card.dart';
import '../../widgets/status_badge.dart';
import '../../widgets/traffic_bar.dart';

class HomeDashboard extends StatefulWidget {
  const HomeDashboard({super.key});

  @override
  State<HomeDashboard> createState() => _HomeDashboardState();
}

// TODO(M4): _statusBadge, _formatTrafficGb, traffic bar, and node card sections
// are duplicated across HomeDashboard, DashboardScreen, and AccountSubscription.
// Extract shared widgets and utility functions to reduce duplication.
class _HomeDashboardState extends State<HomeDashboard> {
  // TODO(M7): Traffic estimate is hardcoded at 100 GB. Fetch the actual plan
  // limit from the API or subscription config instead.
  static const int _trafficEstimateBytes = 100 * 1024 * 1024 * 1024;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final sub = context.read<SubscriptionService>();
      if (sub.subscription == null && sub.statusError == null && !sub.isLoading) {
        sub.loadSubscription();
      }
    });
  }

  Future<void> _refresh() async {
    final subService = context.read<SubscriptionService>();
    await Future.wait([
      subService.loadConfig(),
      subService.loadSubscription(),
    ]);
  }

  StatusBadge _statusBadge(String? status) {
    switch (status) {
      case null:
      case 'SUBSCRIPTION_ACTIVE':
        return StatusBadge.active(label: '已激活');
      case 'SUBSCRIPTION_PENDING':
        return StatusBadge.warning(label: '待订阅');
      case 'SUBSCRIPTION_EXPIRED':
        return StatusBadge.expired(label: '已过期');
      default:
        return StatusBadge(label: status, color: Colors.grey, showDot: true);
    }
  }

  String _formatTrafficGb(double gb) {
    if (gb >= 100) return gb.toStringAsFixed(0);
    if (gb >= 10) return gb.toStringAsFixed(1);
    return gb.toStringAsFixed(2);
  }

  void _copyToClipboard(String text) {
    Clipboard.setData(ClipboardData(text: text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已复制到剪贴板')),
    );
  }

  // BUG: Ping always targets 8.8.8.8:53 instead of the actual node address.
  // Should use node.host / node.port to measure real latency to each node.
  Future<void> _pingNode(VpnService vpn, VpnNode node) async {
    await vpn.pingNode(node.name, '8.8.8.8', 53);
  }

  @override
  Widget build(BuildContext context) {
    final vpn = context.watch<VpnService>();

    return Stack(
      children: [
        RefreshIndicator(
          onRefresh: _refresh,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                '首页概览',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 8),
              Text(
                '订阅状态、流量与节点预览',
                style: TextStyle(color: Colors.grey[400]),
              ),
              const SizedBox(height: 24),
              Consumer<SubscriptionService>(
                builder: (context, subService, _) {
                  final sub = subService.subscription;
                  final statusError = subService.statusError;
                  final previewNodes = (sub?.nodes ?? []).take(3).toList();

                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
                          child: Row(
                            children: [
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      '订阅状态',
                                      style: TextStyle(color: Colors.grey[400], fontSize: 13),
                                    ),
                                    const SizedBox(height: 8),
                                    _statusBadge(statusError),
                                  ],
                                ),
                              ),
                              if (sub != null)
                                Column(
                                  crossAxisAlignment: CrossAxisAlignment.end,
                                  children: [
                                    Text(
                                      '套餐',
                                      style: TextStyle(color: Colors.grey[400], fontSize: 13),
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      sub.tierLabel,
                                      style: const TextStyle(
                                        fontSize: 16,
                                        fontWeight: FontWeight.bold,
                                      ),
                                    ),
                                  ],
                                ),
                            ],
                          ),
                        ),
                      ),
                      if (sub != null) ...[
                        const SizedBox(height: 12),
                        Card(
                          child: Padding(
                            padding: const EdgeInsets.all(16),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                const Text(
                                  '流量使用',
                                  style: TextStyle(
                                    fontSize: 16,
                                    fontWeight: FontWeight.bold,
                                  ),
                                ),
                                const SizedBox(height: 12),
                                TrafficBar(
                                  usedBytes: (_trafficEstimateBytes - sub.trafficRemainingBytes)
                                      .clamp(0, _trafficEstimateBytes),
                                  totalBytes: _trafficEstimateBytes,
                                  label:
                                      '剩余 ${_formatTrafficGb(sub.trafficRemainingBytes / (1024 * 1024 * 1024))} GB',
                                ),
                              ],
                            ),
                          ),
                        ),
                        const SizedBox(height: 12),
                        Row(
                          mainAxisAlignment: MainAxisAlignment.spaceBetween,
                          children: [
                            Text(
                              '节点预览',
                              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                    fontWeight: FontWeight.bold,
                                  ),
                            ),
                            Text(
                              '到期 ${sub.expireDateFormatted}',
                              style: TextStyle(color: Colors.grey[500], fontSize: 12),
                            ),
                          ],
                        ),
                        const SizedBox(height: 8),
                        if (previewNodes.isEmpty)
                          Padding(
                            padding: const EdgeInsets.all(16),
                            child: Text(
                              '暂无可用节点',
                              style: TextStyle(color: Colors.grey[500]),
                            ),
                          )
                        else
                          ...previewNodes.map(
                            (node) => NodeCard(
                              node: node,
                              latency: vpn.latencies[node.name],
                              isSelected: vpn.selectedNode == node.name,
                              isConnected: vpn.connectedNode == node.name && vpn.isConnected,
                              onTap: () => vpn.selectNode(node.name),
                              onPing: () => _pingNode(vpn, node),
                              onCopy: () => _copyToClipboard(node.uri),
                            ),
                          ),
                      ] else if (statusError == 'SUBSCRIPTION_PENDING' ||
                          statusError == 'SUBSCRIPTION_EXPIRED') ...[
                        const SizedBox(height: 12),
                        Card(
                          child: Padding(
                            padding: const EdgeInsets.all(16),
                            child: Text(
                              statusError == 'SUBSCRIPTION_PENDING'
                                  ? '您尚未购买订阅，请前往官网选择套餐。'
                                  : '您的订阅已过期，请续费后继续使用。',
                              style: TextStyle(color: Colors.grey[300]),
                            ),
                          ),
                        ),
                      ],
                    ],
                  );
                },
              ),
            ],
          ),
        ),
        Consumer<SubscriptionService>(
          builder: (context, subService, _) {
            return LoadingOverlay(
              isLoading: subService.isLoading &&
                  subService.subscription == null &&
                  subService.statusError == null,
              message: '加载中...',
            );
          },
        ),
      ],
    );
  }
}
