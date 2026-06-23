import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import 'package:url_launcher/url_launcher.dart';
import '../models/subscription.dart';
import '../services/subscription_service.dart';
import '../services/vpn_service.dart';

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
    // Also ping nodes
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
          // Mode toggle
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
                    Icon(Icons.phone_android, size: 18,
                      color: vpn.mode == VpnMode.builtin ? Colors.cyanAccent : null),
                    const SizedBox(width: 8),
                    const Text('内置 VPN'),
                  ],
                ),
              ),
              PopupMenuItem(
                value: VpnMode.external,
                child: Row(
                  children: [
                    Icon(Icons.open_in_new, size: 18,
                      color: vpn.mode == VpnMode.external ? Colors.cyanAccent : null),
                    const SizedBox(width: 8),
                    const Text('外部客户端'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: Consumer<SubscriptionService>(
        builder: (context, subService, _) {
          final subscription = subService.subscription;
          final nodes = subscription?.nodes ?? [];

          return RefreshIndicator(
            onRefresh: _refresh,
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                // --- Connection status card ---
                _buildConnectionCard(vpn),
                const SizedBox(height: 16),

                // --- Quick actions (external mode) ---
                if (vpn.mode == VpnMode.external) ...[
                  _buildExternalActions(vpn),
                  const SizedBox(height: 16),
                ],

                // --- Speed cards ---
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

                // --- Nodes section ---
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

                if (subService.isLoading && nodes.isEmpty)
                  const Center(
                    child: Padding(
                      padding: EdgeInsets.all(24),
                      child: CircularProgressIndicator(),
                    ),
                  )
                else if (nodes.isEmpty)
                  Center(
                    child: Padding(
                      padding: const EdgeInsets.all(24),
                      child: Text(
                        '暂无可用节点\n请先购买套餐',
                        textAlign: TextAlign.center,
                        style: TextStyle(color: Colors.grey[500]),
                      ),
                    ),
                  )
                else
                  ...nodes.map((node) => _NodeTile(
                        node: node,
                        latency: vpn.latencies[node.name],
                        isSelected: vpn.selectedNode == node.name,
                        isConnected: vpn.connectedNode == node.name && vpn.isConnected,
                        onTap: () => vpn.selectNode(node.name),
                        onPing: () => _pingNode(vpn, node),
                        onCopy: () => _copyToClipboard(node.uri),
                      )),

                const SizedBox(height: 16),

                // Connection info
                if (vpn.isConnected && vpn.connectedNode != null)
                  Center(
                    child: Text(
                      '已连接至: ${vpn.connectedNode}',
                      style: TextStyle(color: Colors.grey[400], fontSize: 12),
                    ),
                  ),
                const SizedBox(height: 8),

                // Error message
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
    );
  }

  Widget _buildConnectionCard(VpnService vpn) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 24, horizontal: 16),
        child: Column(
          children: [
            // Toggle button
            GestureDetector(
              onTap: () => vpn.toggle(),
              child: SizedBox(
                width: 120,
                height: 120,
                child: Stack(
                  alignment: Alignment.center,
                  children: [
                    Container(
                      width: 120,
                      height: 120,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: vpn.isConnected
                            ? Colors.green
                            : vpn.isConnecting
                                ? Colors.grey[700]
                                : Colors.grey[600],
                        boxShadow: [
                          BoxShadow(
                            color: (vpn.isConnected
                                    ? Colors.green
                                    : Colors.grey)
                                .withAlpha(80),
                            blurRadius: 16,
                            spreadRadius: 2,
                          ),
                        ],
                      ),
                      child: Icon(
                        vpn.isConnected
                            ? Icons.vpn_lock
                            : Icons.vpn_lock_outlined,
                        size: 48,
                        color: Colors.white,
                      ),
                    ),
                    if (vpn.isConnecting)
                      SizedBox(
                        width: 120,
                        height: 120,
                        child: CircularProgressIndicator(
                          strokeWidth: 3,
                          color: Colors.cyanAccent.withAlpha(180),
                        ),
                      ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 16),
            // Status text
            Text(
              vpn.isConnecting
                  ? '连接中...'
                  : vpn.isConnected
                      ? '已连接'
                      : '已断开',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.w600,
                    color: vpn.isConnected
                        ? Colors.green
                        : vpn.isConnecting
                            ? Colors.cyanAccent
                            : Colors.grey[400],
                  ),
            ),
            if (vpn.isConnected) ...[
              const SizedBox(height: 4),
              Text(
                '连接时长: ${_formatDuration(vpn.connectedTime)}',
                style: TextStyle(color: Colors.grey[400], fontSize: 13),
              ),
            ],
            // Mode indicator
            const SizedBox(height: 8),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
              decoration: BoxDecoration(
                color: Colors.grey[800],
                borderRadius: BorderRadius.circular(12),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    vpn.mode == VpnMode.builtin ? Icons.phone_android : Icons.open_in_new,
                    size: 14,
                    color: Colors.grey[400],
                  ),
                  const SizedBox(width: 4),
                  Text(
                    vpn.mode == VpnMode.builtin ? '内置 VPN' : '外部客户端',
                    style: TextStyle(color: Colors.grey[400], fontSize: 11),
                  ),
                ],
              ),
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
            // External app links
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
    // Parse node address for ping
    // In production, the node model would have structured address
    await vpn.pingNode(node.name, '8.8.8.8', 53); // Placeholder
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
    // Copy the subscription URL (from /client/config or portal)
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

// --- Sub-widgets ---

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

class _NodeTile extends StatelessWidget {
  final VpnNode node;
  final NodeLatency? latency;
  final bool isSelected;
  final bool isConnected;
  final VoidCallback onTap;
  final VoidCallback onPing;
  final VoidCallback onCopy;

  const _NodeTile({
    required this.node,
    this.latency,
    required this.isSelected,
    required this.isConnected,
    required this.onTap,
    required this.onPing,
    required this.onCopy,
  });

  Color _latencyColor(int ms) {
    if (ms < 0) return Colors.grey;
    if (ms < 100) return Colors.green;
    if (ms < 300) return Colors.orange;
    return Colors.red;
  }

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      color: isConnected
          ? Colors.green.withAlpha(25)
          : isSelected
              ? Colors.cyanAccent.withAlpha(25)
              : Theme.of(context).cardTheme.color,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(12),
        side: isConnected
            ? BorderSide(color: Colors.green.withAlpha(120))
            : isSelected
                ? BorderSide(color: Colors.cyanAccent.withAlpha(120))
                : BorderSide.none,
      ),
      child: ListTile(
        onTap: onTap,
        leading: Stack(
          children: [
            Icon(
              Icons.public,
              color: isConnected
                  ? Colors.green
                  : isSelected
                      ? Colors.cyanAccent
                      : Colors.grey[400],
              size: 28,
            ),
            if (isConnected)
              Positioned(
                right: -2,
                bottom: -2,
                child: Container(
                  width: 12,
                  height: 12,
                  decoration: const BoxDecoration(
                    color: Colors.green,
                    shape: BoxShape.circle,
                  ),
                ),
              ),
          ],
        ),
        title: Row(
          children: [
            Expanded(
              child: Text(
                node.name,
                style: TextStyle(
                  fontWeight: isSelected || isConnected
                      ? FontWeight.bold
                      : FontWeight.normal,
                  color: isConnected
                      ? Colors.green
                      : isSelected
                          ? Colors.cyanAccent
                          : null,
                ),
              ),
            ),
            // Latency badge
            if (latency != null)
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
                decoration: BoxDecoration(
                  color: _latencyColor(latency!.latencyMs).withAlpha(30),
                  borderRadius: BorderRadius.circular(8),
                  border: Border.all(
                    color: _latencyColor(latency!.latencyMs).withAlpha(80),
                  ),
                ),
                child: Text(
                  latency!.label,
                  style: TextStyle(
                    fontSize: 11,
                    color: _latencyColor(latency!.latencyMs),
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
          ],
        ),
        subtitle: Row(
          children: [
            Expanded(
              child: Text(
                node.uri,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 11,
                  color: isConnected
                      ? Colors.green.withAlpha(180)
                      : isSelected
                          ? Colors.cyanAccent.withAlpha(180)
                          : Colors.grey,
                ),
              ),
            ),
          ],
        ),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            // Copy button
            IconButton(
              icon: Icon(Icons.copy, size: 18, color: Colors.grey[500]),
              onPressed: onCopy,
              padding: EdgeInsets.zero,
              constraints: const BoxConstraints(),
            ),
            const SizedBox(width: 4),
            // Select indicator
            Icon(
              isConnected
                  ? Icons.check_circle
                  : isSelected
                      ? Icons.radio_button_checked
                      : Icons.radio_button_unchecked,
              color: isConnected
                  ? Colors.green
                  : isSelected
                      ? Colors.cyanAccent
                      : Colors.grey,
              size: 20,
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
