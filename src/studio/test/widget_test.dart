import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qtcloud_pay_studio/main.dart';
import 'package:qtcloud_pay_studio/services/pay_api.dart';

/// 假客户端：按路径返回固定 JSON。
MockClient _mockClient() {
  return MockClient((request) async {
    switch (request.url.path) {
      case '/reconcile/consistency':
        return http.Response(jsonEncode({'discrepancies': []}), 200,
            headers: {'content-type': 'application/json'});
      case '/accounts':
        return http.Response(
            jsonEncode({
              'id': 'acc_001',
              'customer_id': 'cus_001',
              'balance': 0,
              'created_at': '2026-08-03T10:00:00+08:00',
              'updated_at': '2026-08-03T10:00:00+08:00',
            }),
            201,
            headers: {'content-type': 'application/json'});
      case '/accounts/acc_001':
        return http.Response(
            jsonEncode({
              'id': 'acc_001',
              'customer_id': 'cus_001',
              'balance': 10000,
              'created_at': '2026-08-03T10:00:00+08:00',
              'updated_at': '2026-08-03T10:00:00+08:00',
            }),
            200,
            headers: {'content-type': 'application/json'});
      case '/accounts/acc_001/transactions':
        return http.Response(
            jsonEncode({
              'account_id': 'acc_001',
              'transactions': [
                {
                  'id': 1,
                  'account_id': 'acc_001',
                  'type': 'recharge',
                  'amount': 10000,
                  'balance_after': 10000,
                  'note': '打款',
                  'created_at': '2026-08-03T10:00:00+08:00',
                }
              ],
            }),
            200,
            headers: {'content-type': 'application/json'});
      case '/accounts/acc_001/coupons':
        return http.Response(
            jsonEncode({'account_id': 'acc_001', 'coupons': []}), 200,
            headers: {'content-type': 'application/json'});
      case '/accounts/acc_001/vouchers':
        return http.Response(
            jsonEncode({'account_id': 'acc_001', 'vouchers': []}), 200,
            headers: {'content-type': 'application/json'});
      case '/orders':
        return http.Response(
            jsonEncode({
              'id': 'ORD001',
              'customer_id': 'cus_001',
              'account_id': 'acc_001',
              'amount': 5000,
              'status': 'settled',
              'settle_detail': [
                {'kind': 'balance', 'amount': 5000}
              ],
              'created_at': '2026-08-03T10:00:00+08:00',
              'settled_at': '2026-08-03T10:00:00+08:00',
            }),
            201,
            headers: {'content-type': 'application/json'});
      default:
        return http.Response('{"error": "not found"}', 404,
            headers: {'content-type': 'application/json'});
    }
  });
}

void main() {
  testWidgets('工作台冒烟：总览加载且对账无差异', (tester) async {
    await tester.pumpWidget(
      PayStudioApp(api: PayApi(client: _mockClient())),
    );
    await tester.pumpAndSettle();

    expect(find.text('账本核心工作台 v0.1.0'), findsOneWidget);
    expect(find.text('对账：余额与交易一致，无差异'), findsOneWidget);
    expect(find.text('里程碑进度'), findsOneWidget);
  });

  testWidgets('导航：切换到账户页可创建账户', (tester) async {
    await tester.pumpWidget(
      PayStudioApp(api: PayApi(client: _mockClient())),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('账户'));
    await tester.pumpAndSettle();

    expect(find.text('创建账户'), findsOneWidget);
    await tester.enterText(find.byType(TextFormField).first, 'cus_001');
    await tester.tap(find.text('创建账户'));
    await tester.pumpAndSettle();

    expect(find.textContaining('已创建账户 acc_001'), findsOneWidget);
  });
}
