import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../models/user.dart';
import '../../services/auth_service.dart';
import '../../widgets/status_badge.dart';

class AccountProfile extends StatefulWidget {
  const AccountProfile({super.key});

  @override
  State<AccountProfile> createState() => _AccountProfileState();
}

class _AccountProfileState extends State<AccountProfile> {
  bool _refreshing = false;

  Future<void> _refreshProfile() async {
    setState(() => _refreshing = true);
    await context.read<AuthService>().init();
    if (mounted) {
      setState(() => _refreshing = false);
    }
  }

  String _initials(String name) {
    if (name.isEmpty) return 'U';
    return name[0].toUpperCase();
  }

  StatusBadge _roleBadge(User? user) {
    if (user?.role == 'admin') {
      return StatusBadge.warning(label: '管理员');
    }
    return StatusBadge(
      label: '普通用户',
      color: Colors.blue,
      showDot: true,
    );
  }

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthService>();
    final user = auth.currentUser;

    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              '个人资料',
              style: Theme.of(context).textTheme.headlineSmall?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
            ),
            IconButton(
              onPressed: _refreshing ? null : _refreshProfile,
              icon: _refreshing
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.refresh),
              tooltip: '刷新资料',
            ),
          ],
        ),
        const SizedBox(height: 24),
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
              const SizedBox(height: 8),
              _roleBadge(user),
            ],
          ),
        ),
        const SizedBox(height: 32),
        _ProfileInfoRow(
          icon: Icons.email_outlined,
          label: '邮箱',
          value: user?.email ?? '-',
        ),
        _ProfileInfoRow(
          icon: Icons.badge_outlined,
          label: '用户 ID',
          value: '#${user?.id ?? '-'}',
        ),
        _ProfileInfoRow(
          icon: Icons.calendar_today,
          label: '注册时间',
          value: user?.createdAt.isNotEmpty == true
              ? user!.createdAt.substring(0, 10)
              : '-',
        ),
        const SizedBox(height: 32),
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
      ],
    );
  }
}

class _ProfileInfoRow extends StatelessWidget {
  final IconData icon;
  final String label;
  final String value;

  const _ProfileInfoRow({
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
