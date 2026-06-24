import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../services/auth_service.dart';
import '../../widgets/loading_overlay.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key});

  @override
  State<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  // Account login controllers
  final _formKey = GlobalKey<FormState>();
  final _usernameController = TextEditingController();
  final _passwordController = TextEditingController();
  bool _obscurePassword = true;

  // Token login controllers
  final _tokenFormKey = GlobalKey<FormState>();
  final _tokenController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
  }

  @override
  void dispose() {
    _tabController.dispose();
    _usernameController.dispose();
    _passwordController.dispose();
    _tokenController.dispose();
    super.dispose();
  }

  Future<void> _login() async {
    if (!_formKey.currentState!.validate()) return;

    final auth = context.read<AuthService>();
    auth.clearError();

    final success = await auth.login(
      _usernameController.text.trim(),
      _passwordController.text,
    );

    if (!mounted) return;

    if (success) {
      Navigator.pushReplacementNamed(context, '/main');
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(auth.error ?? '登录失败'),
          backgroundColor: Colors.red.shade800,
        ),
      );
    }
  }

  Future<void> _tokenLogin() async {
    if (!_tokenFormKey.currentState!.validate()) return;

    final auth = context.read<AuthService>();
    auth.clearError();

    final success = await auth.tokenLogin(
      _tokenController.text.trim(),
    );

    if (!mounted) return;

    if (success) {
      Navigator.pushReplacementNamed(context, '/main');
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(auth.error ?? 'Token 登录失败'),
          backgroundColor: Colors.red.shade800,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Stack(
        children: [
          Center(
            child: SingleChildScrollView(
              padding: const EdgeInsets.all(24),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(
                    Icons.flight_takeoff,
                    size: 80,
                    color: Theme.of(context).colorScheme.primary,
                  ),
                  const SizedBox(height: 16),
                  Text(
                    'RFPlay Airport',
                    style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                          fontWeight: FontWeight.bold,
                          color: Theme.of(context).colorScheme.primary,
                        ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    '管理控制台',
                    style: Theme.of(context).textTheme.bodyLarge?.copyWith(
                          color: Colors.grey[400],
                        ),
                  ),
                  const SizedBox(height: 48),
                  TabBar(
                    controller: _tabController,
                    tabs: const [
                      Tab(text: '账号登录'),
                      Tab(text: 'Token 导入'),
                    ],
                    indicatorSize: TabBarIndicatorSize.tab,
                  ),
                  const SizedBox(height: 24),
                  SizedBox(
                    height: 300,
                    child: TabBarView(
                      controller: _tabController,
                      children: [
                        // Account login tab
                        _buildAccountLoginForm(),
                        // Token login tab
                        _buildTokenLoginForm(),
                      ],
                    ),
                  ),
                  const SizedBox(height: 16),
                  TextButton(
                    onPressed: () {
                      Navigator.pushNamed(context, '/register');
                    },
                    child: const Text('没有账号？立即注册'),
                  ),
                ],
              ),
            ),
          ),
          Consumer<AuthService>(
            builder: (context, auth, _) {
              return LoadingOverlay(
                isLoading: auth.isLoading,
                message: '登录中...',
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildAccountLoginForm() {
    return Form(
      key: _formKey,
      child: Column(
        children: [
          TextFormField(
            controller: _usernameController,
            decoration: InputDecoration(
              labelText: '用户名',
              prefixIcon: const Icon(Icons.person_outline),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return '请输入用户名';
              }
              return null;
            },
          ),
          const SizedBox(height: 16),
          TextFormField(
            controller: _passwordController,
            obscureText: _obscurePassword,
            decoration: InputDecoration(
              labelText: '密码',
              prefixIcon: const Icon(Icons.lock_outline),
              suffixIcon: IconButton(
                icon: Icon(
                  _obscurePassword
                      ? Icons.visibility_off
                      : Icons.visibility,
                ),
                onPressed: () {
                  setState(() {
                    _obscurePassword = !_obscurePassword;
                  });
                },
              ),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            validator: (value) {
              if (value == null || value.isEmpty) {
                return '请输入密码';
              }
              return null;
            },
          ),
          const SizedBox(height: 24),
          Consumer<AuthService>(
            builder: (context, auth, _) {
              return SizedBox(
                width: double.infinity,
                height: 48,
                child: ElevatedButton(
                  onPressed: auth.isLoading ? null : _login,
                  style: ElevatedButton.styleFrom(
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: const Text('登录', style: TextStyle(fontSize: 16)),
                ),
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildTokenLoginForm() {
    return Form(
      key: _tokenFormKey,
      child: Column(
        children: [
          TextFormField(
            controller: _tokenController,
            maxLines: 3,
            decoration: InputDecoration(
              labelText: '导入 Token',
              hintText: '请输入 rf_ 或 at_ 开头的 token',
              prefixIcon: const Icon(Icons.vpn_key_outlined),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
              ),
            ),
            validator: (value) {
              if (value == null || value.trim().isEmpty) {
                return '请输入 Token';
              }
              final trimmed = value.trim();
              if (!trimmed.startsWith('rf_') && !trimmed.startsWith('at_')) {
                return 'Token 格式无效，应以 rf_ 或 at_ 开头';
              }
              return null;
            },
          ),
          const SizedBox(height: 12),
          Text(
            '将您的订阅 token 粘贴到上方输入框中即可快速登录',
            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                  color: Colors.grey[500],
                ),
          ),
          const SizedBox(height: 24),
          Consumer<AuthService>(
            builder: (context, auth, _) {
              return SizedBox(
                width: double.infinity,
                height: 48,
                child: ElevatedButton(
                  onPressed: auth.isLoading ? null : _tokenLogin,
                  style: ElevatedButton.styleFrom(
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                  ),
                  child: const Text('导入并登录', style: TextStyle(fontSize: 16)),
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
