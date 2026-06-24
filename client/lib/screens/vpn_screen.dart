import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/subscription.dart';
import '../services/subscription_service.dart';
import '../services/vpn_service.dart';
import '../widgets/loading_overlay.dart';
import '../widgets/node_card.dart';
import '../widgets/status_badge.dart';
import '../widgets/vpn_button.dart';

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
    final subService = context.read<SubscriptionService>();
    final vpn = context.read<VpnService>();
    await subService.loadSubscription();
    if (subService.subscription != null) {
      await vpn.pingAllNodes(subService.subscription!);
    }
  }

  String _formatDuration(Duration duration) {
    final h = duration.inHours.toString().padLeft(2, '0');
    final m = duration.inMinutes.remainder(60).toString().padLeft(2, '0');
    final s = duration.inSeconds.remainder(60).toString().padLeft(2, '0');
    if (duration.inHours > 0) return '$h:$m:$s';
    return '$m:$s';
  }

  @override
  Widget build(BuildContext context) {
    final vpn = context.watch<VpnService>();

    return Scaffold(
      appBar: AppBar(
        title: const Text('VPN 控制'),
        actions: [
          PopupMenuButton<VpnMode>(
            icon: Icon(
              vpn.mode == VpnMode.builtin ? Icons.phone_android : Icons.open_in_new,
              size: 20,
            ),
            tooltip: '连接方式',
            onSelected: (mode) => vpn.setMode(mode),
            itemBuilder: (_) => [
              PopupMenuItem(
                value: VpnMode.builtin,
                child: Row(
                  children: [
                    Icon(
                      Icons.phone_android,
                      size: 18,
                      color: vpn.mode == VpnMode.builtin ? Colors.cyanAccent : null,
                    ),
                    const SizedBox(width: 8),
                    const Text('内置 VPN'),
                  ],
                ),
              ),
              PopupMenuItem(
                value: VpnMode.external,
                child: Row(
                  children: [
                    Icon(
                      Icons.open_in_new,
                      size: 18,
                      color: vpn.mode == VpnMode.external ? Colors.cyanAccent : null,
                    ),
                    const SizedBox(width: 8),
                    const Text('外部客户端'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: Stack(
        children: [
          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final subscription = subService.subscription;
              final nodes = subscription?.nodes ?? [];

              return RefreshIndicator(
                onRefresh: _refresh,
                child: ListView(
                  padding: const EdgeInsets.all(16),
                  children: [
                    _buildConnectionCard(vpn),
                    const SizedBox(height: 16),
                    if (vpn.mode == VpnMode.external) ...[
                      _buildExternalActions(vpn),
                      const SizedBox(height: 16),
                    ],
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
                    Row(
                      mainAxisAlignment: MainAxisAlignment.spaceBetween,
                      children: [
                        Text(
                          '可用节点',
                          style: Theme.of(context).textTheme.titleMedium?.copyWith(
                                fontWeight: FontWeight.bold,
                              ),
                        ),
                        if (nodes.isNotEmpty)
                          TextButton.icon(
                            onPressed: () => _refresh(),
                            icon: const Icon(Icons.refresh, size: 16),
                            label: const Text('测速'),
                          ),
                      ],
                    ),
                    const SizedBox(height: 8),
                    if (nodes.isEmpty)
                      Center(
                        child: Padding(
                          padding: const EdgeInsets.all(24),
                          child: Text(
                            subService.isLoading
                                ? ''
                                : '暂无可用节点\n请先购买套餐',
                            textAlign: TextAlign.center,
                            style: TextStyle(color: Colors.grey[500]),
                          ),
                        ),
                      )
                    else
                      ...nodes.map(
                        (node) => NodeCard(
                          node: node,
                          latency: vpn.latencies[node.name],
                          isSelected: vpn.selectedNode == node.name,
                          isConnected: vpn.connectedNode == node.name && vpn.isConnected,
                          onTap: () => vpn.selectNode(node.name),
                          onPing: () => _pingNode(vpn, node),
                          onCopy: () => _copyToClipboard(node.uri),
                        ),
                      ),
                    const SizedBox(height: 16),
                    if (vpn.isConnected && vpn.connectedNode != null)
                      Center(
                        child: Text(
                          '已连接至: ${vpn.connectedNode}',
                          style: TextStyle(color: Colors.grey[400], fontSize: 12),
                        ),
                      ),
                    const SizedBox(height: 8),
                    if (vpn.errorMessage != null)
                      Center(
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 8),
                          child: Text(
                            vpn.errorMessage!,
                            style: TextStyle(color: Colors.orange[300], fontSize: 13),
                            textAlign: TextAlign.center,
                          ),
                        ),
                      ),
                  ],
                ),
              );
            },
          ),
          Consumer<SubscriptionService>(
            builder: (context, subService, _) {
              final nodes = subService.subscription?.nodes ?? [];
              return LoadingOverlay(
                isLoading: subService.isLoading && nodes.isEmpty,
                message: '加载节点...',
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildConnectionCard(VpnService vpn) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 16),
        child: Column(
          children: [
            VpnButton(
              state: vpn.state,
              onTap: vpn.toggle,
            ),
            if (vpn.isConnected) ...[
              const SizedBox(height: 4),
              Text(
                '连接时长: ${_formatDuration(vpn.connectedTime)}',
                style: TextStyle(color: Colors.grey[400], fontSize: 13),
              ),
            ],
            const SizedBox(height: 8),
            StatusBadge(
              label: vpn.mode == VpnMode.builtin ? '内置 VPN' : '外部客户端',
              icon: vpn.mode == VpnMode.builtin ? Icons.phone_android : Icons.open_in_new,
              color: Colors.grey,
              textColor: Colors.grey[400],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildExternalActions(VpnService vpn) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '快速启动',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: Colors.grey[300],
              ),
            ),
            const SizedBox(height: 12),
            Row(
              children: [
                Expanded(
                  child: _QuickActionButton(
                    icon: Icons.copy,
                    label: '复制订阅',
                    onTap: () => _copySubscriptionToClipboard(context),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: _QuickActionButton(
                    icon: Icons.qr_code,
                    label: '二维码',
                    onTap: () => _showQRCode(context),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: _QuickActionButton(
                    icon: Icons.share,
                    label: '分享',
                    onTap: () => _shareConfig(context),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              '打开方式',
              style: TextStyle(
                fontSize: 12,
                color: Colors.grey[500],
              ),
            ),
            const SizedBox(height: 8),
            _ExternalAppButton(
              icon: Icons.flash_on,
              name: 'v2rayNG',
              subtitle: '通用 V2Ray 客户端',
              onTap: () => _launchExternalApp('v2rayng', context),
            ),
            const SizedBox(height: 4),
            _ExternalAppButton(
              icon: Icons.shield,
              name: 'Clash Meta',
              subtitle: 'Clash 协议客户端',
              onTap: () => _launchExternalApp('clash', context),
            ),
            const SizedBox(height: 4),
            _ExternalAppButton(
              icon: Icons.widgets,
              name: 'Sing-box',
              subtitle: '通用代理客户端',
              onTap: () => _launchExternalApp('singbox', context),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _pingNode(VpnService vpn, VpnNode node) async {
    await vpn.pingNode(node.name, '8.8.8.8', 53);
  }

  void _copyToClipboard(String text) {
    Clipboard.setData(ClipboardData(text: text));
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已复制到剪贴板')),
    );
  }

  Future<void> _copySubscriptionToClipboard(BuildContext context) async {
    final sub = context.read<SubscriptionService>().subscription;
    if (sub == null) return;
    Clipboard.setData(const ClipboardData(text: 'https://rfplay.uk/subscribe/example'));
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('订阅链接已复制到剪贴板')),
      );
    }
  }

  void _showQRCode(BuildContext context) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('QR 码功能开发中')),
    );
  }

  void _shareConfig(BuildContext context) {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('分享功能开发中')),
    );
  }

  Future<void> _launchExternalApp(String scheme, BuildContext context) async {
    final url = '$scheme://';
    final uri = Uri.parse(url);
    if (await canLaunchUrl(uri)) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    } else {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('未找到 $scheme 客户端，请先安装')),
        );
      }
    }
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

class _QuickActionButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _QuickActionButton({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.grey[800],
      borderRadius: BorderRadius.circular(12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: 16, horizontal: 8),
          child: Column(
            children: [
              Icon(icon, color: Colors.cyanAccent, size: 24),
              const SizedBox(height: 6),
              Text(
                label,
                style: TextStyle(
                  fontSize: 12,
                  color: Colors.grey[300],
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ExternalAppButton extends StatelessWidget {
  final IconData icon;
  final String name;
  final String subtitle;
  final VoidCallback onTap;

  const _ExternalAppButton({
    required this.icon,
    required this.name,
    required this.subtitle,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: Colors.grey[850],
      borderRadius: BorderRadius.circular(10),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 12),
          child: Row(
            children: [
              Icon(icon, color: Colors.cyanAccent, size: 22),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      name,
                      style: const TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      subtitle,
                      style: TextStyle(
                        fontSize: 11,
                        color: Colors.grey[500],
                      ),
                    ),
                  ],
                ),
              ),
              Icon(Icons.open_in_new, color: Colors.grey[500], size: 16),
            ],
          ),
        ),
      ),
    );
  }
}
