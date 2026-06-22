import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/api_service.dart';
import '../services/subscription_service.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  Map<String, dynamic> _stats = {};
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _loadStats();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      final sub = context.read<SubscriptionService>();
      if (sub.subscription == null && !sub.isLoading) {
        sub.loadSubscription();
      }
    });
  }

  Future<void> _loadAll() async {
    final subscriptionService = context.read<SubscriptionService>();
    await Future.wait([
      _loadStats(),
      subscriptionService.loadSubscription(),
    ]);
  }

  Future<void> _loadStats() async {
    setState(() => _loading = true);
    try {
      final data = await ApiService().get('/dashboard/stats');
      setState(() {
        _stats = data;
        _loading = false;
      });
    } catch (_) {
      // Mock fallback
      setState(() {
        _stats = {
          'total_users': 1284,
          'active_users': 892,
          'total_products': 56,
          'active_products': 42,
          'revenue_today': 15890.00,
          'revenue_month': 342560.00,
          'orders_today': 47,
          'pending_orders': 12,
        };
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _loadAll,
      child: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          // Header
          Text(
            '仪表盘',
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
          ),
          const SizedBox(height: 8),
          Text(
            '系统概览与关键指标',
            style: TextStyle(color: Colors.grey[400]),
          ),
          const SizedBox(height: 24),

          // Subscription card
          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final sub = subService.subscription;
              if (subService.isLoading && sub == null) {
                return const Padding(
                  padding: EdgeInsets.only(bottom: 16),
                  child: Center(child: CircularProgressIndicator()),
                );
              }
              if (sub == null) return const SizedBox.shrink();

              final ratio = sub.totalTraffic > 0
                  ? (sub.usedTraffic / sub.totalTraffic).clamp(0.0, 1.0)
                  : 0.0;

              return Card(
                margin: const EdgeInsets.only(bottom: 16),
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
                          Icon(
                            Icons.shield,
                            color: Colors.cyanAccent,
                            size: 24,
                          ),
                          const SizedBox(width: 8),
                          Text(
                            sub.planName,
                            style: const TextStyle(
                              fontSize: 18,
                              fontWeight: FontWeight.bold,
                              color: Colors.cyanAccent,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 8),
                      Text(
                        '到期日期: ${sub.expiryDate}',
                        style: TextStyle(color: Colors.grey[400]),
                      ),
                      const SizedBox(height: 16),
                      Text(
                        '已用 ${sub.usedTraffic} GB / ${sub.totalTraffic.toInt()} GB',
                        style: TextStyle(
                          color: Colors.grey[300],
                          fontSize: 13,
                        ),
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
            },
          ),

          if (_loading)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(48),
                child: CircularProgressIndicator(),
              ),
            )
          else ...[
            // Revenue cards row
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: '今日营收',
                    value: '¥${_fmt(_stats['revenue_today'] ?? 0)}',
                    icon: Icons.today,
                    color: Colors.green,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: '本月营收',
                    value: '¥${_fmt(_stats['revenue_month'] ?? 0)}',
                    icon: Icons.date_range,
                    color: Colors.blue,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // User stats
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: '总用户',
                    value: '${_stats['total_users'] ?? 0}',
                    icon: Icons.people,
                    color: Colors.purple,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: '活跃用户',
                    value: '${_stats['active_users'] ?? 0}',
                    icon: Icons.person_pin,
                    color: Colors.teal,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Product stats
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: '总产品',
                    value: '${_stats['total_products'] ?? 0}',
                    icon: Icons.inventory_2,
                    color: Colors.orange,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: '已上架',
                    value: '${_stats['active_products'] ?? 0}',
                    icon: Icons.check_circle,
                    color: Colors.indigo,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Order stats
            Row(
              children: [
                Expanded(
                  child: _StatCard(
                    title: '今日订单',
                    value: '${_stats['orders_today'] ?? 0}',
                    icon: Icons.shopping_cart,
                    color: Colors.cyan,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: _StatCard(
                    title: '待处理',
                    value: '${_stats['pending_orders'] ?? 0}',
                    icon: Icons.pending_actions,
                    color: Colors.red,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),

            // Refresh hint
            Center(
              child: Text(
                '下拉刷新数据',
                style: TextStyle(color: Colors.grey[600], fontSize: 12),
              ),
            ),
          ],
        ],
      ),
    );
  }

  String _fmt(dynamic value) {
    if (value is double) {
      return value.toStringAsFixed(2);
    }
    return value.toString();
  }
}

class _StatCard extends StatelessWidget {
  final String title;
  final String value;
  final IconData icon;
  final Color color;

  const _StatCard({
    required this.title,
    required this.value,
    required this.icon,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      elevation: 2,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
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
                  style: TextStyle(
                    color: Colors.grey[400],
                    fontSize: 13,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              value,
              style: TextStyle(
                fontSize: 24,
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
