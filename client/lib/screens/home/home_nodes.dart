import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../config.dart';
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
      if (AppConfig.storeMode) return;
      final sub = context.read<SubscriptionService>();
      if (sub.subscription == null && !sub.isLoading) {
        sub.loadSubscription();
      }
    });
  }

  Future<void> _refresh() async {
    final subService = context.read<SubscriptionService>();
    final vpn = context.read<VpnService>();
    if (AppConfig.storeMode) {
      final nodes = subService.effectiveNodes;
      if (nodes.isNotEmpty) {
        await vpn.pingAllNodesList(nodes);
      }
      return;
    }
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

  // BUG: Ping always targets 8.8.8.8:53 instead of the actual node address.
  // Should use node.host / node.port to measure real latency to each node.
  Future<void> _pingNode(VpnService vpn, VpnNode node) async {
    await vpn.pingNode(node.name, '8.8.8.8', 53);
  }

  Future<void> _openRenewalPortal(SubscriptionService subService) async {
    final url = subService.renewalUrl;
    if (url == null) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('续费链接暂不可用')),
      );
      return;
    }

    final uri = Uri.parse(url);
    if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('无法打开链接: $url')),
      );
    }
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
                  final sub = subService.subscription;
                  final nodes = AppConfig.storeMode
                      ? subService.effectiveNodes
                      : (sub?.nodes ?? []);
                  final statusError = subService.statusError;

                  // --- Empty / expired state ---
                  if (nodes.isEmpty && statusError != null) {
                    // store 模式无订阅状态概念，直接显示中性空态。
                    if (AppConfig.storeMode) {
                      return SizedBox(
                        height: MediaQuery.of(context).size.height * 0.5,
                        child: Center(
                          child: Text(
                            '暂无可用节点\n请先导入订阅',
                            textAlign: TextAlign.center,
                            style: TextStyle(color: Colors.grey[500]),
                          ),
                        ),
                      );
                    }
                    // Show renewal card for expired/pending subscriptions
                    return _ExpiredRenewalCard(
                      statusError: statusError,
                      onRenew: () => _openRenewalPortal(subService),
                    );
                  }

                  if (nodes.isEmpty) {
                    return SizedBox(
                      height: MediaQuery.of(context).size.height * 0.5,
                      child: Center(
                        child: Text(
                          subService.isLoading
                              ? ''
                              : (AppConfig.storeMode
                                  ? '暂无可用节点\n请先导入订阅'
                                  : '暂无可用节点\n请先购买套餐'),
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
              // --- Renewal button (always visible when nodes exist) ---
              Consumer<SubscriptionService>(
                builder: (context, subService, _) {
                  // store 模式禁止展示续费/购买入口。
                  if (AppConfig.storeMode) return const SizedBox.shrink();

                  final nodes = subService.subscription?.nodes ?? [];
                  if (nodes.isEmpty) return const SizedBox.shrink();

                  return Padding(
                    padding: const EdgeInsets.only(top: 24),
                    child: SizedBox(
                      width: double.infinity,
                      child: ElevatedButton.icon(
                        onPressed: () => _openRenewalPortal(subService),
                        icon: const Icon(Icons.replay, size: 20),
                        label: const Text('续费 / 升级套餐'),
                        style: ElevatedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: 16),
                        ),
                      ),
                    ),
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

/// Card displayed when subscription is expired or pending.
class _ExpiredRenewalCard extends StatelessWidget {
  final String statusError;
  final VoidCallback onRenew;

  const _ExpiredRenewalCard({
    required this.statusError,
    required this.onRenew,
  });

  @override
  Widget build(BuildContext context) {
    final isExpired = statusError == 'SUBSCRIPTION_EXPIRED';
    final Color statusColor = isExpired ? Colors.red : Colors.orange;
    final String heading = isExpired ? '订阅已过期' : '等待订阅';
    final String description = isExpired
        ? '您的订阅已过期，所有节点暂时不可用。请续费后继续使用 VPN 服务。'
        : '您尚未购买订阅，请前往官网选择套餐。';

    return SizedBox(
      height: MediaQuery.of(context).size.height * 0.5,
      child: Center(
        child: Card(
          color: statusColor.withAlpha(15),
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(16),
            side: BorderSide(color: statusColor.withAlpha(80), width: 1.5),
          ),
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  isExpired ? Icons.timer_off : Icons.hourglass_empty,
                  size: 64,
                  color: statusColor.withAlpha(200),
                ),
                const SizedBox(height: 20),
                Text(
                  heading,
                  style: Theme.of(context).textTheme.titleLarge?.copyWith(
                        fontWeight: FontWeight.bold,
                        color: statusColor,
                      ),
                ),
                const SizedBox(height: 12),
                Text(
                  description,
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: Colors.grey[300],
                    fontSize: 14,
                    height: 1.5,
                  ),
                ),
                const SizedBox(height: 28),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: onRenew,
                    icon: const Icon(Icons.open_in_new, size: 20),
                    label: Text(isExpired ? '点击续费' : '前往购买'),
                    style: FilledButton.styleFrom(
                      backgroundColor: statusColor,
                      foregroundColor: Colors.white,
                      padding: const EdgeInsets.symmetric(vertical: 16),
                      shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(12),
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
