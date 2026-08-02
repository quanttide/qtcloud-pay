import 'package:flutter/material.dart';

import '../models/coupon.dart';
import '../models/order.dart';

/// 状态标签。券：issued/used/expired；订单：created/settled。
class StatusChip extends StatelessWidget {
  final String status;
  final String? label; // 覆盖默认文案

  const StatusChip({super.key, required this.status, this.label});

  Color get _color => switch (status) {
        'issued' || 'settled' => Colors.green,
        'used' => Colors.blue,
        'created' => Colors.orange,
        'expired' => Colors.grey,
        _ => Colors.grey,
      };

  String get _label => label ??
      switch (status) {
        'issued' => VoucherStatus.label(status),
        'used' => VoucherStatus.label(status),
        'expired' => VoucherStatus.label(status),
        'created' => OrderStatus.label(status),
        'settled' => OrderStatus.label(status),
        _ => status,
      };

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
      decoration: BoxDecoration(
        color: _color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Text(
        _label,
        style: TextStyle(fontSize: 11, color: _color),
      ),
    );
  }
}
