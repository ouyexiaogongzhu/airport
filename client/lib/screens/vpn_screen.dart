import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/subscription.dart';
import '../services/subscription_service.dart';
import '../services/vpn_service.dart';

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
    final subscriptionService = context.read<SubscriptionService>();
    await subscriptionService.loadSubscription();
  }

  String _formatDuration(Duration duration) {
    final minutes = duration.inMinutes.remainder(60).toString().padLeft(2, '0');
    final seconds = duration.inSeconds.remainder(60).toString().padLeft(2, '0');
    return '$minutes:$seconds';
  }

  @override
  Widget build(BuildContext context) {
    final vpn = context.watch<VpnService>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN 控制'),
      ),
      body: Consumer<SubscriptionService>(
        builder: (context, subService, _) {
          final subscription = subService.subscription;
          final nodes = subscription?.nodes ?? [];

          return RefreshIndicator(
            onRefresh: _refresh,
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                const SizedBox(height: 16),

                // Toggle button
                Center(
                  child: GestureDetector(
                    onTap: vpn.isConnecting ? null : () => vpn.toggle(),
                    child: SizedBox(
                      width: 120,
                      height: 120,
                      child: Stack(
                        alignment: Alignment.center,
                        children: [
                          Container(
                            width: 120,
                            height: 120,
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: vpn.isConnected
                                  ? Colors.green
                                  : vpn.isConnecting
                                      ? Colors.grey[700]
                                      : Colors.grey[600],
                              boxShadow: [
                                BoxShadow(
                                  color: (vpn.isConnected
                                          ? Colors.green
                                          : Colors.grey)
                                      .withAlpha(80),
                                  blurRadius: 16,
                                  spreadRadius: 2,
                                ),
                              ],
                            ),
                            child: Icon(
                              vpn.isConnected
                                  ? Icons.vpn_lock
                                  : Icons.vpn_lock_outlined,
                              size: 48,
                              color: Colors.white,
                            ),
                          ),
                          if (vpn.isConnecting)
                            SizedBox(
                              width: 120,
                              height: 120,
                              child: CircularProgressIndicator(
                                strokeWidth: 3,
                                color: Colors.cyanAccent.withAlpha(180),
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: 16),

                // Connection status
                Center(
                  child: Text(
                    vpn.isConnecting
                        ? '连接中...'
                        : vpn.isConnected
                            ? '已连接'
                            : '已断开',
                    style: Theme.of(context).textTheme.titleMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: vpn.isConnected
                              ? Colors.green
                              : vpn.isConnecting
                                  ? Colors.cyanAccent
                                  : Colors.grey[400],
                        ),
                  ),
                ),

                if (vpn.isConnected) ...[
                  const SizedBox(height: 8),
                  Center(
                    child: Text(
                      '节点: ${vpn.selectedNode ?? '未选择'}',
                      style: TextStyle(color: Colors.grey[400]),
                    ),
                  ),
                  const SizedBox(height: 4),
                  Center(
                    child: Text(
                      '连接时长: ${_formatDuration(vpn.connectedTime)}',
                      style: TextStyle(color: Colors.grey[400]),
                    ),
                  ),
                ],
                const SizedBox(height: 24),

                // Speed cards
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

                // Nodes section
                Text(
                  '可用节点',
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.bold,
                      ),
                ),
                const SizedBox(height: 12),

                if (subService.isLoading && nodes.isEmpty)
                  const Center(
                    child: Padding(
                      padding: EdgeInsets.all(24),
                      child: CircularProgressIndicator(),
                    ),
                  )
                else if (nodes.isEmpty)
                  Center(
                    child: Text(
                      '暂无可用节点',
                      style: TextStyle(color: Colors.grey[500]),
                    ),
                  )
                else
                  ...nodes.map((node) => _NodeTile(
                        node: node,
                        isSelected: vpn.selectedNode == node.name,
                        onTap: () =>
                            context.read<VpnService>().selectNode(node.name),
                      )),

                const SizedBox(height: 24),
                Center(
                  child: Text(
                    '下拉刷新节点',
                    style: TextStyle(color: Colors.grey[600], fontSize: 12),
                  ),
                ),
              ],
            ),
          );
        },
      ),
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

class _NodeTile extends StatelessWidget {
  final VpnNode node;
  final bool isSelected;
  final VoidCallback onTap;

  const _NodeTile({
    required this.node,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: isSelected
          ? Colors.cyanAccent.withAlpha(25)
          : Theme.of(context).cardTheme.color,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: isSelected
            ? BorderSide(color: Colors.cyanAccent.withAlpha(120))
            : BorderSide.none,
      ),
      child: ListTile(
        onTap: onTap,
        leading: Icon(
          Icons.public,
          color: isSelected ? Colors.cyanAccent : Colors.grey[400],
        ),
        title: Text(
          node.name,
          style: TextStyle(
            fontWeight: isSelected ? FontWeight.bold : FontWeight.normal,
            color: isSelected ? Colors.cyanAccent : null,
          ),
        ),
        subtitle: Text(
          node.uri,
          style: TextStyle(
            fontSize: 12,
            color: isSelected ? Colors.cyanAccent.withAlpha(180) : Colors.grey,
          ),
        ),
        trailing: Icon(
          isSelected ? Icons.check_circle : Icons.circle_outlined,
          color: isSelected ? Colors.cyanAccent : Colors.grey,
          size: 20,
        ),
      ),
    );
  }
}
