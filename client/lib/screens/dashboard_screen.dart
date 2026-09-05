import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/subscription_service.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  @override
  Widget build(BuildContext context) {
    return Consumer<SubscriptionService>(
      builder: (context, subService, _) {
        final nodes = subService.nodes;
        final hasNodes = nodes.isNotEmpty;

        return ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(
              'VPN 仪表盘',
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            const SizedBox(height: 8),
            Text(
              '节点导入与连接状态',
              style: TextStyle(color: Colors.grey[400]),
            ),
            const SizedBox(height: 24),
            Card(
              color: (hasNodes ? Colors.green : Colors.orange).withAlpha(15),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
                side: BorderSide(
                  color: (hasNodes ? Colors.green : Colors.orange).withAlpha(80),
                ),
              ),
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Row(
                  children: [
                    Icon(
                      hasNodes ? Icons.cloud_done : Icons.cloud_queue,
                      size: 28,
                      color: hasNodes ? Colors.green : Colors.orange,
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '节点状态',
                            style: TextStyle(color: Colors.grey[400], fontSize: 13),
                          ),
                          const SizedBox(height: 4),
                          Text(
                            hasNodes ? '已导入 ${nodes.length} 个节点' : '尚未导入订阅',
                            style: const TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.bold,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 12),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Text(
                  hasNodes
                      ? '节点已就绪，前往「VPN」页选择节点并连接。'
                      : '请通过订阅链接导入节点后使用。节点信息由您的订阅服务商提供。',
                  style: TextStyle(color: Colors.grey[300]),
                ),
              ),
            ),
            const SizedBox(height: 12),
            SizedBox(
              width: double.infinity,
              height: 48,
              child: ElevatedButton.icon(
                onPressed: () => Navigator.pushNamed(context, '/subscription/input'),
                icon: const Icon(Icons.link, size: 20),
                label: Text(hasNodes ? '更新订阅' : '导入订阅'),
              ),
            ),
          ],
        );
      },
    );
  }
}
