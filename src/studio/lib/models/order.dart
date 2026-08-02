import 'billing_rule.dart' show Deduction;

/// 订单状态（与服务端 order Status 常量一致）。
class OrderStatus {
  static const created = 'created'; // 已创建
  static const settled = 'settled'; // 已结算

  static String label(String status) => switch (status) {
        created => '已创建',
        settled => '已结算',
        _ => status,
      };
}

/// 客户购买付费服务的交易请求。JSON 契约与服务端 order.Order 一致。
class Order {
  final String id; // 商户订单号（幂等键）
  final String customerId; // json: customer_id
  final String accountId; // json: account_id
  final String? productId; // json: product_id
  final String? scope; // 业务类型：cloud / course / data
  final int amount; // 订单金额（分）
  final String status; // created / settled
  final List<Deduction> settleDetail; // json: settle_detail 结算计划快照（逐项抵扣）
  final DateTime createdAt; // json: created_at
  final DateTime? settledAt; // json: settled_at

  const Order({
    required this.id,
    required this.customerId,
    required this.accountId,
    this.productId,
    this.scope,
    required this.amount,
    required this.status,
    required this.settleDetail,
    required this.createdAt,
    this.settledAt,
  });

  factory Order.fromJson(Map<String, dynamic> json) => Order(
        id: json['id'] as String,
        customerId: json['customer_id'] as String,
        accountId: json['account_id'] as String,
        productId: json['product_id'] as String?,
        scope: json['scope'] as String?,
        amount: json['amount'] as int,
        status: json['status'] as String,
        settleDetail: (json['settle_detail'] as List? ?? [])
            .map((e) => Deduction.fromJson(e as Map<String, dynamic>))
            .toList(),
        createdAt: DateTime.parse(json['created_at'] as String),
        settledAt: json['settled_at'] != null
            ? DateTime.parse(json['settled_at'] as String)
            : null,
      );
}
