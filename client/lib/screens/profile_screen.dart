import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../config.dart';
import '../services/auth_service.dart';
import '../services/api_service.dart';

class ProfileScreen extends StatefulWidget {
  const ProfileScreen({super.key});

  @override
  State<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends State<ProfileScreen> {
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    if (AppConfig.storeMode) {
      // store 模式无账号体系，不需要拉取个人资料。
      _loading = false;
      return;
    }
    _loadProfile();
  }

  Future<void> _loadProfile() async {
    setState(() => _loading = true);
    try {
      // TODO(M6): The API response is fetched but never used. Parse the response
      // to populate profile fields (e.g. email, avatar, settings) instead of
      // relying solely on AuthService.currentUser.
      final response = await ApiService().get('/auth/profile');
      debugPrint('[ProfileScreen] profile response: $response');
    } catch (_) {
    }
    if (mounted) {
      setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    // store 模式：通用代理客户端设置页，不展示账号/设备/订单/退出登录。
    if (AppConfig.storeMode) {
      return _buildStoreModeSettings(context);
    }

    final auth = context.watch<AuthService>();
    final user = auth.currentUser;

    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // Header
        Text(
          '个人中心',
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
        const SizedBox(height: 24),

        // Avatar & basic info
        Center(
          child: Column(
            children: [
              CircleAvatar(
                radius: 48,
                backgroundColor: Theme.of(context).colorScheme.primary.withAlpha(30),
                child: Text(
                  _initials(user?.username ?? 'U'),
                  style: TextStyle(
                    fontSize: 32,
                    fontWeight: FontWeight.bold,
                    color: Theme.of(context).colorScheme.primary,
                  ),
                ),
              ),
              const SizedBox(height: 12),
              Text(
                user?.username ?? 'Unknown',
                style: const TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(height: 4),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                decoration: BoxDecoration(
                  color: user?.role == 'admin'
                      ? Colors.amber.withAlpha(25)
                      : Colors.blue.withAlpha(25),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Text(
                  user?.role == 'admin' ? '管理员' : '普通用户',
                  style: TextStyle(
                    color: user?.role == 'admin' ? Colors.amber : Colors.blue,
                    fontSize: 12,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 32),

        // Info cards
        _InfoItem(
          icon: Icons.email_outlined,
          label: '邮箱',
          value: user?.email ?? '-',
        ),
        _InfoItem(
          icon: Icons.badge_outlined,
          label: '用户 ID',
          value: '#${user?.id ?? '-'}',
        ),
        _InfoItem(
          icon: Icons.calendar_today,
          label: '注册时间',
          value: user?.createdAt.isNotEmpty == true
              ? user!.createdAt.substring(0, 10)
              : '-',
        ),
        const SizedBox(height: 32),

        // My Devices
        Card(
          child: ListTile(
            leading: Icon(Icons.devices, color: Theme.of(context).colorScheme.primary),
            title: const Text('我的设备'),
            subtitle: const Text('查看已连接的设备'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => Navigator.pushNamed(context, '/devices'),
          ),
        ),
        const SizedBox(height: 8),

        // Order history button
        SizedBox(
          width: double.infinity,
          height: 48,
          child: OutlinedButton.icon(
            onPressed: () {
              Navigator.pushNamed(context, '/orders');
            },
            icon: const Icon(Icons.receipt_long_outlined),
            label: const Text('订单历史'),
            style: OutlinedButton.styleFrom(
              side: BorderSide(color: Colors.grey[700]!),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
          ),
        ),
        const SizedBox(height: 12),

        // Logout button
        SizedBox(
          width: double.infinity,
          height: 48,
          child: OutlinedButton.icon(
            onPressed: () {
              showDialog(
                context: context,
                builder: (ctx) => AlertDialog(
                  title: const Text('确认退出'),
                  content: const Text('确定要退出登录吗？'),
                  actions: [
                    TextButton(
                      onPressed: () => Navigator.pop(ctx),
                      child: const Text('取消'),
                    ),
                    TextButton(
                      onPressed: () {
                        Navigator.pop(ctx);
                        auth.logout();
                        Navigator.pushReplacementNamed(context, '/login');
                      },
                      child: const Text('退出', style: TextStyle(color: Colors.red)),
                    ),
                  ],
                ),
              );
            },
            icon: const Icon(Icons.logout, color: Colors.red),
            label: const Text('退出登录', style: TextStyle(color: Colors.red)),
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: Colors.red),
              shape: RoundedRectangleBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
          ),
        ),
        const SizedBox(height: 16),

        // Version info
        Center(
          child: Text(
            'RFPlay Airport v1.0.0',
            style: TextStyle(color: Colors.grey[600], fontSize: 12),
          ),
        ),
      ],
    );
  }

  String _initials(String name) {
    if (name.isEmpty) return 'U';
    return name[0].toUpperCase();
  }

  /// store 模式下的设置页：仅保留订阅导入与关于信息。
  Widget _buildStoreModeSettings(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Text(
          '设置',
          style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                fontWeight: FontWeight.bold,
              ),
        ),
        const SizedBox(height: 8),
        Text(
          '通用代理客户端',
          style: TextStyle(color: Colors.grey[400]),
        ),
        const SizedBox(height: 24),
        Card(
          child: ListTile(
            leading: Icon(Icons.link, color: Theme.of(context).colorScheme.primary),
            title: const Text('导入订阅'),
            subtitle: const Text('粘贴订阅链接或扫描二维码导入节点'),
            trailing: const Icon(Icons.chevron_right),
            onTap: () => Navigator.pushNamed(context, '/subscription/input'),
          ),
        ),
        const SizedBox(height: 8),
        Card(
          child: ListTile(
            leading: Icon(
              Icons.info_outline,
              color: Theme.of(context).colorScheme.primary,
            ),
            title: const Text('关于'),
            subtitle: const Text('本应用为通用代理客户端，节点信息由您订阅的服务商提供。'),
          ),
        ),
        const SizedBox(height: 24),
        Center(
          child: Text(
            'RFPlay Airport v1.0.0',
            style: TextStyle(color: Colors.grey[600], fontSize: 12),
          ),
        ),
      ],
    );
  }
}

class _InfoItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;

  const _InfoItem({
    required this.icon,
    required this.label,
    required this.value,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      child: ListTile(
        leading: Icon(icon, color: Theme.of(context).colorScheme.primary),
        title: Text(label, style: TextStyle(color: Colors.grey[400], fontSize: 12)),
        subtitle: Text(value, style: const TextStyle(fontSize: 14)),
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
      ),
    );
  }
}
