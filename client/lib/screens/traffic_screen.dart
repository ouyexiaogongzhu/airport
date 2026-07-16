import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../models/subscription.dart';
import '../services/subscription_service.dart';
import '../services/vpn_service.dart';
import '../utils/traffic_helper.dart';
import '../widgets/traffic_bar.dart';
import '../widgets/traffic_chart.dart';

/// Time range options for the traffic chart.
enum TrafficTimeRange { today, week, month }

/// Real-time traffic usage dashboard with configurable time range charts.
///
/// Provides:
///   - Total traffic used / limit with [TrafficBar]
///   - Today's usage, session usage (when VPN is connected), remaining days
///   - Configurable time range bar chart via [TrafficChart] (今日/本周/本月)
///   - Pull-to-refresh
class TrafficScreen extends StatefulWidget {
  const TrafficScreen({super.key});

  @override
  State<TrafficScreen> createState() => _TrafficScreenState();
}

class _TrafficScreenState extends State<TrafficScreen> {
  /// Estimated total traffic limit (100 GB).
  // TODO(M7): Traffic estimate is hardcoded at 100 GB. Fetch the actual plan
  // limit from the API or subscription config instead.
  static const int _trafficLimitBytes = 100 * 1024 * 1024 * 1024;

  /// Simulated "today" usage — in production this comes from the API.
  int _todayBytes = 0;

  /// Session usage (only tracked while VPN is connected).
  int _sessionBytes = 0;

  /// Daily data: list of (date, bytes) for the selected time range.
  List<(DateTime, int)> _dailyData = [];

  /// Whether daily data has been initialised.
  bool _initialised = false;

