/// 代金券：直接抵现的优惠手段，本身**就是钱**。JSON 契约与服务端 voucher.Voucher 一致。
class Voucher {
  final int id;
  final String accountId; // json: account_id
  final int amount; // 面值（分），等价现金
  final String scope; // all / cloud / course / data / product
  final String? productId; // json: product_id（scope=product 时）
  final DateTime expiresAt; // json: expires_at
  final String status; // issued / used / expired
  final DateTime? usedAt; // json: used_at
  final String? orderId; // json: order_id
  final DateTime createdAt; // json: created_at

  const Voucher({
    required this.id,
    required this.accountId,
    required this.amount,
    required this.scope,
    this.productId,
    required this.expiresAt,
    required this.status,
    this.usedAt,
    this.orderId,
    required this.createdAt,
  });

  factory Voucher.fromJson(Map<String, dynamic> json) => Voucher(
        id: json['id'] as int,
        accountId: json['account_id'] as String,
        amount: json['amount'] as int,
        scope: json['scope'] as String,
        productId: json['product_id'] as String?,
        expiresAt: DateTime.parse(json['expires_at'] as String),
        status: json['status'] as String,
        usedAt: json['used_at'] != null
            ? DateTime.parse(json['used_at'] as String)
            : null,
        orderId: json['order_id'] as String?,
        createdAt: DateTime.parse(json['created_at'] as String),
      );
}
