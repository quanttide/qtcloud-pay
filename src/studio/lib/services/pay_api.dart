import 'dart:convert';

import 'package:http/http.dart' as http;

import '../models/account.dart';
import '../models/coupon.dart';
import '../models/order.dart';
import '../models/reconciliation.dart';
import '../models/transaction.dart';
import '../models/voucher.dart';

/// API 调用异常。message 取服务端 `{"error": "..."}` 或 HTTP 状态文本。
class ApiException implements Exception {
  final int statusCode;
  final String message;

  const ApiException(this.statusCode, this.message);

  @override
  String toString() => 'ApiException($statusCode): $message';
}

/// 发券结果（POST /accounts/{id}/coupons 与 /vouchers 的响应）。
class IssueResult {
  final String accountId; // json: account_id
  final String batchNo; // json: batch_no
  final int count;

  const IssueResult({
    required this.accountId,
    required this.batchNo,
    required this.count,
  });

  factory IssueResult.fromJson(Map<String, dynamic> json) => IssueResult(
        accountId: json['account_id'] as String,
        batchNo: json['batch_no'] as String,
        count: json['count'] as int,
      );
}

/// 充值登记结果（POST /accounts/{id}/recharges 的响应）。
class RechargeResult {
  final String accountId; // json: account_id

  const RechargeResult({required this.accountId});

  factory RechargeResult.fromJson(Map<String, dynamic> json) => RechargeResult(
        accountId: json['account_id'] as String,
      );
}

/// 账本 API 客户端。封装全部 v0.1.0 端点，页面只依赖本文件，不直接拼 URL。
///
/// 约定（见 doc/conventions.md）：
/// - 金额一律整数分
/// - 错误响应统一 `{"error": "..."}`，按状态码抛 [ApiException]
/// - 列表响应统一包装 `{"account_id", "transactions|coupons|vouchers"}`
class PayApi {
  PayApi({http.Client? client, this.baseUrl = 'http://localhost:8080'})
      : _client = client ?? http.Client();

  final http.Client _client;
  final String baseUrl;

  Uri _uri(String path, [Map<String, String>? query]) =>
      Uri.parse('$baseUrl$path').replace(queryParameters: query);

  /// 发送请求并统一处理响应与错误。
  Future<Map<String, dynamic>> _request(
    String method,
    String path, {
    Object? body,
    Map<String, String>? query,
  }) async {
    final uri = _uri(path, query);
    late http.Response resp;
    try {
      resp = switch (method) {
        'GET' => await _client.get(uri),
        'POST' => await _client.post(
            uri,
            headers: {'Content-Type': 'application/json'},
            body: jsonEncode(body),
          ),
        _ => throw ArgumentError('unsupported method: $method'),
      };
    } on ApiException {
      rethrow;
    } catch (e) {
      throw ApiException(0, '无法连接服务端：$e');
    }

    final text = resp.body.isEmpty ? '{}' : resp.body;
    Map<String, dynamic> data;
    try {
      data = jsonDecode(text) as Map<String, dynamic>;
    } catch (_) {
      throw ApiException(resp.statusCode, '响应不是 JSON：$text');
    }

    if (resp.statusCode >= 400) {
      throw ApiException(
        resp.statusCode,
        (data['error'] as String?) ?? 'HTTP ${resp.statusCode}',
      );
    }
    return data;
  }

  // ---------- 账户与余额（M1） ----------

  /// POST /accounts 创建账户。
  Future<Account> createAccount(String customerId) async {
    final data = await _request('POST', '/accounts', body: {
      'customer_id': customerId,
    });
    return Account.fromJson(data);
  }

  /// GET /accounts/{id} 账户与余额。
  Future<Account> getAccount(String accountId) async {
    final data = await _request('GET', '/accounts/$accountId');
    return Account.fromJson(data);
  }

  /// POST /accounts/{id}/recharges 充值登记（对公打款入账，幂等键 = voucherNo）。
  Future<RechargeResult> recharge(
    String accountId, {
    required int amount, // 分
    required String voucherNo,
    String? note,
  }) async {
    final data = await _request('POST', '/accounts/$accountId/recharges', body: {
      'amount': amount,
      'voucher_no': voucherNo,
      'note': note,
    });
    return RechargeResult.fromJson(data);
  }

