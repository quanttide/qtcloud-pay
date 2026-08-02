/// 账户（客户虚拟钱包）。JSON 契约与服务端 account.Account 一致。
class Account {
  final String id; // 业务号，如 acc_xxx
  final String customerId; // json: customer_id
  final int balance; // 余额（分），交易的投影
  final DateTime createdAt; // json: created_at
  final DateTime updatedAt; // json: updated_at

  const Account({
    required this.id,
    required this.customerId,
    required this.balance,
    required this.createdAt,
    required this.updatedAt,
  });

  factory Account.fromJson(Map<String, dynamic> json) => Account(
        id: json['id'] as String,
        customerId: json['customer_id'] as String,
        balance: json['balance'] as int,
        createdAt: DateTime.parse(json['created_at'] as String),
        updatedAt: DateTime.parse(json['updated_at'] as String),
      );
}
