import 'dart:convert';

/// 代金券发放规则草稿（离线设计用，不联网）。
///
/// 形状对齐服务端计价规则集 payload 的 issuance 段
/// （provider `internal/voucher/service.go` 的 `pricingPayload`）；
/// 核销场景（redemption）联调时在服务端补齐，见 [uploadNote]。
class RuleDraft {
  RuleDraft({
    required this.id,
    this.source = '',
    this.version = '',
    List<RuleDraftChannel>? channels,
    this.bonusDenominationRule = '',
    DateTime? updatedAt,
  })  : channels = channels ?? <RuleDraftChannel>[],
        updatedAt = updatedAt ?? DateTime.now();

  String id;
  String source;
  String version;
  List<RuleDraftChannel> channels;
  String bonusDenominationRule;
  DateTime updatedAt;

  static const uploadNote =
      '服务端 PUT 还要求核销场景（redemption.scenarios），联调时需补齐；本页只设计发放规则段。';

  bool get hasBonusChannel => channels.any((c) => c.isBonus);

  /// 导出为服务端 PUT /admin/voucher-pricing-rules/{id} 的请求体。
  Map<String, dynamic> toPutBody() => {
        if (source.trim().isNotEmpty) 'source': source.trim(),
        if (version.trim().isNotEmpty) 'version': version.trim(),
        'payload': toPayload(),
      };

  /// 服务端 payload 形状（issuance 段 + billing_semantics）。
  Map<String, dynamic> toPayload() => {
        'meta': {
          if (source.trim().isNotEmpty) 'source': source.trim(),
          'updated_at': updatedAt.toIso8601String(),
        },
        'issuance': {
          'channels': [for (final c in channels) c.toJson()],
          if (bonusDenominationRule.trim().isNotEmpty)
            'bonus_denomination_rule': bonusDenominationRule.trim(),
        },
        'billing_semantics': {'voucher_is_money': true},
      };

  /// 按服务端 validatePricingRuleSet 的发放段规则预检，返回问题清单（空 = 通过）。
  List<String> validate() {
    final problems = <String>[];
    if (id.trim().isEmpty) {
      problems.add('草稿 ID 不能为空');
    }
    if (channels.isEmpty) {
      problems.add('至少需要一个发放渠道');
    }
    for (var i = 0; i < channels.length; i++) {
      final label = '渠道 ${i + 1}（${channels[i].name.isEmpty ? '未命名' : channels[i].name}）';
      problems.addAll(channels[i].validate(label));
    }
    if (hasBonusChannel && bonusDenominationRule.trim().isEmpty) {
      problems.add('含追加奖励渠道时必须填写面额规则（bonus_denomination_rule）');
    }
    return problems;
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'source': source,
        'version': version,
        'channels': [for (final c in channels) c.toJson()],
        'bonus_denomination_rule': bonusDenominationRule,
        'updated_at': updatedAt.toIso8601String(),
      };

  factory RuleDraft.fromJson(Map<String, dynamic> json) => RuleDraft(
        id: json['id'] as String? ?? '',
        source: json['source'] as String? ?? '',
        version: json['version'] as String? ?? '',
        channels: (json['channels'] as List? ?? [])
            .map((e) =>
                RuleDraftChannel.fromJson((e as Map).cast<String, dynamic>()))
            .toList(),
        bonusDenominationRule:
            json['bonus_denomination_rule'] as String? ?? '',
        updatedAt: DateTime.tryParse(json['updated_at'] as String? ?? '') ??
            DateTime.now(),
      );
}

/// 单条发放渠道（草稿态；字段与服务端 channels[] 对齐）。
class RuleDraftChannel {
  RuleDraftChannel({
    this.name = '',
    this.trigger = '',
    this.amountCents = 0,
    this.amountRule = '',
    this.scope = 'all',
    this.bonusType = '',
    this.countPerEvent = 1,
    this.entry = '',
    this.expiresAtRule = '发放时确定',
  });

