import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:qtcloud_pay_studio/main.dart';
import 'package:qtcloud_pay_studio/services/local_draft_store.dart';
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

/// 假客户端：按路径返回固定 JSON。带发放规则集，供「发放规则」页签测试。
MockClient _mockClientWithRules() {
  return MockClient((request) async {
    switch (request.url.path) {
      case '/reconcile/consistency':
        return http.Response(jsonEncode({'discrepancies': []}), 200,
            headers: {'content-type': 'application/json'});
      case '/admin/voucher-pricing-rules':
        return http.Response(
            jsonEncode({
              'rule_sets': [
                {
                  'id': 'qtclass-voucher-pricing',
                  'source': 'qtclass-voucher-pricing.json',
                  'version': '2026-09-02',
                  'payload': {
                    'issuance': {
                      'channels': [
                        {
                          'name': '课堂实训任务',
                          'trigger': '实训任务验收通过',
                          'voucher': {
                            'amount_cents': 10000,
                            'amount_display': '100 元',
                            'scope': 'all',
                          },
                          'count_per_event': 1,
                        },
                        {
                          'name': '追加奖励·产出评定',
                          'trigger': '按产出评定',
                          'voucher': {
                            'amount_rule': '仅限 100 元或 500 元面额',
                            'scope': 'all',
                          },
                          'bonus_type': 'output_assessment',
                        },
                      ],
                      'bonus_denomination_rule': '仅使用 100 元、500 元两种面额',
                    },
                  },
                  'created_at': '2026-09-02T00:00:00Z',
                  'updated_at': '2026-09-02T00:00:00Z',
                },
              ],
            }),
            200,
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

  testWidgets('发放规则页签：只读展示发放渠道与面额规则', (tester) async {
    await tester.pumpWidget(
      PayStudioApp(api: PayApi(client: _mockClientWithRules())),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.text('发券').first);
    await tester.pumpAndSettle();
    await tester.tap(find.text('发放规则'));
    await tester.pumpAndSettle();

    expect(find.text('qtclass-voucher-pricing'), findsOneWidget);
    expect(find.text('课堂实训任务'), findsOneWidget);
    expect(find.textContaining('触发：实训任务验收通过'), findsOneWidget);
    expect(find.textContaining('面额：100 元'), findsOneWidget);
    expect(find.textContaining('追加奖励（output_assessment）'), findsOneWidget);
    expect(
      find.textContaining('面额规则：仅使用 100 元、500 元两种面额'),
      findsOneWidget,
    );
    // 监管视图无任何发放操作按钮。
    expect(find.text('发放'), findsNothing);
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

  ruleDesignTests();
}

void ruleDesignTests() {
  testWidgets('规则设计页：新建草稿并完成一个合法渠道', (tester) async {
    final dir = Directory.systemTemp.createTempSync('rule-design-test');
    final store = LocalDraftStore(filePath: '${dir.path}/drafts.json');
    addTearDown(() => dir.deleteSync(recursive: true));

    await tester.pumpWidget(PayStudioApp(
      api: PayApi(client: _mockClient()),
      ruleDesignStore: store,
    ));
    await tester.pumpAndSettle();

    await tester.tap(find.text('规则设计').first);
    await tester.pumpAndSettle();
    expect(find.text('还没有草稿'), findsOneWidget);

    // 新建草稿。
    await tester.tap(find.text('新建草稿'));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.widgetWithText(TextField, '规则集 ID（如 qtclass-voucher-pricing）'),
        'qtclass-voucher-pricing');
    await tester.tap(find.text('创建'));
    await tester.pumpAndSettle();
    expect(find.text('qtclass-voucher-pricing'), findsWidgets);

    // 预置空渠道触发校验问题。
    expect(find.textContaining('缺名称'), findsOneWidget);

    // 编辑渠道填成合法值。
    await tester.tap(find.text('（未命名渠道）'));
    await tester.pumpAndSettle();
    await tester.enterText(
        find.widgetWithText(TextField, '渠道名称（如 课堂实训任务）'), '课堂实训任务');
    await tester.enterText(
        find.widgetWithText(TextField, '触发条件（如 实训任务验收通过）'), '实训任务验收通过');
    await tester.enterText(find.widgetWithText(TextField, '面值（元，如 100）'), '100');
    await tester.tap(find.text('保存'));
    await tester.pumpAndSettle();

    expect(find.textContaining('发放段校验通过'), findsOneWidget);
    expect(find.textContaining('课堂实训任务'), findsOneWidget);

    // 本地文件已落盘。
    final persisted = store.load();
    expect(persisted.single.id, 'qtclass-voucher-pricing');
    expect(persisted.single.channels.single.amountCents, 10000);
  });
}
