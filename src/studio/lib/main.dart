import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'screens/account_detail_screen.dart';
import 'screens/accounts_screen.dart';
import 'screens/coupon_screen.dart';
import 'screens/dashboard_screen.dart';
import 'screens/order_screen.dart';
import 'screens/recharge_screen.dart';
import 'screens/rule_design_screen.dart';
import 'screens/reconcile_screen.dart';
import 'screens/settings_screen.dart';
import 'services/local_draft_store.dart';
import 'services/pay_api.dart';
import 'services/pay_store.dart';

void main() {
  runApp(const PayStudioApp());
}

/// 量潮支付工作台客户端（qtcloud_pay_studio）。
class PayStudioApp extends StatelessWidget {
  const PayStudioApp({super.key, this.api, this.ruleDesignStore});

  /// 可注入的 API 客户端（测试用）；默认连接 http://localhost:8080。
  final PayApi? api;

  /// 可注入的本地草稿仓（测试用）；默认存 ~/.qtcloud-pay-studio/rule_drafts.json。
  final LocalDraftStore? ruleDesignStore;

  @override
  Widget build(BuildContext context) {
    return MultiProvider(
      providers: [
        Provider<PayApi>.value(value: api ?? PayApi()),
        ChangeNotifierProvider(
          create: (ctx) => PayStore(ctx.read<PayApi>()),
        ),
      ],
      child: MaterialApp(
        title: '量潮支付工作台',
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(seedColor: Colors.indigo),
          useMaterial3: true,
        ),
        home: HomeShell(ruleDesignStore: ruleDesignStore),
      ),
    );
  }
}

/// 应用主框架：左侧导航 + 内容区。
class HomeShell extends StatefulWidget {
  const HomeShell({super.key, this.ruleDesignStore});

  final LocalDraftStore? ruleDesignStore;

  @override
  State<HomeShell> createState() => _HomeShellState();
}

class _HomeShellState extends State<HomeShell> {
  int _index = 0;
  String? _detailAccountId;

  static const _titles = [
    '总览',
    '账户',
    '充值登记',
    '发券',
    '订单结算',
    '对账',
    '参数配置',
    '规则设计',
  ];

  void _navigate(int index) {
    setState(() {
      _index = index;
      _detailAccountId = null;
    });
  }

  void _openAccount(String accountId) {
    setState(() => _detailAccountId = accountId);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(_detailAccountId != null
            ? '账户详情 · $_detailAccountId'
            : '量潮支付工作台 · ${_titles[_index]}'),
      ),
      body: Row(
        children: [
          NavigationRail(
            selectedIndex: _index,
            onDestinationSelected: _navigate,
            labelType: NavigationRailLabelType.all,
            destinations: const [
              NavigationRailDestination(
                icon: Icon(Icons.dashboard_outlined),
                selectedIcon: Icon(Icons.dashboard),
                label: Text('总览'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.account_balance_wallet_outlined),
                selectedIcon: Icon(Icons.account_balance_wallet),
                label: Text('账户'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.add_card_outlined),
                selectedIcon: Icon(Icons.add_card),
                label: Text('充值登记'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.card_giftcard_outlined),
                selectedIcon: Icon(Icons.card_giftcard),
                label: Text('发券'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.receipt_long_outlined),
                selectedIcon: Icon(Icons.receipt_long),
                label: Text('订单结算'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.fact_check_outlined),
                selectedIcon: Icon(Icons.fact_check),
                label: Text('对账'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.settings_outlined),
                selectedIcon: Icon(Icons.settings),
                label: Text('参数配置'),
              ),
              NavigationRailDestination(
                icon: Icon(Icons.edit_note_outlined),
                selectedIcon: Icon(Icons.edit_note),
                label: Text('规则设计'),
              ),
            ],
          ),
          const VerticalDivider(width: 1),
          Expanded(
            child: _detailAccountId != null
                ? AccountDetailScreen(accountId: _detailAccountId!)
                : _buildPage(_index),
          ),
        ],
      ),
    );
  }

  Widget _buildPage(int index) {
    switch (index) {
      case 0:
        return DashboardScreen(onNavigate: _navigate);
      case 1:
        return AccountsScreen(onOpenDetail: _openAccount);
      case 2:
        return const RechargeScreen();
      case 3:
        return const CouponScreen();
      case 4:
        return const OrderScreen();
      case 5:
        return ReconcileScreen(onOpenAccount: _openAccount);
      case 6:
        return const SettingsScreen();
      case 7:
        return RuleDesignScreen(store: widget.ruleDesignStore);
      default:
        return const SizedBox.shrink();
    }
  }
}
