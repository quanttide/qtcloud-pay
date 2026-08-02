import 'package:flutter/material.dart';

/// 幂等键输入：必填 + 唯一性提示（凭证号/批次号/订单号）。
///
/// 对应服务端幂等约定（doc/conventions.md）：
/// - 充值：打款凭证号 voucher_no → transaction.idempotency_key
/// - 发券：发放批次号 batch_no → coupon/voucher.batch_no
/// - 结算：商户订单号 order_id → order.id
class IdempotencyField extends StatelessWidget {
  final TextEditingController controller;
  final String labelText;
  final String hintText;
  final String helperText;

  const IdempotencyField({
    super.key,
    required this.controller,
    required this.labelText,
    required this.hintText,
    this.helperText = '唯一键：重复提交会被服务端拦截（幂等）',
  });

  @override
  Widget build(BuildContext context) {
    return TextFormField(
      controller: controller,
      decoration: InputDecoration(
        labelText: labelText,
        hintText: hintText,
        helperText: helperText,
        border: const OutlineInputBorder(),
      ),
      validator: (v) =>
          (v == null || v.trim().isEmpty) ? '必填（幂等键，请勿重复使用）' : null,
    );
  }
}
