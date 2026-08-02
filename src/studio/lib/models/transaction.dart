/// 交易类型（与服务端 transaction 常量一致）。
class TransactionType {
  static const recharge = 'recharge'; // 充值（对公打款入账）
  static const consume = 'consume'; // 消费（余额支付部分）
  static const issue = 'issue'; // 发券（信息性记录，不影响余额）
  static const redeem = 'redeem'; // 核销（券抵扣部分，不影响余额）

  static String label(String type) => switch (type) {
        recharge => '充值',
        consume => '消费',
        issue => '发券',
        redeem => '核销',
        _ => type,
      };
}

/// 一笔客户交易的不可变记录，是账本。JSON 契约与服务端 transaction.Transaction 一致。
class Transaction {
  final int id;
  final String accountId; // json: account_id
  final String type; // recharge / consume / issue / redeem
  final int amount; // 金额（分）
  final int balanceAfter; // json: balance_after 交易后余额快照（仅充值/消费有效）
  final String? orderId; // json: order_id 消费/核销时关联订单
  final String? note;
  final DateTime createdAt; // json: created_at

  const Transaction({
    required this.id,
    required this.accountId,
    required this.type,
    required this.amount,
    required this.balanceAfter,
    this.orderId,
    this.note,
    required this.createdAt,
  });

  factory Transaction.fromJson(Map<String, dynamic> json) => Transaction(
        id: json['id'] as int,
        accountId: json['account_id'] as String,
        type: json['type'] as String,
        amount: json['amount'] as int,
        balanceAfter: json['balance_after'] as int,
        orderId: json['order_id'] as String?,
        note: json['note'] as String?,
        createdAt: DateTime.parse(json['created_at'] as String),
      );

  /// 是否影响余额（发券/核销不参与余额求和）。
  bool get affectsBalance => type == TransactionType.recharge || type == TransactionType.consume;

  /// 带符号金额：充值 +，消费 −，其余 0（与服务端 SignedAmount 一致）。
  int get signedAmount => switch (type) {
        TransactionType.recharge => amount,
        TransactionType.consume => -amount,
        _ => 0,
      };
}
