import 'package:flutter/material.dart';

import '../models/transaction.dart';
import 'money_text.dart';

/// 交易流水：类型/金额/时间/来源，任意交易可追溯。
///
/// 方向着色：充值 + 绿、消费 − 红；发券/核销不参与余额求和，金额置灰。
class TransactionList extends StatelessWidget {
  final List<Transaction> transactions;
  final bool loading;
  final String? error;
  final VoidCallback? onRetry;

  const TransactionList({
    super.key,
    required this.transactions,
    this.loading = false,
    this.error,
    this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    if (loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(error!, style: TextStyle(color: Colors.red.shade700)),
            if (onRetry != null)
              TextButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      );
    }
    if (transactions.isEmpty) {
      return const Center(child: Text('暂无交易流水'));
    }
    return ListView.builder(
      itemCount: transactions.length,
      itemBuilder: (context, i) {
        final t = transactions[i];
        final affects = t.affectsBalance;
        return ListTile(
          dense: true,
          leading: Icon(
            switch (t.type) {
              TransactionType.recharge => Icons.add_circle_outline,
              TransactionType.consume => Icons.remove_circle_outline,
              TransactionType.issue => Icons.card_giftcard,
              TransactionType.redeem => Icons.local_offer_outlined,
              _ => Icons.receipt_long,
            },
            color: affects
                ? (t.signedAmount >= 0 ? Colors.green : Colors.red)
                : Colors.grey,
          ),
          title: Text('${TransactionType.label(t.type)}'
              '${t.note != null && t.note!.isNotEmpty ? ' · ${t.note}' : ''}'),
          subtitle: Text(
            '${_fmtTime(t.createdAt)}'
            '${t.orderId != null ? ' · 订单 ${t.orderId}' : ''}',
          ),
          trailing: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              MoneyText(
                t.signedAmount,
                showSign: affects,
                style: Theme.of(context).textTheme.bodyMedium,
              ),
              if (affects)
                Text(
                  '余额 ${MoneyText.format(t.balanceAfter)}',
                  style: TextStyle(fontSize: 11, color: Colors.grey.shade600),
                ),
            ],
          ),
        );
      },
    );
  }

  static String _fmtTime(DateTime t) =>
      '${t.year}-${_pad2(t.month)}-${_pad2(t.day)} ${_pad2(t.hour)}:${_pad2(t.minute)}';

  static String _pad2(int n) => n.toString().padLeft(2, '0');
}