  String name;
  String trigger;

  /// 固定面额（分）；追加奖励渠道必须为 0。
  int amountCents;

  /// 动态面额规则描述；追加奖励渠道必填。
  String amountRule;
  String scope;

  /// 追加奖励类型：''（普通渠道）/ first_completion / output_assessment / group_interaction。
  String bonusType;

  /// 每次事件发放张数：正整数或描述文字。
  Object countPerEvent;
  String entry;
  String expiresAtRule;

  static const bonusTypes = {
    'first_completion': '首次完成',
    'output_assessment': '产出评定',
    'group_interaction': '群内互动',
  };
  static const scopes = ['all', 'cloud', 'course', 'data', 'product'];

  bool get isBonus => bonusType.isNotEmpty;

  List<String> validate(String label) {
    final problems = <String>[];
    if (name.trim().isEmpty) problems.add('$label：缺名称');
    if (trigger.trim().isEmpty) problems.add('$label：缺触发条件');
    if (expiresAtRule.trim().isEmpty) problems.add('$label：缺有效期规则');
    if (!_validCountPerEvent(countPerEvent)) {
      problems.add('$label：每次事件张数需为正整数或非空描述');
    }
    if (!scopes.contains(scope)) {
      problems.add('$label：范围「$scope」不在允许值内');
    }
    if (isBonus) {
      if (!bonusTypes.containsKey(bonusType)) {
        problems.add('$label：追加奖励类型「$bonusType」不在允许值内');
      }
      if (amountRule.trim().isEmpty) {
        problems.add('$label：追加奖励需填写面额规则（如「仅限 100 元或 500 元面额」）');
      }
      if (amountCents != 0) {
        problems.add('$label：追加奖励不得同时写固定面额');
      }
    } else {
      if (amountCents <= 0) problems.add('$label：固定面额需为正数');
      if (amountRule.trim().isNotEmpty) {
        problems.add('$label：普通渠道不得写面额规则（填了就该设为追加奖励）');
      }
    }
    return problems;
  }

  static bool _validCountPerEvent(Object v) {
    if (v is int) return v > 0;
    if (v is String) return v.trim().isNotEmpty;
    return false;
  }

  Map<String, dynamic> toJson() => {
        'name': name.trim(),
        'trigger': trigger.trim(),
        'voucher': {
          if (!isBonus) 'amount_cents': amountCents,
          if (isBonus) 'amount_rule': amountRule.trim(),
          'scope': scope,
          'expires_at_rule': expiresAtRule.trim(),
        },
        if (isBonus) 'bonus_type': bonusType,
        'count_per_event': countPerEvent is String
            ? (countPerEvent as String).trim()
            : countPerEvent,
        if (entry.trim().isNotEmpty) 'entry': entry.trim(),
      };

  factory RuleDraftChannel.fromJson(Map<String, dynamic> json) {
    final voucher =
        (json['voucher'] as Map?)?.cast<String, dynamic>() ?? const {};
    final rawCount = json['count_per_event'];
    return RuleDraftChannel(
      name: json['name'] as String? ?? '',
      trigger: json['trigger'] as String? ?? '',
      amountCents: (voucher['amount_cents'] as num?)?.toInt() ?? 0,
      amountRule: voucher['amount_rule'] as String? ?? '',
      scope: voucher['scope'] as String? ?? 'all',
      bonusType: json['bonus_type'] as String? ?? '',
      countPerEvent: rawCount is num ? rawCount.toInt() : rawCount ?? 1,
      entry: json['entry'] as String? ?? '',
      expiresAtRule: voucher['expires_at_rule'] as String? ?? '发放时确定',
    );
  }
}

/// 草稿集导出（pretty JSON，便于入仓或人工核对）。
String encodeDraftSet(List<RuleDraft> drafts) =>
    const JsonEncoder.withIndent('  ').convert({
      'drafts': [for (final d in drafts) d.toJson()],
    });
