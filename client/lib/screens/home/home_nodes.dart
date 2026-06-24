import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../../models/subscription.dart';
import '../../services/subscription_service.dart';
import '../../services/vpn_service.dart';
import '../../widgets/loading_overlay.dart';
import '../../widgets/node_card.dart';

class HomeNodes extends StatefulWidget {
  const HomeNodes({super.key});

  @override
  State<HomeNodes> createState() => _HomeNodesState();
}

class _HomeNodesState extends State<HomeNodes> {
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

  void _copyToClipboard(String text) {
    Clipboard.setData(ClipboardData(text: text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已复制到剪贴板')),
    );
  }

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
                '全部节点',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 8),
              Text(
                '下拉刷新并测速所有节点',
                style: TextStyle(color: Colors.grey[400]),
              ),
              const SizedBox(height: 24),
              Consumer<SubscriptionService>(
                builder: (context, subService, _) {
                  final nodes = subService.subscription?.nodes ?? [];

                  if (nodes.isEmpty) {
                    return SizedBox(
                      height: MediaQuery.of(context).size.height * 0.5,
                      child: Center(
                        child: Text(
                          subService.isLoading ? '' : '暂无可用节点\n请先购买套餐',
                          textAlign: TextAlign.center,
                          style: TextStyle(color: Colors.grey[500]),
                        ),
                      ),
                    );
                  }

                  return Column(
                    children: nodes
                        .map(
                          (node) => NodeCard(
                            node: node,
                            latency: vpn.latencies[node.name],
                            isSelected: vpn.selectedNode == node.name,
                            isConnected: vpn.connectedNode == node.name && vpn.isConnected,
                            onTap: () => vpn.selectNode(node.name),
                            onPing: () => _pingNode(vpn, node),
                            onCopy: () => _copyToClipboard(node.uri),
                          ),
                        )
                        .toList(),
                  );
                },
              ),
            ],
          ),
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
    );
  }
}
