import 'package:flutter/material.dart';

import '../models/billing_rule.dart';
import 'money_text.dart';

/// 结算明细面板：逐项列出抵扣（优惠券 → 代金券 → 余额）与余额变化。
class SettleDetailPanel extends StatelessWidget {
  final List<Deduction> deductions;
  final int orderAmount; // 订单金额（分）

  const SettleDetailPanel({
    super.key,
    required this.deductions,
    required this.orderAmount,
  });

  @override
  Widget build(BuildContext context) {
    final totalDeducted = deductions.fold<int>(0, (sum, d) => sum + d.amount);
    return Card(
      margin: EdgeInsets.zero,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('结算明细',
                style: Theme.of(context).textTheme.titleSmall),
            const SizedBox(height: 8),
            for (final d in deductions)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(
                  children: [
                    Icon(
                      switch (d.kind) {
                        DeductionKind.coupon => Icons.percent,
                        DeductionKind.voucher => Icons.confirmation_number,
                        _ => Icons.account_balance_wallet,
                      },
                      size: 18,
                      color: Colors.blueGrey,
                    ),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        '${DeductionKind.label(d.kind)}'
                        '${d.kind == DeductionKind.balance ? '' : ' #${d.refId}'}',
                      ),
                    ),
                    MoneyText(-d.amount, showSign: true),
                  ],
                ),
              ),
            const Divider(),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('订单金额',
                    style: Theme.of(context).textTheme.bodyMedium),
                MoneyText(orderAmount),
              ],
            ),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('合计抵扣', style: Theme.of(context).textTheme.bodyMedium),
                MoneyText(-totalDeducted, showSign: true),
              ],
            ),
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text('余额实付', style: Theme.of(context).textTheme.bodyMedium),
                MoneyText(orderAmount - totalDeducted),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
