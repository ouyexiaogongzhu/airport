import 'package:flutter/material.dart';
import 'package:url_launcher/url_launcher.dart';

// DUPLICATE: This file is an exact copy of screens/register_screen.dart.
// The router imports screens/register_screen.dart — this file is unused
// and can be safely deleted.
class RegisterScreen extends StatelessWidget {
  // TODO: Load portal URL from remote config or SubscriptionService
  static const _kPortalRegisterUrl = 'https://www.rfplay.uk/register';

  const RegisterScreen({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('注册'),
      ),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.open_in_browser, size: 64, color: Colors.cyanAccent),
            const SizedBox(height: 16),
            Text('注册账号', style: Theme.of(context).textTheme.headlineSmall),
            const SizedBox(height: 8),
            Text('请通过网站注册', style: TextStyle(color: Colors.grey[400])),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              icon: const Icon(Icons.open_in_new),
              label: const Text('前往注册'),
              onPressed: () async {
                await launchUrl(
                  Uri.parse(_kPortalRegisterUrl),
                  mode: LaunchMode.externalApplication,
                );
              },
            ),
            const SizedBox(height: 12),
            TextButton(
              onPressed: () => Navigator.pushReplacementNamed(context, '/login'),
              child: const Text('返回登录'),
            ),
          ],
        ),
      ),
    );
  }
}