  /// GET /accounts/{id}/transactions 交易流水（分页倒序）。
  Future<List<Transaction>> listTransactions(
    String accountId, {
    int limit = 20,
    int offset = 0,
  }) async {
    final data = await _request(
      'GET',
      '/accounts/$accountId/transactions',
      query: {'limit': '$limit', 'offset': '$offset'},
    );
    return (data['transactions'] as List? ?? [])
        .map((e) => Transaction.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// GET /accounts 账户列表（v0.1.0 服务端未提供，客户端缓存已创建账户）。
  ///
  /// 说明：服务端 v0.1.0 无 GET /accounts 端点；客户端在本会话内缓存
  /// 创建/使用过的账户，避免全量接口缺失影响操作台使用。
  Future<List<Account>> listAccounts() async {
    throw ApiException(501, 'v0.1.0 服务端未提供账户列表端点，请使用最近账户');
  }

  // ---------- 优惠券（M2） ----------

  /// POST /accounts/{id}/coupons 发放优惠券（批量 + 幂等，幂等键 = batchNo）。
  Future<IssueResult> issueCoupons(
    String accountId, {
    required String type, // discount / full_reduction
    int? rate,
    int? threshold,
    int? amount,
    required String scope,
    String? productId,
    required DateTime expiresAt,
    required int count,
    required String batchNo,
    String? note,
  }) async {
    final data = await _request('POST', '/accounts/$accountId/coupons', body: {
      'type': type,
      'rate': ?rate,
      'threshold': ?threshold,
      'amount': ?amount,
      'scope': scope,
      'product_id': ?productId,
      'expires_at': expiresAt.toIso8601String(),
      'count': count,
      'batch_no': batchNo,
      'note': ?note,
    });
    return IssueResult.fromJson(data);
  }

  /// GET /accounts/{id}/coupons 查询优惠券。
  Future<List<Coupon>> listCoupons(String accountId) async {
    final data = await _request('GET', '/accounts/$accountId/coupons');
    return (data['coupons'] as List? ?? [])
        .map((e) => Coupon.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  // ---------- 代金券（M2） ----------

  /// POST /accounts/{id}/vouchers 发放代金券（批量 + 幂等，幂等键 = batchNo）。
  Future<IssueResult> issueVouchers(
    String accountId, {
    required int amount, // 面值（分）
    required String scope,
    String? productId,
    required DateTime expiresAt,
    required int count,
    required String batchNo,
    String? note,
  }) async {
    final data = await _request('POST', '/accounts/$accountId/vouchers', body: {
      'amount': amount,
      'scope': scope,
      'product_id': ?productId,
      'expires_at': expiresAt.toIso8601String(),
      'count': count,
      'batch_no': batchNo,
      'note': ?note,
    });
    return IssueResult.fromJson(data);
  }

  /// GET /accounts/{id}/vouchers 查询代金券。
  Future<List<Voucher>> listVouchers(String accountId) async {
    final data = await _request('GET', '/accounts/$accountId/vouchers');
    return (data['vouchers'] as List? ?? [])
        .map((e) => Voucher.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  // ---------- 订单与结算（M3） ----------

  /// POST /orders 下单并结算（幂等键 = orderId）。
  Future<Order> settleOrder({
    required String orderId,
    required String customerId,
    required String accountId,
    String? productId,
    String? scope,
    required int amount, // 分
  }) async {
    final data = await _request('POST', '/orders', body: {
      'order_id': orderId,
      'customer_id': customerId,
      'account_id': accountId,
      'product_id': ?productId,
      'scope': ?scope,
      'amount': amount,
    });
    return Order.fromJson(data);
  }

  /// GET /orders/{id} 订单与结算明细。
  Future<Order> getOrder(String orderId) async {
    final data = await _request('GET', '/orders/$orderId');
    return Order.fromJson(data);
  }

  /// GET /orders 订单列表（v0.1.0 服务端未提供，客户端缓存本会话订单）。
  Future<List<Order>> listOrders() async {
    throw ApiException(501, 'v0.1.0 服务端未提供订单列表端点');
  }

  // ---------- 对账与可查（M4） ----------

  /// GET /reconcile/consistency 一致性校验。
  Future<List<Discrepancy>> checkConsistency() async {
    final data = await _request('GET', '/reconcile/consistency');
    return (data['discrepancies'] as List? ?? [])
        .map((e) => Discrepancy.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// POST /reconcile/bank 对公打款核对（上传银行流水 CSV）。
  Future<BankReport> reconcileBankFile(String csv) async {
    final uri = _uri('/reconcile/bank');
    late http.Response resp;
    try {
      resp = await _client.post(
        uri,
        headers: {'Content-Type': 'text/csv'},
        body: csv,
      );
    } catch (e) {
      throw ApiException(0, '无法连接服务端：$e');
    }
    final data = jsonDecode(resp.body) as Map<String, dynamic>;
    if (resp.statusCode >= 400) {
      throw ApiException(
        resp.statusCode,
        (data['error'] as String?) ?? 'HTTP ${resp.statusCode}',
      );
    }
    return BankReport.fromJson(data);
  }

  /// GET /accounts/{id}/statement 账单导出。
  Future<Statement> getStatement(String accountId) async {
    final data = await _request('GET', '/accounts/$accountId/statement');
    return Statement.fromJson(data);
  }
}
