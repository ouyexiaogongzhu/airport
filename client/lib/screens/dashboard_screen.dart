import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/subscription.dart' show SubscriptionInfo;
import '../services/subscription_service.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
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

  Future<void> _loadAll() async {
    final subscriptionService = context.read<SubscriptionService>();
    await Future.wait([
      subscriptionService.loadConfig(),
      subscriptionService.loadSubscription(),
    ]);
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

  String _formatExpireTime(int unixTs) {
    if (unixTs <= 0) return '—';
    final dt = DateTime.fromMillisecondsSinceEpoch(unixTs * 1000);
    final month = dt.month.toString().padLeft(2, '0');
    final day = dt.day.toString().padLeft(2, '0');
    return '${dt.year}-$month-$day';
  }

  String _formatTrafficGb(double gb) {
    if (gb >= 100) return gb.toStringAsFixed(0);
    if (gb >= 10) return gb.toStringAsFixed(1);
    return gb.toStringAsFixed(2);
  }

  ({String label, Color color, IconData icon}) _statusStyle(String? status) {
    switch (status) {
      case null:
      case 'SUBSCRIPTION_ACTIVE':
        return (label: '已激活', color: Colors.green, icon: Icons.check_circle);
      case 'SUBSCRIPTION_PENDING':
        return (label: '待订阅', color: Colors.orange, icon: Icons.hourglass_top);
      case 'SUBSCRIPTION_EXPIRED':
        return (label: '已过期', color: Colors.red, icon: Icons.error_outline);
      default:
        return (label: status, color: Colors.grey, icon: Icons.help_outline);
    }
  }

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _loadAll,
      child: ListView(
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
            '订阅状态与流量概览',
            style: TextStyle(color: Colors.grey[400]),
          ),
          const SizedBox(height: 24),

          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final sub = subService.subscription;
              final statusError = subService.statusError;
              final statusStyle = _statusStyle(statusError);

              if (subService.isLoading && sub == null && statusError == null) {
                return const Padding(
                  padding: EdgeInsets.all(48),
                  child: Center(child: CircularProgressIndicator()),
                );
              }

              return Column(
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Card(
                    color: statusStyle.color.withAlpha(20),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                      side: BorderSide(color: statusStyle.color.withAlpha(80)),
                    ),
                    child: Padding(
                      padding: const EdgeInsets.all(16),
                      child: Row(
                        children: [
                          Icon(statusStyle.icon, color: statusStyle.color, size: 28),
                          const SizedBox(width: 12),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  '订阅状态',
                                  style: TextStyle(color: Colors.grey[400], fontSize: 13),
                                ),
                                Text(
                                  statusStyle.label,
                                  style: TextStyle(
                                    fontSize: 20,
                                    fontWeight: FontWeight.bold,
                                    color: statusStyle.color,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          if (statusError == 'SUBSCRIPTION_PENDING' || statusError == 'SUBSCRIPTION_EXPIRED')
                            FilledButton.icon(
                              onPressed: () => _openRenewalPortal(subService),
                              icon: const Icon(Icons.open_in_new, size: 18),
                              label: const Text('续费'),
                            ),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),

                  if (sub != null) ...[
                    _InfoCard(
                      title: '套餐等级',
                      value: sub.tier.isNotEmpty ? sub.tier.toUpperCase() : '—',
                      icon: Icons.workspace_premium,
                      color: Colors.cyanAccent,
                    ),
                    const SizedBox(height: 12),
                    _InfoCard(
                      title: '到期时间',
                      value: _formatExpireTime(sub.expireTime),
                      icon: Icons.event,
                      color: Colors.blue,
                    ),
                    const SizedBox(height: 12),
                    _InfoCard(
                      title: '可用节点',
                      value: '${sub.nodes.length} 个',
                      icon: Icons.public,
                      color: Colors.teal,
                    ),
                    const SizedBox(height: 12),
                    _TrafficCard(subscription: sub, formatTrafficGb: _formatTrafficGb),
                  ] else if (statusError == 'SUBSCRIPTION_PENDING' || statusError == 'SUBSCRIPTION_EXPIRED') ...[
                    Card(
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: Text(
                          statusError == 'SUBSCRIPTION_PENDING'
                              ? '您尚未购买订阅，请点击续费前往官网选择套餐。'
                              : '您的订阅已过期，请续费后继续使用 VPN 服务。',
                          style: TextStyle(color: Colors.grey[300]),
                        ),
                      ),
                    ),
                  ],

                  const SizedBox(height: 24),
                  Center(
                    child: Text(
                      '下拉刷新数据',
                      style: TextStyle(color: Colors.grey[600], fontSize: 12),
                    ),
                  ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }
}

class _InfoCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData icon;
  final Color color;

  const _InfoCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Row(
          children: [
            Icon(icon, color: color, size: 24),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: TextStyle(color: Colors.grey[400], fontSize: 13)),
                  const SizedBox(height: 4),
                  Text(
                    value,
                    style: const TextStyle(fontSize: 18, fontWeight: FontWeight.bold),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _TrafficCard extends StatelessWidget {
  final SubscriptionInfo subscription;
  final String Function(double) formatTrafficGb;

  const _TrafficCard({
    required this.subscription,
    required this.formatTrafficGb,
  });

  @override
  Widget build(BuildContext context) {
    final remainingGb = subscription.trafficRemainingBytes / (1024 * 1024 * 1024);
    final limitGb = 100.0; // Estimated total display (100GB)
    final ratio = (remainingGb / limitGb).clamp(0.0, 1.0);

    return Card(
      color: Colors.cyanAccent.withAlpha(20),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: Colors.cyanAccent.withAlpha(60)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.data_usage, color: Colors.cyanAccent, size: 24),
                const SizedBox(width: 8),
                Text(
                  '流量使用',
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.bold,
                    color: Colors.cyanAccent,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Text(
              '剩余 ${formatTrafficGb(remainingGb)} GB',
              style: TextStyle(color: Colors.grey[300], fontSize: 14),
            ),
            const SizedBox(height: 8),
            ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: ratio,
                minHeight: 8,
                backgroundColor: Colors.grey[800],
                color: Colors.cyanAccent,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
