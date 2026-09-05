import 'package:flutter/material.dart';

class ProfileScreen extends StatelessWidget {
  const ProfileScreen({super.key});

  @override
  Widget build(BuildContext context) {
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
            'RFPlay Proxy v1.0.0',
            style: TextStyle(color: Colors.grey[600], fontSize: 12),
          ),
        ),
      ],
    );
  }
}