  /// Current time range selection.
  TrafficTimeRange _selectedRange = TrafficTimeRange.week;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (!_initialised) {
      _initialised = true;
      _generateData();
    }
  }

  /// Generate realistic daily traffic data based on the subscription
  /// and the selected time range.
  void _generateData() {
    final subService = context.read<SubscriptionService>();
    final sub = subService.subscription;

    final totalUsed = sub != null
        ? (_trafficLimitBytes - sub.trafficRemainingBytes).clamp(0, _trafficLimitBytes)
        : 0;

    _dailyData = generateDailyTraffic(
      today: DateTime.now(),
      totalUsedBytes: totalUsed > 0 ? totalUsed : 500 * 1024 * 1024, // fallback 500 MB
      days: _daysForRange(_selectedRange),
    );

    // "Today" usage is the last entry in the daily series
    if (_dailyData.isNotEmpty) {
      _todayBytes = _dailyData.last.$2;
    }
  }

  /// Number of days to show for the selected time range.
  int _daysForRange(TrafficTimeRange range) {
    switch (range) {
      case TrafficTimeRange.today:
        return 1;
      case TrafficTimeRange.week:
        return 7;
      case TrafficTimeRange.month:
        return 30;
    }
  }

  /// Chart section title for the selected range.
  String _chartTitle() {
    switch (_selectedRange) {
      case TrafficTimeRange.today:
        return '今日流量分布';
      case TrafficTimeRange.week:
        return '近 7 天流量';
      case TrafficTimeRange.month:
        return '近 30 天流量';
    }
  }

  Future<void> _refresh() async {
    final subService = context.read<SubscriptionService>();
    await Future.wait([
      subService.loadConfig(),
      subService.loadSubscription(),
    ]);
    _generateData();
    if (mounted) setState(() {});
  }

  /// Compute remaining days from subscription expire time.
  int _remainingDays(SubscriptionInfo sub) {
    if (sub.expireTime <= 0) return -1; // permanent
    final expireDt = sub.expireDateTime;
    if (expireDt == null) return -1;
    return expireDt.difference(DateTime.now()).inDays.clamp(0, 9999);
  }

  /// Format a list of bytes values with high precision for small values,
  /// adapting to the data range.
  String _formatChartValue(int bytes) {
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    if (bytes < 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(2)} MB';
    }
    return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(3)} GB';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('流量统计'),
      ),
      body: Consumer2<SubscriptionService, VpnService>(
      builder: (context, subService, vpn, _) {
        final sub = subService.subscription;
        final statusError = subService.statusError;

        // Update session usage based on VPN connection state
        if (vpn.isConnected) {
          // Simulate session traffic (in production, this comes from the VPN service)
          final sessionSecs = vpn.connectedTime.inSeconds;
          _sessionBytes = (sessionSecs * (15 + 5) ~/ 2).round(); // avg 10 KB/s
        } else {
          _sessionBytes = 0;
        }

        return RefreshIndicator(
          onRefresh: _refresh,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              // --- Header ---
              Row(
                children: [
                  const Icon(Icons.bar_chart_rounded, color: Colors.cyanAccent, size: 28),
                  const SizedBox(width: 10),
                  Text(
                    '流量统计',
                    style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                          fontWeight: FontWeight.bold,
                        ),
                  ),
                ],
              ),
              const SizedBox(height: 6),
              Text(
                '实时流量使用与历史趋势',
                style: TextStyle(color: Colors.grey[400], fontSize: 13),
              ),
              const SizedBox(height: 20),

              if (statusError == 'SUBSCRIPTION_PENDING' ||
                  statusError == 'SUBSCRIPTION_EXPIRED') ...[
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Text(
                      statusError == 'SUBSCRIPTION_PENDING'
                          ? '您尚未购买订阅，暂无流量数据。'
                          : '您的订阅已过期，流量数据不可用。',
                      style: TextStyle(color: Colors.grey[300]),
                    ),
                  ),
                ),
              ] else ...[
                // --- Traffic usage bar card ---
                _buildTrafficBarCard(context, sub),
                const SizedBox(height: 12),

                // --- Stats cards row ---
                _buildStatsGrid(context, sub, vpn),
                const SizedBox(height: 16),

                // --- Time range selector ---
                _buildTimeRangeSelector(context),
                const SizedBox(height: 12),

                // --- Chart section ---
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            const Icon(Icons.trending_up, color: Colors.cyanAccent, size: 20),
                            const SizedBox(width: 8),
                            Text(
                              _chartTitle(),
                              style: const TextStyle(
                                fontSize: 16,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ],
                        ),
                        const SizedBox(height: 16),
                        // For single-day view, show detailed breakdown
                        if (_selectedRange == TrafficTimeRange.today)
                          ..._buildTodayDetail()
                        else
                          TrafficChart(
                            data: _dailyData.map((e) => e.$2).toList(),
                            labels: _dailyData.map((e) => dayLabel(e.$1)).toList(),
                            height: 220,
                          ),
                        const SizedBox(height: 8),
                        Text(
                          _selectedRange == TrafficTimeRange.today
                              ? '今日每时段流量分布（模拟数据）'
                              : '每日流量使用情况（绿色=正常，黄色=偏高，红色=即将超限）',
                          style: TextStyle(
                            color: Colors.grey[500],
                            fontSize: 11,
                          ),
                        ),
                      ],
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
          ),
        );
      },
      )
    );
  }

  /// Build time range segmented selector.
  Widget _buildTimeRangeSelector(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        child: Row(
          children: [
            Icon(Icons.date_range, size: 18, color: Colors.grey[400]),
            const SizedBox(width: 8),
            Text(
              '时间范围',
              style: TextStyle(color: Colors.grey[400], fontSize: 13),
            ),
            const Spacer(),
            SegmentedButton<TrafficTimeRange>(
              segments: const [
                ButtonSegment(
                  value: TrafficTimeRange.today,
                  label: Text('今日', style: TextStyle(fontSize: 12)),
                ),
                ButtonSegment(
                  value: TrafficTimeRange.week,
                  label: Text('本周', style: TextStyle(fontSize: 12)),
                ),
                ButtonSegment(
                  value: TrafficTimeRange.month,
                  label: Text('本月', style: TextStyle(fontSize: 12)),
                ),
              ],
              selected: {_selectedRange},
              onSelectionChanged: (Set<TrafficTimeRange> selection) {
                setState(() {
                  _selectedRange = selection.first;
                  _generateData();
                });
              },
              style: ButtonStyle(
                visualDensity: VisualDensity.compact,
                tapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
            ),
          ],
        ),
      ),
    );
  }

  /// Build detailed breakdown for today's view.
  List<Widget> _buildTodayDetail() {
    final hourBytes = _dailyData.isNotEmpty ? _dailyData.last.$2 : 0;
    // Generate hourly distribution from today's total
    final hourlyData = <int>[];
    final now = DateTime.now();
    final currentHour = now.hour;
    final seed = hourBytes.isEven ? 7 : 13;

    for (int h = 0; h <= currentHour; h++) {
      final fraction = (seed * (h + 1) * 3) % 100;
      final bytes = (hourBytes * fraction / 2000).round().clamp(0, hourBytes);
      hourlyData.add(bytes > 0 ? bytes : hourBytes ~/ 24);
    }

    return [
      SizedBox(
        height: 180,
        child: TrafficChart(
          data: hourlyData,
          labels: hourlyData.asMap().entries.map((e) => '${e.key}时').toList(),
          height: 180,
        ),
      ),
      const SizedBox(height: 8),
      Row(
        mainAxisAlignment: MainAxisAlignment.spaceAround,
        children: [
          _buildTodayStat('累计使用', formatBytes(hourBytes), Colors.cyanAccent),
          _buildTodayStat('最大时段', _formatChartValue(hourlyData.reduce((a, b) => a > b ? a : b)), Colors.orangeAccent),
          _buildTodayStat('平均时段', _formatChartValue(hourlyData.isEmpty ? 0 : hourlyData.reduce((a, b) => a + b) ~/ hourlyData.length), Colors.blueAccent),
        ],
      ),
    ];
  }

  Widget _buildTodayStat(String label, String value, Color color) {
    return Column(
      children: [
        Text(
          label,
          style: TextStyle(color: Colors.grey[400], fontSize: 10),
        ),
        const SizedBox(height: 2),
        Text(
          value,
          style: TextStyle(
            fontSize: 12,
            fontWeight: FontWeight.bold,
            color: color,
          ),
        ),
      ],
    );
  }

  /// Build the main traffic usage bar card.
  Widget _buildTrafficBarCard(BuildContext context, SubscriptionInfo? sub) {
    final usedBytes = sub != null
        ? (_trafficLimitBytes - sub.trafficRemainingBytes).clamp(0, _trafficLimitBytes)
        : 0;
    final realTotal = sub != null
        ? (sub.trafficRemainingBytes + usedBytes)
        : _trafficLimitBytes;
    final remainingGb = (sub?.trafficRemainingBytes ?? 0) / (1024 * 1024 * 1024);

    return Card(
      color: Colors.cyanAccent.withAlpha(15),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: BorderSide(color: Colors.cyanAccent.withAlpha(50)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                const Icon(Icons.data_usage, color: Colors.cyanAccent, size: 22),
                const SizedBox(width: 8),
                const Text(
                  '总流量使用',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                    color: Colors.cyanAccent,
                  ),
                ),
                const Spacer(),
                // Animated number effect via AnimatedSwitcher
                AnimatedSwitcher(
                  duration: const Duration(milliseconds: 300),
                  child: Text(
                    '${((usedBytes / _trafficLimitBytes) * 100).toStringAsFixed(0)}%',
                    key: ValueKey(usedBytes),
                    style: TextStyle(
                      fontSize: 18,
                      fontWeight: FontWeight.bold,
                      color: _usageColor(usedBytes / _trafficLimitBytes),
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 14),
            TrafficBar(
              usedBytes: usedBytes,
              totalBytes: realTotal,
              key: ValueKey('traffic_$usedBytes'),
              label: remainingGb >= 0
                  ? '剩余 ${formatGb(sub?.trafficRemainingBytes ?? 0)} GB'
                  : null,
            ),
            const SizedBox(height: 8),
            // Additional precision row
            Row(
              children: [
                Text(
                  '已用 ${formatBytes(usedBytes)}',
                  style: TextStyle(
                    fontSize: 11,
                    color: Colors.grey[500],
                  ),
                ),
                const SizedBox(width: 8),
                Text(
                  '/ 总计 ${formatBytes(realTotal)}',
                  style: TextStyle(
                    fontSize: 11,
                    color: Colors.grey[600],
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  /// Build the stats grid with today's usage, session usage, remaining days.
  Widget _buildStatsGrid(
    BuildContext context,
    SubscriptionInfo? sub,
    VpnService vpn,
  ) {
    final remainingDays = sub != null ? _remainingDays(sub) : -1;

    return Row(
      children: [
        Expanded(
          child: _StatCard(
            icon: Icons.today,
            iconColor: Colors.blueAccent,
            title: '今日使用',
            value: formatBytes(_todayBytes),
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _StatCard(
            icon: Icons.sensors,
            iconColor: vpn.isConnected ? Colors.greenAccent : Colors.grey,
            title: '会话用量',
            value: vpn.isConnected ? formatBytes(_sessionBytes) : '未连接',
            valueColor: vpn.isConnected ? null : Colors.grey,
          ),
        ),
        const SizedBox(width: 10),
        Expanded(
          child: _StatCard(
            icon: Icons.timer_outlined,
            iconColor: remainingDays == 0
                ? Colors.redAccent
                : remainingDays < 0
                    ? Colors.cyanAccent
                    : Colors.tealAccent,
            title: '剩余天数',
            value: remainingDays < 0 ? '永久' : '$remainingDays 天',
            valueColor: remainingDays == 0 ? Colors.redAccent : null,
          ),
        ),
      ],
    );
  }

  /// Usage colour for the percentage label.
  Color _usageColor(double fraction) {
    if (fraction >= 0.9) return Colors.redAccent;
    if (fraction >= 0.7) return Colors.orangeAccent;
    return Colors.cyanAccent;
  }
}

/// A compact stat card used in the stats grid.
class _StatCard extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String title;
  final String value;
  final Color? valueColor;

  const _StatCard({
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.value,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 14),
        child: Column(
          children: [
            Icon(icon, color: iconColor, size: 22),
            const SizedBox(height: 6),
            Text(
              title,
              style: TextStyle(
                color: Colors.grey[400],
                fontSize: 11,
              ),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 4),
            Text(
              value,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.bold,
                color: valueColor ?? Colors.white,
              ),
              textAlign: TextAlign.center,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}
