// 钱包页：余额 + 交易流水（只读，不做在线充值）。
// customerId = auth 用户 sub；生产路径自行拉取 userinfo，测试可注入。

import 'package:flutter/material.dart';

import '../services/account_api.dart';
import '../services/auth_service.dart';
import '../services/wallet_api.dart';

/// 流水类型 → 中文（qtcloud-pay ledger 类型契约）。
String _typeLabel(String type) {
  switch (type) {
    case 'recharge':
      return '充值';
    case 'refund':
      return '退款';
    case 'consume':
      return '消费';
    case 'issue':
      return '发券';
    case 'redeem':
      return '核销';
    default:
      return type;
  }
}

/// 金额方向（与 ledger.SignedAmount 语义一致）。
int _signedAmount(TransactionItem t) {
  switch (t.type) {
    case 'recharge':
      return t.amount;
    case 'refund':
    case 'consume':
      return -t.amount;
    default:
      return 0;
  }
}

String _formatDate(String createdAt) {
  // RFC3339 时间截取日期部分（YYYY-MM-DD）
  if (createdAt.length >= 10) {
    return createdAt.substring(0, 10);
  }
  return createdAt;
}

class WalletScreen extends StatefulWidget {
  const WalletScreen({super.key, required this.auth, this.api, this.customerId});

  final AuthService auth;

  /// 测试注入替身；生产按当前 token 创建。
  final WalletApi? api;

  /// 测试注入用户 ID；生产从 userinfo 获取。
  final String? customerId;

  @override
  State<WalletScreen> createState() => _WalletScreenState();
}

class _WalletScreenState extends State<WalletScreen> {
  WalletApi? _api;
  Wallet? _wallet;
  List<TransactionItem> _transactions = [];
  String? _error;
  bool _noAccount = false;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    try {
      String customerId = '';
      if (widget.api != null) {
        _api = widget.api;
        customerId = widget.customerId ?? '';
      } else {
        final token = await widget.auth.storedToken();
        if (token == null) {
          throw Exception('未登录');
        }
        customerId = widget.customerId ?? '';
        if (customerId.isEmpty) {
          final accountApi = AccountApi(baseUrl: widget.auth.baseUrl, token: token);
          customerId = (await accountApi.fetchUserInfo()).sub;
        }
        _api = WalletApi(
          baseUrl: WalletApi.defaultBaseUrl(),
          token: token,
          customerId: customerId,
        );
      }
      await _load();
    } on WalletNotFoundException {
      // 与 _load 一致：无钱包账户属空态而非故障
      if (mounted) {
        setState(() {
          _noAccount = true;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _error = '加载钱包失败，请稍后重试';
          _loading = false;
        });
      }
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
      _noAccount = false;
    });
    try {
      final wallet = await _api!.fetchWallet();
      final transactions = await _api!.fetchTransactions(wallet.accountId);
      if (mounted) {
        setState(() {
          _wallet = wallet;
          _transactions = transactions;
          _loading = false;
        });
      }
    } on WalletNotFoundException {
      // 该用户尚未开通钱包账户（qtcloud-pay 404），属业务空态
      if (mounted) {
        setState(() {
          _noAccount = true;
          _loading = false;
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _error = '加载钱包失败，请稍后重试';
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_noAccount) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.account_balance_wallet_outlined, size: 56, color: Colors.grey),
            const SizedBox(height: 12),
            const Text('暂无钱包账户'),
            const SizedBox(height: 4),
            const Text(
              '钱包账户由平台开通，暂不支持自助开通',
              style: TextStyle(color: Colors.grey, fontSize: 12),
            ),
          ],
        ),
      );
    }
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!),
            const SizedBox(height: 12),
            FilledButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      );
    }
    final wallet = _wallet!;
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        physics: const AlwaysScrollableScrollPhysics(),
        children: [
          Card(
            margin: const EdgeInsets.all(16),
            child: Padding(
              padding: const EdgeInsets.all(20),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text('钱包余额（元）', style: TextStyle(color: Colors.grey)),
                  const SizedBox(height: 8),
                  Text(
                    wallet.balanceYuan,
                    style: const TextStyle(fontSize: 32, fontWeight: FontWeight.bold),
                  ),
                  const SizedBox(height: 8),
                  const Text('仅展示余额与流水，暂不支持在线充值', style: TextStyle(color: Colors.grey, fontSize: 12)),
                ],
              ),
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 16),
            child: Text('交易流水', style: Theme.of(context).textTheme.titleMedium),
          ),
          if (_transactions.isEmpty)
            const Padding(
              padding: EdgeInsets.all(24),
              child: Center(child: Text('暂无交易流水')),
            )
          else
            ..._transactions.map((t) {
              final signed = _signedAmount(t);
              final isIncome = signed > 0;
              return ListTile(
                title: Text(_typeLabel(t.type)),
                subtitle: Text(_formatDate(t.createdAt)),
                trailing: Text(
                  signed == 0
                      ? '¥${(t.amount / 100).toStringAsFixed(2)}'
                      : '${isIncome ? '+' : '-'}¥${(t.amount / 100).toStringAsFixed(2)}',
                  style: TextStyle(
                    fontSize: 16,
                    fontWeight: FontWeight.w500,
                    color: isIncome ? Colors.green : (signed < 0 ? Colors.red : Colors.grey),
                  ),
                ),
              );
            }),
        ],
      ),
    );
  }
}
