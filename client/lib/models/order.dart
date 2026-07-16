// TODO(L1): Implement operator == and hashCode for value-based equality.
// Consider using package:equatable or manually overriding these.
class OrderInfo {
  final int id;
  final String plan;
  final double amount;
  final String status;
  final int createdAt;
  final int? paidAt;
  final String? gateway;

  OrderInfo({
    required this.id,
    required this.plan,
    required this.amount,
    required this.status,
    required this.createdAt,
    this.paidAt,
    this.gateway,
  });

  factory OrderInfo.fromJson(Map<String, dynamic> json) {
    return OrderInfo(
      id: json['id'] as int,
      plan: json['plan'] as String? ?? 'unknown',
      amount: (json['amount'] as num?)?.toDouble() ?? 0.0,
      status: json['status'] as String? ?? 'pending',
      createdAt: (json['created_at'] as num?)?.toInt() ?? 0,
      paidAt: (json['paid_at'] as num?)?.toInt(),
      gateway: json['gateway'] as String?,
    );
  }

  String get statusLabel {
    switch (status) {
      case 'paid':
      case 'completed':
        return '已完成';
      case 'pending':
        return '待支付';
      case 'cancelled':
        return '已取消';
      case 'expired':
        return '已过期';
      case 'refunded':
        return '已退款';
      default:
        return status;
    }
  }

  bool get isPaid => status == 'paid' || status == 'completed';

  String get formattedAmount => '\$${amount.toStringAsFixed(2)}';

  String get formattedDate {
    if (createdAt <= 0) return '—';
    final dt = DateTime.fromMillisecondsSinceEpoch(createdAt * 1000);
    final month = dt.month.toString().padLeft(2, '0');
    final day = dt.day.toString().padLeft(2, '0');
    return '${dt.year}-$month-$day';
  }
}
