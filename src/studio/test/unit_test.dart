import 'package:flutter_test/flutter_test.dart';
import 'package:qtcloud_pay_studio/models/pricing_rule_set.dart';
import 'package:qtcloud_pay_studio/models/transaction.dart';
import 'package:qtcloud_pay_studio/services/pay_api.dart';
import 'package:qtcloud_pay_studio/widgets/amount_field.dart';
import 'package:qtcloud_pay_studio/widgets/money_text.dart';

void main() {
  group('AmountField.yuanToCents（元 → 分）', () {
    test('整数元', () => expect(AmountField.yuanToCents('100'), 10000));
    test('带两位小数', () => expect(AmountField.yuanToCents('100.50'), 10050));
    test('一位小数补零', () => expect(AmountField.yuanToCents('99.9'), 9990));
    test('带符号', () => expect(AmountField.yuanToCents('-5.25'), -525));
    test('0 与小数', () => expect(AmountField.yuanToCents('0.01'), 1));
  });

  group('MoneyText.format（分 → 元）', () {
    test('两位小数', () => expect(MoneyText.format(10000), '100.00'));
    test('小于 1 元', () => expect(MoneyText.format(5), '0.05'));
  });

  group('Transaction.signedAmount（方向约定）', () {
    test('充值 +', () {
      final t = Transaction.fromJson({
        'id': 1,
        'account_id': 'a',
        'type': 'recharge',
        'amount': 100,
        'balance_after': 100,
        'created_at': '2026-08-03T10:00:00+08:00',
      });
      expect(t.signedAmount, 100);
      expect(t.affectsBalance, isTrue);
    });

    test('消费 −', () {
      final t = Transaction.fromJson({
        'id': 2,
        'account_id': 'a',
        'type': 'consume',
        'amount': 40,
        'balance_after': 60,
        'created_at': '2026-08-03T10:00:00+08:00',
      });
      expect(t.signedAmount, -40);
      expect(t.affectsBalance, isTrue);
    });

    test('发券/核销不参与余额求和', () {
      for (final type in ['issue', 'redeem']) {
        final t = Transaction.fromJson({
          'id': 3,
          'account_id': 'a',
          'type': type,
          'amount': 10,
          'balance_after': 60,
          'created_at': '2026-08-03T10:00:00+08:00',
        });
        expect(t.signedAmount, 0);
        expect(t.affectsBalance, isFalse);
      }
    });
  });

  group('ApiException 错误映射', () {
    test('携带状态码与消息', () {
      const e = ApiException(422, 'billing: insufficient balance');
      expect(e.statusCode, 422);
      expect(e.message, contains('insufficient'));
    });
  });

  pricingRuleSetTests();
}

void pricingRuleSetTests() {
  group('PricingRuleSet（发放规则解析）', () {
    final payload = {
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
    };

    test('固定面额渠道优先人读字段', () {
      final rs = PricingRuleSet.fromJson({
        'id': 'qtclass-voucher-pricing',
        'payload': payload,
        'updated_at': '2026-09-02T00:00:00Z',
      });
      final fixed = IssuanceChannel.fromJson(rs.issuanceChannels.first);
      expect(fixed.name, '课堂实训任务');
      expect(fixed.amountLabel, '100 元');
      expect(fixed.countPerEvent, 1);
    });

    test('动态面额渠道回退到规则描述', () {
      final rs = PricingRuleSet.fromJson({
        'id': 'qtclass-voucher-pricing',
        'payload': payload,
        'updated_at': '2026-09-02T00:00:00Z',
      });
      final bonus = IssuanceChannel.fromJson(rs.issuanceChannels[1]);
      expect(bonus.bonusType, 'output_assessment');
      expect(bonus.amountLabel, '仅限 100 元或 500 元面额');
      expect(rs.bonusDenominationRule, '仅使用 100 元、500 元两种面额');
    });

    test('无 amount_display 时分换算元', () {
      final ch = IssuanceChannel.fromJson({
        'name': 'n',
        'trigger': 't',
        'voucher': {'amount_cents': 100000, 'scope': 'all'},
      });
      expect(ch.amountLabel, '1000.00 元');
    });

    test('payload 缺 issuance 不抛异常', () {
      final rs = PricingRuleSet.fromJson({
        'id': 'x',
        'payload': <String, dynamic>{},
        'updated_at': '2026-09-02T00:00:00Z',
      });
      expect(rs.issuanceChannels, isEmpty);
      expect(rs.bonusDenominationRule, isNull);
    });
  });
}
