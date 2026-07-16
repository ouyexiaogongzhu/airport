import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../services/subscription_service.dart';
import '../../widgets/loading_overlay.dart';
import '../../widgets/status_badge.dart';
import '../../widgets/traffic_bar.dart';

class AccountSubscription extends StatefulWidget {
  const AccountSubscription({super.key});

  @override
  State<AccountSubscription> createState() => _AccountSubscriptionState();
}

// TODO(M4): _statusBadge, _formatTrafficGb, and _trafficEstimateBytes are
// duplicated across AccountSubscription, HomeDashboard, and DashboardScreen.
// Extract shared utilities and widgets.
class _AccountSubscriptionState extends State<AccountSubscription> {
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

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('订阅详情'),
      ),
      body: Stack(
      children: [
        RefreshIndicator(
          onRefresh: _refresh,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Text(
                '订阅详情',
                style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                      fontWeight: FontWeight.bold,
                    ),
              ),
              const SizedBox(height: 8),
              Text(
                '套餐、到期时间与流量信息',
                style: TextStyle(color: Colors.grey[400]),
              ),
              const SizedBox(height: 24),
              Consumer<SubscriptionService>(
                builder: (context, subService, _) {
                  final sub = subService.subscription;
                  final statusError = subService.statusError;

                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Card(
                        child: Padding(
                          padding: const EdgeInsets.all(16),
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
                      ),
                      if (sub != null) ...[
                        const SizedBox(height: 12),
                        Card(
                          child: Padding(
                            padding: const EdgeInsets.all(16),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                _DetailRow(
                                  label: '套餐等级',
                                  value: sub.tierLabel,
                                ),
                                const Divider(height: 24),
                                _DetailRow(
                                  label: '到期时间',
                                  value: sub.expireDateFormatted,
                                ),
                                const Divider(height: 24),
                                _DetailRow(
                                  label: '可用节点',
                                  value: '${sub.nodeCount} 个',
                                ),
                                const Divider(height: 24),
                                _DetailRow(
                                  label: '订阅版本',
                                  value: 'v${sub.subscriptionVersion}',
                                ),
                              ],
                            ),
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
      )
    );
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({
    required this.label,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: TextStyle(color: Colors.grey[400], fontSize: 14),
        ),
        Text(
          value,
          style: const TextStyle(
            fontSize: 15,
            fontWeight: FontWeight.w600,
          ),
        ),
      ],
    );
  }
}
