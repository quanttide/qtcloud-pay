import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:qtcloud_pay_studio/models/pricing_rule_set.dart';
import 'package:qtcloud_pay_studio/models/rule_draft.dart';
import 'package:qtcloud_pay_studio/models/transaction.dart';
import 'package:qtcloud_pay_studio/services/local_draft_store.dart';
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
  ruleDraftTests();
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

void ruleDraftTests() {
  group('RuleDraft（发放规则草稿）', () {
    RuleDraftChannel fixedChannel() => RuleDraftChannel(
          name: '课堂实训任务',
          trigger: '实训任务验收通过',
          amountCents: 10000,
          scope: 'all',
          countPerEvent: 1,
          entry: '任命公告',
        );

    test('普通渠道校验通过', () {
      final draft = RuleDraft(id: 'qtclass')..channels.add(fixedChannel());
      expect(draft.validate(), isEmpty);
    });

    test('缺名称与触发条件报错', () {
      final draft = RuleDraft(id: 'qtclass')
        ..channels.add(RuleDraftChannel(amountCents: 100));
      final problems = draft.validate();
      expect(problems, contains(contains('缺名称')));
      expect(problems, contains(contains('缺触发条件')));
    });

    test('面额为 0 报错', () {
      final draft = RuleDraft(id: 'qtclass')..channels.add(fixedChannel()..amountCents = 0);
      expect(draft.validate(), contains(contains('固定面额需为正数')));
    });

    test('追加奖励渠道必须填面额规则且不带固定面额', () {
      final draft = RuleDraft(id: 'qtclass')
        ..channels.add(RuleDraftChannel(
          name: '追加奖励·首次完成',
          trigger: '首个完成',
          bonusType: 'first_completion',
          countPerEvent: '多张券组合',
        ));
      final problems = draft.validate();
      expect(problems, contains(contains('需填写面额规则')));
      expect(problems, contains(contains('必须填写面额规则')));

      draft.channels[0].amountRule = '等额追加';
      draft.bonusDenominationRule = '仅 100/500 元面额';
      expect(draft.validate(), isEmpty);
    });

    test('追加奖励渠道写固定面额报错', () {
      final ch = RuleDraftChannel(
        name: 'n',
        trigger: 't',
        amountCents: 100,
        bonusType: 'group_interaction',
        amountRule: 'r',
      );
      expect(ch.validate('渠道'), contains(contains('不得同时写固定面额')));
    });

    test('导出 payload 对齐服务端形状', () {
      final draft = RuleDraft(id: 'qtclass', source: 'journal', version: '2026-09-02')
        ..channels.add(fixedChannel())
        ..channels.add(RuleDraftChannel(
          name: '追加奖励·产出评定',
          trigger: '按产出评定',
          bonusType: 'output_assessment',
          amountRule: '仅限 100 元或 500 元面额',
          countPerEvent: '多张券组合',
        ))
        ..bonusDenominationRule = '仅使用 100 元、500 元两种面额';
      final payload = draft.toPayload();
      final channels = (payload['issuance'] as Map)['channels'] as List;
      expect((channels[0] as Map)['voucher']['amount_cents'], 10000);
      expect((channels[1] as Map)['bonus_type'], 'output_assessment');
      expect((channels[1] as Map)['voucher']['amount_rule'], '仅限 100 元或 500 元面额');
      expect((channels[1] as Map)['voucher']['amount_cents'], isNull);
      expect(
        (payload['billing_semantics'] as Map)['voucher_is_money'],
        isTrue,
      );
      final body = draft.toPutBody();
      expect(body['source'], 'journal');
    });

    test('JSON 往返保持内容', () {
      final draft = RuleDraft(id: 'qtclass', source: 's', version: 'v')
        ..channels.add(fixedChannel())
        ..bonusDenominationRule = 'r';
      final back = RuleDraft.fromJson(draft.toJson());
      expect(back.id, 'qtclass');
      expect(back.channels.single.amountCents, 10000);
      expect(back.channels.single.entry, '任命公告');
      expect(back.bonusDenominationRule, 'r');
    });
  });

  group('LocalDraftStore（本地草稿仓）', () {
    test('保存后可读回', () {
      final dir = Directory.systemTemp.createTempSync('draft-store-test');
      final store = LocalDraftStore(filePath: '${dir.path}/drafts.json');
      final draft = RuleDraft(id: 'qtclass')
        ..channels.add(RuleDraftChannel(
            name: 'n', trigger: 't', amountCents: 100));
      expect(store.save([draft]), isTrue);
      final loaded = store.load();
      expect(loaded.single.id, 'qtclass');
      expect(loaded.single.channels.single.amountCents, 100);
      dir.deleteSync(recursive: true);
    });

    test('文件不存在返回空列表', () {
      final dir = Directory.systemTemp.createTempSync('draft-store-test');
      final store = LocalDraftStore(filePath: '${dir.path}/missing.json');
      expect(store.load(), isEmpty);
      dir.deleteSync(recursive: true);
    });

    test('文件损坏返回空列表而不抛', () {
      final dir = Directory.systemTemp.createTempSync('draft-store-test');
      final file = File('${dir.path}/broken.json')..writeAsStringSync('{oops');
      final store = LocalDraftStore(filePath: file.path);
      expect(store.load(), isEmpty);
      dir.deleteSync(recursive: true);
    });
  });
}
