/// 适用范围（与服务端 coupon/voucher Scope 常量一致）。
class Scope {
  static const all = 'all'; // 全场通用
  static const cloud = 'cloud'; // 云服务
  static const course = 'course'; // 课程
  static const data = 'data'; // 数据服务
  static const product = 'product'; // 指定商品

  static String label(String scope) => switch (scope) {
        all => '全场',
        cloud => '云服务',
        course => '课程',
        data => '数据服务',
        product => '指定商品',
        _ => scope,
      };
}

/// 券状态（与服务端 coupon/voucher Status 常量一致）。
class VoucherStatus {
  static const issued = 'issued'; // 已发放
  static const used = 'used'; // 已使用
  static const expired = 'expired'; // 已过期

  static String label(String status) => switch (status) {
        issued => '已发放',
        used => '已使用',
        expired => '已过期',
        _ => status,
      };
}

/// 优惠券类型（与服务端 coupon Type 常量一致）。
class CouponType {
  static const discount = 'discount'; // 折扣券：按比例优惠
  static const fullReduction = 'full_reduction'; // 满减券：满足门槛后减额

  static String label(String type) => switch (type) {
        discount => '折扣券',
        fullReduction => '满减券',
        _ => type,
      };
}

/// 优惠券：按规则抵扣的优惠手段，本身不代表钱。JSON 契约与服务端 coupon.Coupon 一致。
class Coupon {
  final int id;
  final String accountId; // json: account_id
  final String type; // discount / full_reduction
  final int? rate; // 折扣券：整数百分比（90 = 9 折）
  final int? threshold; // 满减券：门槛（分）
  final int? amount; // 满减券：减额（分）
  final String scope; // all / cloud / course / data / product
  final String? productId; // json: product_id（scope=product 时）
  final DateTime expiresAt; // json: expires_at
  final String status; // issued / used / expired
  final DateTime? usedAt; // json: used_at
  final String? orderId; // json: order_id
  final DateTime createdAt; // json: created_at

  const Coupon({
    required this.id,
    required this.accountId,
    required this.type,
    this.rate,
    this.threshold,
    this.amount,
    required this.scope,
    this.productId,
    required this.expiresAt,
    required this.status,
    this.usedAt,
    this.orderId,
    required this.createdAt,
  });

  factory Coupon.fromJson(Map<String, dynamic> json) => Coupon(
        id: json['id'] as int,
        accountId: json['account_id'] as String,
        type: json['type'] as String,
        rate: json['rate'] as int?,
        threshold: json['threshold'] as int?,
        amount: json['amount'] as int?,
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

  /// 参数描述，如「9 折」「满 100 减 20」。
  String get paramLabel => switch (type) {
        CouponType.discount => '${rate ?? 0} 折',
        CouponType.fullReduction =>
          '满 ${_yuan(threshold ?? 0)} 减 ${_yuan(amount ?? 0)}',
        _ => '',
      };

  static String _yuan(int cents) =>
      (cents / 100).toStringAsFixed(cents % 100 == 0 ? 0 : 2);
}
