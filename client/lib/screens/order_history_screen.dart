import 'package:flutter/material.dart';
import '../models/order.dart';
import '../services/api_service.dart';

class OrderHistoryScreen extends StatefulWidget {
  const OrderHistoryScreen({super.key});

  @override
  State<OrderHistoryScreen> createState() => _OrderHistoryScreenState();
}

class _OrderHistoryScreenState extends State<OrderHistoryScreen> {
  final ApiService _api = ApiService();
  List<OrderInfo> _orders = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadOrders();
  }

  Future<void> _loadOrders() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final data = await _api.get('/user/orders');
      final list = (data['orders'] as List<dynamic>?)
              ?.map((e) => OrderInfo.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [];
      if (mounted) {
        setState(() {
          _orders = list;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('订单历史'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => Navigator.pop(context),
        ),
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.error_outline, size: 48, color: Colors.grey[600]),
            const SizedBox(height: 16),
            Text(
              '加载失败',
              style: TextStyle(color: Colors.grey[400], fontSize: 16),
            ),
            const SizedBox(height: 4),
            Text(
              _error!,
              style: TextStyle(color: Colors.grey[600], fontSize: 13),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 24),
            FilledButton.tonalIcon(
              onPressed: _loadOrders,
              icon: const Icon(Icons.refresh, size: 18),
              label: const Text('重试'),
            ),
          ],
        ),
      );
    }

    if (_orders.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.receipt_long_outlined, size: 64, color: Colors.grey[700]),
            const SizedBox(height: 16),
            Text(
              '暂无订单记录',
              style: TextStyle(color: Colors.grey[400], fontSize: 16),
            ),
            const SizedBox(height: 8),
            Text(
              '购买套餐后，订单将显示在此处',
              style: TextStyle(color: Colors.grey[600], fontSize: 13),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadOrders,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: _orders.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (context, index) {
          final order = _orders[index];
          return _OrderCard(order: order);
        },
      ),
    );
  }
}

class _OrderCard extends StatelessWidget {
  final OrderInfo order;

  const _OrderCard({required this.order});

  Color _statusColor() {
    if (order.isPaid) return Colors.green;
    if (order.status == 'cancelled' || order.status == 'expired') {
      return Colors.grey;
    }
    if (order.status == 'refunded') return Colors.orange;
    return Colors.amber;
  }

  @override
  Widget build(BuildContext context) {
    final statusColor = _statusColor();

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header row: plan name + status
            Row(
              children: [
                Icon(Icons.receipt, color: Theme.of(context).colorScheme.primary, size: 20),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    order.plan.toUpperCase(),
                    style: const TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.bold,
                    ),
                  ),
                ),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: statusColor.withAlpha(25),
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(color: statusColor.withAlpha(80)),
                  ),
                  child: Text(
                    order.statusLabel,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: statusColor,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),

            // Detail rows
            _DetailRow(label: '订单编号', value: '#${order.id}'),
            const SizedBox(height: 4),
            _DetailRow(label: '金额', value: order.formattedAmount),
            const SizedBox(height: 4),
            _DetailRow(label: '下单时间', value: order.formattedDate),
            if (order.paidAt != null && order.isPaid) ...[
              const SizedBox(height: 4),
              _DetailRow(
                label: '支付时间',
                value: _formatTimestamp(order.paidAt!),
              ),
            ],
            if (order.gateway != null && order.gateway!.isNotEmpty) ...[
              const SizedBox(height: 4),
              _DetailRow(label: '支付方式', value: order.gateway!),
            ],
          ],
        ),
      ),
    );
  }

  String _formatTimestamp(int ts) {
    if (ts <= 0) return '—';
    final dt = DateTime.fromMillisecondsSinceEpoch(ts * 1000);
    final month = dt.month.toString().padLeft(2, '0');
    final day = dt.day.toString().padLeft(2, '0');
    final hour = dt.hour.toString().padLeft(2, '0');
    final min = dt.minute.toString().padLeft(2, '0');
    return '${dt.year}-$month-$day $hour:$min';
  }
}

class _DetailRow extends StatelessWidget {
  final String label;
  final String value;

  const _DetailRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 72,
          child: Text(
            label,
            style: TextStyle(color: Colors.grey[500], fontSize: 13),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: const TextStyle(fontSize: 13),
          ),
        ),
      ],
    );
  }
}
