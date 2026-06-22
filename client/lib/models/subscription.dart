class Subscription {
  final String planName;
  final String expiryDate;
  final double totalTraffic;
  final double usedTraffic;
  final List<VpnNode> nodes;

  Subscription({
    required this.planName,
    required this.expiryDate,
    required this.totalTraffic,
    required this.usedTraffic,
    required this.nodes,
  });

  factory Subscription.fromJson(Map<String, dynamic> json) {
    return Subscription(
      planName: json['plan_name'] as String? ?? '',
      expiryDate: json['expiry_date'] as String? ?? '',
      totalTraffic: (json['total_traffic'] as num?)?.toDouble() ?? 0,
      usedTraffic: (json['used_traffic'] as num?)?.toDouble() ?? 0,
      nodes: (json['nodes'] as List<dynamic>?)
              ?.map((e) => VpnNode.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'plan_name': planName,
      'expiry_date': expiryDate,
      'total_traffic': totalTraffic,
      'used_traffic': usedTraffic,
      'nodes': nodes.map((e) => e.toJson()).toList(),
    };
  }
}

class VpnNode {
  final String name;
  final String location;
  final int latency;
  final bool isOnline;

  VpnNode({
    required this.name,
    required this.location,
    required this.latency,
    required this.isOnline,
  });

  factory VpnNode.fromJson(Map<String, dynamic> json) {
    return VpnNode(
      name: json['name'] as String? ?? '',
      location: json['location'] as String? ?? '',
      latency: json['latency'] as int? ?? 0,
      isOnline: json['is_online'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'name': name,
      'location': location,
      'latency': latency,
      'is_online': isOnline,
    };
  }
}
