import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../models/subscription.dart';
import '../services/subscription_service.dart';
import '../services/vpn_service.dart';
import '../widgets/loading_overlay.dart';
import '../widgets/node_card.dart';
import '../widgets/vpn_button.dart';

class VpnScreen extends StatefulWidget {
  const VpnScreen({super.key});

  @override
  State<VpnScreen> createState() => _VpnScreenState();
}

class _VpnScreenState extends State<VpnScreen> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final sub = context.read<SubscriptionService>();
      if (sub.subscription == null && !sub.isLoading) {
        sub.loadSubscription();
      }
    });
  }

  Future<void> _refresh() async {
    final subService = context.read<SubscriptionService>();
    final vpn = context.read<VpnService>();
    await subService.loadSubscription();
    if (subService.subscription != null) {
      await vpn.pingAllNodes(subService.subscription!);
    }
  }

  String _formatDuration(Duration duration) {
    final h = duration.inHours.toString().padLeft(2, '0');
    final m = duration.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = duration.inSeconds.remainder(60).toString().padLeft(2, '0');
    if (duration.inHours > 0) return '$h:$m:$s';
    return '$m:$s';
  }

  @override
  Widget build(BuildContext context) {
    final vpn = context.watch<VpnService>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN 控制'),
      ),
      body: Stack(
        children: [
          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final subscription = subService.subscription;
              final nodes = subscription?.nodes ?? [];

              return RefreshIndicator(
                onRefresh: _refresh,
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    _buildConnectionCard(vpn, subscription),
                    const SizedBox(height: 16),
                    Row(
                      children: [
                        Expanded(
                          child: _SpeedCard(
                            title: '下载',
                            speed: vpn.downloadSpeed,
                            icon: Icons.arrow_downward,
                            color: Colors.green,
                          ),
                        ),
                        const SizedBox(width: 12),
                        Expanded(
                          child: _SpeedCard(
                            title: '上传',
                            speed: vpn.uploadSpeed,
                            icon: Icons.arrow_upward,
                            color: Colors.blue,
                          ),
                        ),
                      ],
                    ),
                    const SizedBox(height: 24),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          '可用节点',
                          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                        ),
                        if (nodes.isNotEmpty)
                          TextButton.icon(
                            onPressed: () => _refresh(),
                            icon: const Icon(Icons.refresh, size: 16),
                            label: const Text('测速'),
                          ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    if (nodes.isEmpty)
                      Center(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Text(
                            subService.isLoading
                                ? ''
                                : '暂无可用节点\n请先购买套餐',
                            textAlign: TextAlign.center,
                            style: TextStyle(color: Colors.grey[500]),
                          ),
                        ),
                      )
                    else
                      ...nodes.map(
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
                    const SizedBox(height: 16),
                    if (vpn.isConnected && vpn.connectedNode != null)
                      Center(
                        child: Text(
                          '已连接至: ${vpn.connectedNode}',
                          style: TextStyle(color: Colors.grey[400], fontSize: 12),
                        ),
                      ),
                    const SizedBox(height: 8),
                    if (vpn.errorMessage != null)
                      Card(
                        color: Colors.red.withAlpha(15),
                        shape: RoundedRectangleBorder(
                          borderRadius: BorderRadius.circular(8),
                          side: BorderSide(color: Colors.red.withAlpha(60)),
                        ),
                        child: Padding(
                          padding: const EdgeInsets.all(12),
                          child: Row(
                            children: [
                              Icon(Icons.error_outline, color: Colors.red[300], size: 18),
                              const SizedBox(width: 8),
                              Expanded(
                                child: Text(
                                  vpn.errorMessage!,
                                  style: TextStyle(color: Colors.red[300], fontSize: 13),
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                  ],
                ),
              );
            },
          ),
          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final nodes = subService.subscription?.nodes ?? [];
              return LoadingOverlay(
                isLoading: subService.isLoading && nodes.isEmpty,
                message: '加载节点...',
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildConnectionCard(VpnService vpn, SubscriptionInfo? subscription) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 16),
        child: Column(
          children: [
            VpnButton(
              state: vpn.state,
              onTap: () => vpn.toggle(subscription: subscription),
              errorMessage: vpn.errorMessage,
            ),
            if (vpn.isConnected) ...[
              const SizedBox(height: 4),
              Text(
                '连接时长: ${_formatDuration(vpn.connectedTime)}',
                style: TextStyle(color: Colors.grey[400], fontSize: 13),
              ),
            ],
          ],
        ),
      ),
    );
  }

  // BUG: Ping always targets 8.8.8.8:53 instead of the actual node address.
  // Should use node.host / node.port to measure real latency to each node.
  Future<void> _pingNode(VpnService vpn, VpnNode node) async {
    await vpn.pingNode(node.name, '8.8.8.8', 53);
  }

  void _copyToClipboard(String text) {
    Clipboard.setData(ClipboardData(text: text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已复制到剪贴板')),
    );
  }
}

class _SpeedCard extends StatelessWidget {
  final String title;
  final double speed;
  final IconData icon;
  final Color color;

  const _SpeedCard({
    required this.title,
    required this.speed,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(icon, color: color, size: 20),
                const SizedBox(width: 8),
                Text(
                  title,
                  style: TextStyle(color: Colors.grey[400], fontSize: 13),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              '${speed.toStringAsFixed(1)} Mbps',
              style: TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.bold,
                color: color,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
