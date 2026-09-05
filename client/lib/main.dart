import 'dart:async';
import 'dart:io' show Platform;
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'services/subscription_service.dart';
import 'services/vpn_service.dart';
import 'screens/main_shell.dart';
import 'screens/subscription/input_page.dart';
import 'screens/subscription/qr_scanner_page.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  final subService = SubscriptionService();
  await subService.init(); // 读取已保存的订阅/节点链接（不联网）

  // 已有保存的链接：直接进主界面，后台刷新节点。
  final initialRoute = subService.hasSavedSource ? '/main' : '/subscription/input';
  if (subService.hasSavedSource) {
    unawaited(subService.restore());
  }

  runApp(
    MultiProvider(
      providers: [
        ChangeNotifierProvider.value(value: subService),
        ChangeNotifierProvider(create: (_) => VpnService()),
      ],
      child: RFPlayApp(initialRoute: initialRoute),
    ),
  );
}

// TODO(L8): All user-visible strings are currently in Chinese (zh-CN).
// Extract strings to an i18n/l10n system (e.g. flutter_localizations +
// intl) before adding multi-language support.
class RFPlayApp extends StatelessWidget {
  final String initialRoute;

  const RFPlayApp({super.key, required this.initialRoute});

  @override
  Widget build(BuildContext context) {
    final app = MaterialApp(
      title: 'RFPlay Proxy',
      debugShowCheckedModeBanner: false,
      initialRoute: initialRoute,
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

  /// 路由表：无账号通用代理客户端，仅订阅导入 + 主界面。
  Map<String, WidgetBuilder> _buildRoutes() {
    return {
      '/main': (context) => const MainShell(),
      '/subscription/input': (context) => const SubscriptionInputPage(),
      '/subscription/qr': (context) => const QrScannerPage(),
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
