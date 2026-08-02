import 'package:flutter/material.dart';

/// 金额展示（分 → 元）。全链路整数分，无浮点误差。
///
/// - [amount] 为整数分，可传负数表示扣减
/// - 方向着色：正数绿（充值 +）、负数红（消费 −）
/// - [showSign] 为 true 时显示 +/- 符号
class MoneyText extends StatelessWidget {
  final int amount; // 分
  final bool showSign;
  final Color? positiveColor;
  final Color? negativeColor;
  final TextStyle? style;

  const MoneyText(
    this.amount, {
    super.key,
    this.showSign = false,
    this.positiveColor,
    this.negativeColor,
    this.style,
  });

  static String format(int cents) => (cents / 100).toStringAsFixed(2);

  @override
  Widget build(BuildContext context) {
    final negative = amount < 0;
    final color = negative
        ? (negativeColor ?? Colors.red.shade700)
        : (positiveColor ?? Colors.green.shade700);
    final sign = showSign && amount != 0 ? (negative ? '-' : '+') : '';
    return Text(
      '$sign¥${format(amount.abs())}',
      style: (style ?? Theme.of(context).textTheme.bodyMedium)
          ?.copyWith(color: color, fontWeight: FontWeight.w600),
    );
  }
}
