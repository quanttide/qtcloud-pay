/// 抵扣类型（与服务端 billing Kind 常量一致）。
class DeductionKind {
  static const coupon = 'coupon'; // 优惠券抵扣
  static const voucher = 'voucher'; // 代金券抵现
  static const balance = 'balance'; // 余额支付

  static String label(String kind) => switch (kind) {
        coupon => '优惠券',
        voucher => '代金券',
        balance => '余额',
        _ => kind,
      };
}

/// 一项抵扣（即订单 settle_detail 的元素）。JSON 契约与服务端 billing.Deduction 一致。
class Deduction {
  final String kind; // coupon / voucher / balance
  final int refId; // json: ref_id 券 ID（balance 时为 0）
  final int amount; // 抵扣额（分）

  const Deduction({required this.kind, required this.refId, required this.amount});

  factory Deduction.fromJson(Map<String, dynamic> json) => Deduction(
        kind: json['kind'] as String,
        refId: (json['ref_id'] as num?)?.toInt() ?? 0,
        amount: json['amount'] as int,
      );
}

/// 计费规则：抵扣顺序配置。JSON 契约与服务端 billing.BillingRule 一致。
class BillingRule {
  final int id;
  final int priority; // 执行顺序（小者先执行）
  final String kind; // coupon / voucher / balance
  final String? condition; // JSON 条件（scope、min_amount 等）

  const BillingRule({
    required this.id,
    required this.priority,
    required this.kind,
    this.condition,
  });

  factory BillingRule.fromJson(Map<String, dynamic> json) => BillingRule(
        id: json['id'] as int,
        priority: json['priority'] as int,
        kind: json['kind'] as String,
        condition: json['condition'] as String?,
      );
}
