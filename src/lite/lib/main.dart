// 量潮钱包（lite）入口：AuthGate 依据本地 JWT 决定登录页/主界面。
// 用户端自服务客户端——查看自己的钱包余额与交易流水（账本核心在 qtcloud-pay provider）。
// API 基址经 --dart-define 注入（QTCLOUD_PAY_BASE_URL / QTCLOUD_AUTH_BASE_URL，本地联调 = localhost）。

import 'package:flutter/material.dart';

import 'screens/login_screen.dart';
import 'screens/wallet_screen.dart';
import 'services/auth_service.dart';

void main() {
  runApp(MainApp(auth: AuthService()));
}

class MainApp extends StatelessWidget {
  const MainApp({super.key, required this.auth});

  final AuthService auth;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: '量潮钱包',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.teal),
      ),
      home: AuthGate(auth: auth),
    );
  }
}

/// 登录闸门：本地有 token 进主界面，否则进登录页；登出回登录页。
class AuthGate extends StatefulWidget {
  const AuthGate({super.key, required this.auth});

  final AuthService auth;

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool? _hasToken;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    final token = await widget.auth.storedToken();
    if (mounted) {
      setState(() => _hasToken = token != null && token.isNotEmpty);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_hasToken == null) {
      return const Scaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (_hasToken == true) {
      return WalletShell(auth: widget.auth, onLogout: _logout);
    }
    return LoginScreen(auth: widget.auth, onSuccess: _login);
  }

  void _login() => setState(() => _hasToken = true);

  void _logout() => setState(() => _hasToken = false);
}

/// 主界面：钱包（账号信息属认证域，走 qtcloud-auth lite）。
class WalletShell extends StatelessWidget {
  const WalletShell({super.key, required this.auth, required this.onLogout});

  final AuthService auth;
  final VoidCallback onLogout;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('钱包'),
        actions: [
          IconButton(
            tooltip: '退出登录',
            icon: const Icon(Icons.logout),
            onPressed: () async {
              await auth.logout();
              onLogout();
            },
          ),
        ],
      ),
      body: WalletScreen(auth: auth),
    );
  }
}
