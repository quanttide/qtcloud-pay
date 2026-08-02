import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/account.dart';
import '../models/coupon.dart';
import '../models/transaction.dart';
import '../models/voucher.dart';
import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/money_text.dart';
import '../widgets/status_chip.dart';
import '../widgets/transaction_list.dart';

/// 账户详情页：余额、交易流水、券列表（Tabs）。
class AccountDetailScreen extends StatefulWidget {
  final String accountId;

  const AccountDetailScreen({super.key, required this.accountId});

  @override
  State<AccountDetailScreen> createState() => _AccountDetailScreenState();
}

class _AccountDetailScreenState extends State<AccountDetailScreen> {
  Account? _account;
  List<Transaction>? _transactions;
  List<Coupon>? _coupons;
  List<Voucher>? _vouchers;
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final store = context.read<PayStore>();
      final api = store.api;
      final account = await api.getAccount(widget.accountId);
      store.cacheAccount(account);
      final txs =
          await api.listTransactions(widget.accountId, limit: 50);
      store.cacheTransactions(widget.accountId, txs);
      final coupons = await api.listCoupons(widget.accountId);
      final vouchers = await api.listVouchers(widget.accountId);
      if (!mounted) return;
      setState(() {
        _account = account;
        _transactions = txs;
        _coupons = coupons;
        _vouchers = vouchers;
      });
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _error = '加载失败：${e.message}');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(_error!, style: TextStyle(color: Colors.red.shade700)),
            TextButton(onPressed: _load, child: const Text('重试')),
          ],
        ),
      );
    }
    final account = _account!;
    return DefaultTabController(
      length: 3,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(24),
            child: Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('账户 ${account.id}',
                          style: Theme.of(context).textTheme.headlineMedium),
                      const SizedBox(height: 4),
                      Text('客户 ${account.customerId}',
                          style: TextStyle(color: Colors.grey.shade600)),
                    ],
                  ),
                ),
                Column(
                  crossAxisAlignment: CrossAxisAlignment.end,
                  children: [
                    Text('当前余额',
                        style: TextStyle(color: Colors.grey.shade600)),
                    MoneyText(
                      account.balance,
                      style: Theme.of(context).textTheme.headlineSmall,
                    ),
                  ],
                ),
              ],
            ),
          ),
          const TabBar(
            tabs: [
              Tab(text: '交易流水'),
              Tab(text: '优惠券'),
              Tab(text: '代金券'),
            ],
          ),
          Expanded(
            child: TabBarView(
              children: [
                TransactionList(
                  transactions: _transactions ?? [],
                  onRetry: _load,
                ),
                _couponList(_coupons ?? []),
                _voucherList(_vouchers ?? []),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _couponList(List<Coupon> coupons) {
    if (coupons.isEmpty) return const Center(child: Text('暂无优惠券'));
    return ListView.builder(
      itemCount: coupons.length,
      itemBuilder: (context, i) {
        final c = coupons[i];
        return ListTile(
          dense: true,
          leading: const Icon(Icons.percent),
          title: Text('${couponTypeLabel(c.type)} · ${c.paramLabel}'),
          subtitle: Text(
            '${scopeLabel(c.scope)}${c.productId != null ? ' · ${c.productId}' : ''}'
            ' · 有效期至 ${c.expiresAt.toLocal().toString().split(' ').first}',
          ),
          trailing: StatusChip(status: c.status),
        );
      },
    );
  }

  Widget _voucherList(List<Voucher> vouchers) {
    if (vouchers.isEmpty) return const Center(child: Text('暂无代金券'));
    return ListView.builder(
      itemCount: vouchers.length,
      itemBuilder: (context, i) {
        final v = vouchers[i];
        return ListTile(
          dense: true,
          leading: const Icon(Icons.confirmation_number),
          title: Text('面值 ${MoneyText.format(v.amount)} 元'),
          subtitle: Text(
            '${scopeLabel(v.scope)}${v.productId != null ? ' · ${v.productId}' : ''}'
            ' · 有效期至 ${v.expiresAt.toLocal().toString().split(' ').first}',
          ),
          trailing: StatusChip(status: v.status),
        );
      },
    );
  }
}

String couponTypeLabel(String type) => switch (type) {
      'discount' => '折扣券',
      'full_reduction' => '满减券',
      _ => type,
    };

String scopeLabel(String scope) => switch (scope) {
      'all' => '全场',
      'cloud' => '云服务',
      'course' => '课程',
      'data' => '数据服务',
      'product' => '指定商品',
      _ => scope,
    };
