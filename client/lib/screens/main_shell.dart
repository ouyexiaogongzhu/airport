import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import '../services/auth_service.dart';
import 'dashboard_screen.dart';
import 'subscription/input_page.dart';
import 'profile_screen.dart';

/// Main shell with bottom navigation bar.
///
/// Uses GoRouter's [StatefulShellRoute.indexedStack] for persistent tab state.
class MainShell extends StatelessWidget {
  final StatefulNavigationShell navigationShell;

  const MainShell({super.key, required this.navigationShell});

  @override
  Widget build(BuildContext context) {
    // Guard: if not logged in, redirect to login
    final auth = context.watch<AuthService>();
    if (!auth.isLoggedIn) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        context.go('/login');
      });
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    return Scaffold(
      body: navigationShell,
      bottomNavigationBar: NavigationBar(
        selectedIndex: navigationShell.currentIndex,
        onDestinationSelected: (index) {
          navigationShell.goBranch(
            index,
            initialLocation: index == navigationShell.currentIndex,
          );
        },
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.dashboard_outlined),
            selectedIcon: Icon(Icons.dashboard),
            label: '首页',
          ),
          NavigationDestination(
            icon: Icon(Icons.cloud_download_outlined),
            selectedIcon: Icon(Icons.cloud_download),
            label: '订阅',
          ),
          NavigationDestination(
            icon: Icon(Icons.person_outline),
            selectedIcon: Icon(Icons.person),
            label: '设置',
          ),
        ],
      ),
    );
  }
}
