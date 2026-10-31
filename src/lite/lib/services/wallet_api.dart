// 钱包 API：余额 + 交易流水（复用 qtcloud-pay，customer_id = 用户 ID）。

import 'dart:convert';

import 'package:http/http.dart' as http;

class WalletApiException implements Exception {
  const WalletApiException(this.message);
  final String message;
  @override
  String toString() => message;
}

/// 客户没有钱包账户（qtcloud-pay 返回 404，属业务空态而非故障）。
class WalletNotFoundException implements Exception {
  const WalletNotFoundException();
}

class Wallet {
  const Wallet({required this.balance, required this.accountId});
  final int balance;
  final String accountId;

  String get balanceYuan => (balance / 100).toStringAsFixed(2);
}

class TransactionItem {
  const TransactionItem({
    required this.id,
    required this.type,
    required this.amount,
    required this.note,
    required this.createdAt,
  });

  final int id;
  final String type;
  final int amount;
  final String note;
  final String createdAt;

  factory TransactionItem.fromJson(Map<String, dynamic> json) => TransactionItem(
    id: json['id'] as int,
    type: json['type'] as String,
    amount: (json['amount'] as Map<String, dynamic>)['amount'] as int? ?? 0,
    note: json['note'] as String? ?? '',
    createdAt: json['created_at'] as String? ?? '',
  );
}

class WalletApi {
  WalletApi({http.Client? client, required this.baseUrl, required this.token, required this.customerId})
    : _client = client ?? http.Client();

  /// 默认支付服务地址（qtcloud-pay，经网关统一入口）。
  static String defaultBaseUrl() {
    const String fromEnv = String.fromEnvironment('QTCLOUD_PAY_BASE_URL');
    if (fromEnv.isNotEmpty) {
      return fromEnv;
    }
    return 'http://localhost:8080';
  }

  final http.Client _client;
  final String baseUrl;
  final String token;
  final String customerId;

  Map<String, String> get _headers => {
    'Authorization': 'Bearer $token',
    'Content-Type': 'application/json',
  };

  Future<Wallet> fetchWallet() async {
    final resp = await _client
        .get(Uri.parse('$baseUrl/customers/$customerId/account'), headers: _headers)
        .timeout(const Duration(seconds: 15));
    if (resp.statusCode == 404) {
      throw const WalletNotFoundException();
    }
    if (resp.statusCode != 200) {
      throw WalletApiException('获取钱包失败（HTTP ${resp.statusCode}）');
    }
    final body = jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
    return Wallet(
      balance: (body['balance'] as Map<String, dynamic>)['amount'] as int? ?? 0,
      accountId: body['id'] as String,
    );
  }

  Future<List<TransactionItem>> fetchTransactions([String? accountId]) async {
    final id = accountId ?? (await fetchWallet()).accountId;
    final resp = await _client
        .get(Uri.parse('$baseUrl/accounts/$id/transactions'), headers: _headers)
        .timeout(const Duration(seconds: 15));
    if (resp.statusCode != 200) {
      throw WalletApiException('获取流水失败（HTTP ${resp.statusCode}）');
    }
    final body = jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;
    return (body['transactions'] as List<dynamic>? ?? [])
        .map((e) => TransactionItem.fromJson(e as Map<String, dynamic>))
        .toList();
  }
}
