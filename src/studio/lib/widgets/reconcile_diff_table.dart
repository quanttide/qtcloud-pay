import 'package:flutter/material.dart';

import '../models/reconciliation.dart';
import 'money_text.dart';

/// 对账差异表：差异行定位（账户、余额 vs 期望值）+ 跳转该账户流水。
class ReconcileDiffTable extends StatelessWidget {
  final List<Discrepancy> discrepancies;
  final void Function(String accountId)? onJump;

  const ReconcileDiffTable({super.key, required this.discrepancies, this.onJump});

  @override
  Widget build(BuildContext context) {
    if (discrepancies.isEmpty) {
      return const Center(child: Text('一致性校验通过，无差异 ✅'));
    }
    return Column(
      children: [
        for (final d in discrepancies)
          Card(
            margin: const EdgeInsets.only(bottom: 8),
            child: ListTile(
              leading: Icon(Icons.error_outline, color: Colors.red.shade700),
              title: Text('账户 ${d.accountId} 余额不一致'),
              subtitle: Row(
                children: [
                  Text('余额 '),
                  MoneyText(d.balance),
                  const Text('  vs  期望 '),
                  MoneyText(d.expected),
                ],
              ),
              trailing: onJump == null
                  ? null
                  : TextButton(
                      onPressed: () => onJump!(d.accountId),
                      child: const Text('查看流水'),
                    ),
            ),
          ),
      ],
    );
  }
}
