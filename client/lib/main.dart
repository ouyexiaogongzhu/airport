import 'dart:io' show Platform;
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'config.dart';
import 'services/api_service.dart';
import 'services/auth_service.dart';
import 'services/subscription_service.dart';
import 'services/vpn_service.dart';
import 'screens/login_screen.dart';
import 'screens/register_screen.dart';
import 'screens/main_shell.dart';
import 'screens/order_history_screen.dart';
import 'screens/traffic_screen.dart';
import 'screens/payment/webview_page.dart';
import 'screens/subscription/input_page.dart';
import 'screens/subscription/qr_scanner_page.dart';
import 'screens/devices/device_list.dart';
import 'screens/account/account_subscription.dart';

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

// TODO(L8): All user-visible strings are currently in Chinese (zh-CN).
// Extract strings to an i18n/l10n system (e.g. flutter_localizations +
// intl) before adding multi-language support.
class RFPlayApp extends StatelessWidget {
  final AuthService authService;

  const RFPlayApp({super.key, required this.authService});

  @override
  Widget build(BuildContext context) {
    final app = MaterialApp(
      title: 'RFPlay Airport',
      debugShowCheckedModeBanner: false,
      initialRoute: _initialRoute,
      routes: _buildRoutes(),
      theme: _buildTheme(),
    );

    if (Platform.isLinux) {
      // 16:9 portrait: height=16, width=9 → width:height = 9:16
      return LayoutBuilder(
        builder: (context, constraints) {
          double w, h;
          final ratio = 9.0 / 16.0; // width / height

          if (constraints.maxWidth / constraints.maxHeight < ratio) {
            // Width-limited
            w = constraints.maxWidth;
            h = w / ratio;
          } else {
            // Height-limited
            h = constraints.maxHeight;
            w = h * ratio;
          }

          return Center(
            child: SizedBox(width: w, height: h, child: app),
          );
        },
      );
    }
    return app;
  }

  /// 首屏路由。
  ///
  /// App Store 版（storeMode）直接进入订阅导入页，无需登录；
  /// 普通版保持原有的「已登录进主界面 / 未登录进登录页」行为。
  String get _initialRoute {
    if (AppConfig.storeMode) return '/subscription/input';
    return authService.isLoggedIn ? '/main' : '/login';
  }

  /// 路由表。
  ///
  /// App Store 版为通用代理客户端：账号、金流、官网相关路由一律不注册，
  /// 改为导向订阅导入页，确保任何旧入口都无法进入这些页面。
  Map<String, WidgetBuilder> _buildRoutes() {
    if (AppConfig.storeMode) {
      return {
        '/main': (context) => const MainShell(),
        '/subscription/input': (context) => const SubscriptionInputPage(),
        '/subscription/qr': (context) => const QrScannerPage(),
        '/login': (context) => const _StoreModeRedirect(),
        '/register': (context) => const _StoreModeRedirect(),
        '/orders': (context) => const _StoreModeRedirect(),
        '/traffic': (context) => const _StoreModeRedirect(),
        '/devices': (context) => const _StoreModeRedirect(),
        '/account/subscription': (context) => const _StoreModeRedirect(),
        '/payment': (context) => const _StoreModeRedirect(),
      };
    }
    return {
      '/login': (context) => const LoginScreen(),
      '/register': (context) => const RegisterScreen(),
      '/main': (context) => const MainShell(),
      '/orders': (context) => const OrderHistoryScreen(),
      '/traffic': (context) => const TrafficScreen(),
      '/devices': (context) => const DeviceList(),
      '/account/subscription': (context) => const AccountSubscription(),
      '/subscription/input': (context) => const SubscriptionInputPage(),
      '/subscription/qr': (context) => const QrScannerPage(),
      '/payment': (context) {
        final args = ModalRoute.of(context)?.settings.arguments;
        if (args is String) {
          return PaymentWebViewPage(url: args);
        }
        return const PaymentWebViewPage(
          url: 'https://www.rfplay.uk/plans',
          title: '支付中心',
        );
      },
    };
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

/// App Store 版（storeMode）中被隐藏的账号/金流/官网路由的占位页。
///
/// 任何旧入口 push 到这些路由时都会立即替换到订阅导入页，
/// 保证通用代理客户端内永远无法进入登录、购买、订单等页面。
class _StoreModeRedirect extends StatelessWidget {
  const _StoreModeRedirect();

  @override
  Widget build(BuildContext context) {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      Navigator.of(context).pushReplacementNamed('/subscription/input');
    });
    return const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    );
  }
}

