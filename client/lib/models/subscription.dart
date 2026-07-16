// TODO(L1): Implement operator == and hashCode on VpnNode and SubscriptionInfo
// for value-based equality. Consider using package:equatable.
class VpnNode {
  final String name;
  final String uri;

  VpnNode({required this.name, required this.uri});

  factory VpnNode.fromUri(String uri, int index) {
    return VpnNode(
      name: 'Node-${index + 1}',
      uri: uri,
    );
  }
}

class SubscriptionInfo {
  final int id;
  final String tier;
  final int trafficRemainingBytes;
  final int expireTime;
  final List<VpnNode> nodes;
  final Map<String, dynamic> routing;
  final int subscriptionVersion;

  SubscriptionInfo({
    required this.id,
    required this.tier,
    required this.trafficRemainingBytes,
    required this.expireTime,
    required this.nodes,
    required this.routing,
    required this.subscriptionVersion,
  });

  factory SubscriptionInfo.fromJson(Map<String, dynamic> json) {
    final user = json['user'] as Map<String, dynamic>? ?? {};
    final nodeUris = (json['nodes'] as List<dynamic>?)
            ?.map((e) => e.toString())
            .toList() ?? [];
    final nodes = nodeUris
        .asMap()
        .entries
        .map((e) => VpnNode.fromUri(e.value, e.key))
        .toList();

    return SubscriptionInfo(
      id: user['id'] as int? ?? 0,
      tier: user['tier'] as String? ?? 'free',
      trafficRemainingBytes: (user['traffic_remaining_bytes'] as num?)?.toInt() ?? 0,
      expireTime: (user['expire_time'] as num?)?.toInt() ?? 0,
      nodes: nodes,
      routing: (json['routing'] as Map<String, dynamic>?) ?? {},
      subscriptionVersion: json['subscription_version'] as int? ?? 0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'user': {
        'id': id,
        'tier': tier,
        'traffic_remaining_bytes': trafficRemainingBytes,
        'expire_time': expireTime,
      },
      'nodes': nodes.map((n) => n.uri).toList(),
      'routing': routing,
      'subscription_version': subscriptionVersion,
    };
  }

  /// Format bytes to human-readable GB
  String get trafficRemainingGb {
    final gb = trafficRemainingBytes / (1024 * 1024 * 1024);
    return gb.toStringAsFixed(2);
  }

  /// Get total traffic estimate (not available from API, use remaining + 10GB as estimate)
  String get totalTrafficEstimateGb {
    final gb = trafficRemainingBytes / (1024 * 1024 * 1024);
    return gb.toStringAsFixed(2);
  }

  /// Format expire time as DateTime
  DateTime? get expireDateTime {
    if (expireTime <= 0) return null;
    return DateTime.fromMillisecondsSinceEpoch(expireTime * 1000);
  }

  /// Format expire time as readable string
  String get expireDateFormatted {
    if (expireTime <= 0) return '永久';
    final dt = expireDateTime!;
    return '${dt.year}-${dt.month.toString().padLeft(2, '0')}-${dt.day.toString().padLeft(2, '0')}';
  }

  String get tierLabel {
    switch (tier) {
      case 'pro':
        return 'Pro';
      case 'basic':
        return '基础版';
      case 'premium':
        return '高级版';
      default:
        return tier;
    }
  }

  int get nodeCount => nodes.length;
}
