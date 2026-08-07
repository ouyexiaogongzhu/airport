import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../config.dart';
import '../services/auth_service.dart';
import 'dashboard_screen.dart';
import 'vpn_screen.dart';
import 'profile_screen.dart';

/// Main shell with bottom navigation bar.
///
/// Uses [IndexedStack] for persistent tab state across navigation.
///
/// TODO(M2): The [DashboardScreen], [VpnScreen], and [ProfileScreen] all use
/// const constructors. Ensure new screens added here also use const.
class MainShell extends StatefulWidget {
  const MainShell({super.key});

  @override
  State<MainShell> createState() => _MainShellState();
}

class _MainShellState extends State<MainShell> {
  int _currentIndex = 0;

  final List<Widget> _screens = const [
    DashboardScreen(),
    VpnScreen(),
    ProfileScreen(),
  ];

  @override
  Widget build(BuildContext context) {
    // Guard: if not logged in, redirect to login.
    // ANTI-PATTERN: Calling addPostFrameCallback inside build() mutates state
    // during the build phase. This works but triggers an unnecessary rebuild.
    // Consider using a [Listener] or [NavigatorObserver] to react to auth
    // state changes outside of build, or use an auth-aware routing wrapper.
    //
    // store 模式为通用代理客户端，无账号体系，跳过登录守卫。
    final auth = context.watch<AuthService>();
    if (!auth.isLoggedIn && !AppConfig.storeMode) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        Navigator.pushReplacementNamed(context, '/login');
      });
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }

    return Scaffold(
      body: IndexedStack(
        index: _currentIndex,
        children: _screens,
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _currentIndex,
        onDestinationSelected: (index) {
          setState(() => _currentIndex = index);
        },
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.dashboard_outlined),
            selectedIcon: Icon(Icons.dashboard),
            label: '首页',
          ),
          NavigationDestination(
            icon: Icon(Icons.vpn_key_outlined),
            selectedIcon: Icon(Icons.vpn_key),
            label: 'VPN',
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
