import 'package:flutter/foundation.dart';

import '../models/account.dart';
import '../models/order.dart';
import '../models/transaction.dart';
import 'pay_api.dart';

/// 客户端共享状态（provider）：缓存本会话已创建/使用过的账户与订单，
/// 弥补服务端 v0.1.0 无列表端点的缺口（GET /accounts、GET /orders 未实现）。
///
/// 只缓存服务端返回过的数据，不做任何账务计算。
class PayStore extends ChangeNotifier {
  PayStore(this.api);

  final PayApi api;

  final Map<String, Account> _accounts = {};
  final Map<String, Order> _orders = {};
  final Map<String, List<Transaction>> _transactions = {};

  List<Account> get accounts => _accounts.values.toList();

  List<Order> get orders => _orders.values.toList();

  Account? accountOf(String accountId) => _accounts[accountId];

  Order? orderOf(String orderId) => _orders[orderId];

  List<Transaction>? transactionsOf(String accountId) =>
      _transactions[accountId];

  /// 记录服务端返回的账户。
  void cacheAccount(Account account) {
    _accounts[account.id] = account;
    notifyListeners();
  }

  /// 记录服务端返回的订单。
  void cacheOrder(Order order) {
    _orders[order.id] = order;
    notifyListeners();
  }

  /// 记录服务端返回的流水。
  void cacheTransactions(String accountId, List<Transaction> txs) {
    _transactions[accountId] = txs;
    notifyListeners();
  }
}
