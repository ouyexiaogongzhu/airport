import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:provider/provider.dart';
import 'services/api_service.dart';
import 'services/auth_service.dart';
import 'services/subscription_service.dart';
import 'services/vpn_service.dart';
import 'screens/dashboard_screen.dart';
import 'screens/login_screen.dart';
import 'screens/register_screen.dart';
import 'screens/main_shell.dart';
import 'screens/order_history_screen.dart';
import 'screens/profile_screen.dart';
import 'screens/traffic_screen.dart';
import 'screens/subscription/input_page.dart';
import 'screens/subscription/qr_scanner_page.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Configure API base URL from --dart-define at build time.
  const apiBaseUrl = String.fromEnvironment('API_BASE_URL');
  if (apiBaseUrl.isNotEmpty) {
    ApiService.configure(baseUrl: apiBaseUrl);
  }

  final authService = AuthService();
  await authService.init();

  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: authService),
        ChangeNotifierProvider(create: (_) => SubscriptionService()),
        ChangeNotifierProvider(create: (_) => VpnService()),
      ],
      child: RFPlayApp(authService: authService),
    ),
  );
}

class RFPlayApp extends StatelessWidget {
  final AuthService authService;

  const RFPlayApp({super.key, required this.authService});

  @override
  Widget build(BuildContext context) {
    final router = _buildRouter(context);

    return MaterialApp.router(
      title: 'RFPlay Airport',
      debugShowCheckedModeBanner: false,
      routerConfig: router,
      theme: _buildTheme(),
    );
  }

  GoRouter _buildRouter(BuildContext context) {
    return GoRouter(
      initialLocation: authService.isLoggedIn ? '/home' : '/login',
      redirect: (context, state) {
        final loggedIn = authService.isLoggedIn;
        final isAuthRoute = state.matchedLocation == '/login' ||
            state.matchedLocation == '/register';

        if (!loggedIn && !isAuthRoute) return '/login';
        if (loggedIn && isAuthRoute) return '/home';
        return null;
      },
      routes: [
        GoRoute(
          path: '/login',
          builder: (context, state) => const LoginScreen(),
        ),
        GoRoute(
          path: '/register',
          builder: (context, state) => const RegisterScreen(),
        ),
        StatefulShellRoute.indexedStack(
          builder: (context, state, navigationShell) =>
              MainShell(navigationShell: navigationShell),
          branches: [
            // Home tab — dashboard
            StatefulShellBranch(
              routes: [
                GoRoute(
                  path: '/home',
                  builder: (context, state) => const DashboardScreen(),
                ),
              ],
            ),
            // Subscription tab — import / manage nodes
            StatefulShellBranch(
              routes: [
                GoRoute(
                  path: '/subscription',
                  builder: (context, state) => const SubscriptionInputPage(),
                  routes: [
                    GoRoute(
                      path: 'qr',
                      pageBuilder: (context, state) => CustomTransitionPage(
                        key: state.pageKey,
                        child: const QrScannerPage(),
                        transitionsBuilder:
                            (context, animation, secondaryAnimation, child) {
                          return SlideTransition(
                            position: Tween<Offset>(
                              begin: const Offset(1, 0),
                              end: Offset.zero,
                            ).animate(animation),
                            child: child,
                          );
                        },
                      ),
                    ),
                  ],
                ),
              ],
            ),
            // Settings tab — profile
            StatefulShellBranch(
              routes: [
                GoRoute(
                  path: '/settings',
                  builder: (context, state) => const ProfileScreen(),
                ),
              ],
            ),
          ],
        ),
        // Standalone routes (pushed on top of shell)
        GoRoute(
          path: '/orders',
          builder: (context, state) => const OrderHistoryScreen(),
        ),
        GoRoute(
          path: '/traffic',
          builder: (context, state) => const TrafficScreen(),
        ),
      ],
    );
  }

  ThemeData _buildTheme() {
    return ThemeData(
      brightness: Brightness.dark,
      colorScheme: ColorScheme.dark(
        primary: Colors.cyanAccent,
        secondary: Colors.tealAccent,
        surface: const Color(0xFF1E1E2C),
        onPrimary: Colors.black,
        onSecondary: Colors.black,
        onSurface: Colors.white,
      ),
      scaffoldBackgroundColor: const Color(0xFF121220),
      cardTheme: CardThemeData(
        color: const Color(0xFF1E1E2C),
        elevation: 2,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(12),
        ),
      ),
      appBarTheme: const AppBarTheme(
        backgroundColor: Color(0xFF1A1A2E),
        elevation: 0,
        centerTitle: true,
      ),
      navigationBarTheme: NavigationBarThemeData(
        backgroundColor: const Color(0xFF1A1A2E),
        indicatorColor: Colors.cyanAccent.withAlpha(40),
        labelTextStyle: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return TextStyle(
              color: Colors.cyanAccent,
              fontSize: 12,
              fontWeight: FontWeight.w600,
            );
          }
          return TextStyle(color: Colors.grey[400], fontSize: 12);
        }),
        iconTheme: WidgetStateProperty.resolveWith((states) {
          if (states.contains(WidgetState.selected)) {
            return const IconThemeData(color: Colors.cyanAccent);
          }
          return IconThemeData(color: Colors.grey[400]);
        }),
      ),
      inputDecorationTheme: InputDecorationTheme(
        filled: true,
        fillColor: const Color(0xFF2A2A3E),
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: const BorderSide(color: Colors.cyanAccent, width: 1.5),
        ),
        labelStyle: TextStyle(color: Colors.grey[400]),
      ),
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: Colors.cyanAccent,
          foregroundColor: Colors.black,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: Colors.cyanAccent,
        ),
      ),
      snackBarTheme: SnackBarThemeData(
        behavior: SnackBarBehavior.floating,
        shape: RoundedRectangleBorder(
          borderRadius: BorderRadius.circular(8),
        ),
      ),
    );
  }
}
