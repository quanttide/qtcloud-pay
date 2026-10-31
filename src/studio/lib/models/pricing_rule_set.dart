/// 代金券计价规则集（服务端 GET /admin/voucher-pricing-rules）。
///
/// payload 为服务端透传的原始机器 JSON（结构随事实来源演进），
/// 客户端按「发放渠道（issuance.channels）+ 面额规则」做防御性渲染，
/// 未识别字段原样保留在 [raw] 中。
class PricingRuleSet {
  final String id;
  final String? source;
  final String? version;
  final Map<String, dynamic> raw;
  final DateTime updatedAt;

  const PricingRuleSet({
    required this.id,
    this.source,
    this.version,
    required this.raw,
    required this.updatedAt,
  });

  factory PricingRuleSet.fromJson(Map<String, dynamic> json) =>
      PricingRuleSet(
        id: json['id'] as String,
        source: json['source'] as String?,
        version: json['version'] as String?,
        raw: (json['payload'] as Map?)?.cast<String, dynamic>() ?? const {},
        updatedAt: DateTime.parse(json['updated_at'] as String),
      );

  /// 发放渠道列表（payload.issuance.channels）。
  List<Map<String, dynamic>> get issuanceChannels => ((raw['issuance']
              as Map?)?['channels'] as List? ??
          const [])
      .map((e) => (e as Map).cast<String, dynamic>())
      .toList();

  /// 追加奖励面额规则（payload.issuance.bonus_denomination_rule）。
  String? get bonusDenominationRule =>
      ((raw['issuance'] as Map?)?['bonus_denomination_rule'] as String?) ??
      raw['bonus_denomination_rule'] as String?;
}

/// 单条发放渠道（payload.issuance.channels[]）。
class IssuanceChannel {
  final String name;
  final String trigger;
  final String scope;
  final String? amountDisplay; // 人读面额（如「100 元」），来源端提供
  final int? amountCents; // 固定面额（分）
  final String? amountRule; // 动态面额规则描述
  final String? bonusType; // 追加奖励类型
  final Object? countPerEvent; // 每次事件发放张数（数字或描述）
  final String? entry;

  const IssuanceChannel({
    required this.name,
    required this.trigger,
    required this.scope,
    this.amountDisplay,
    this.amountCents,
    this.amountRule,
    this.bonusType,
    this.countPerEvent,
    this.entry,
  });

  factory IssuanceChannel.fromJson(Map<String, dynamic> json) {
    final voucher = (json['voucher'] as Map?)?.cast<String, dynamic>() ?? {};
    return IssuanceChannel(
      name: json['name'] as String? ?? '',
      trigger: json['trigger'] as String? ?? '',
      scope: voucher['scope'] as String? ?? '',
      amountDisplay: voucher['amount_display'] as String?,
      amountCents: (voucher['amount_cents'] as num?)?.toInt(),
      amountRule: voucher['amount_rule'] as String?,
      bonusType: json['bonus_type'] as String?,
      countPerEvent: json['count_per_event'],
      entry: json['entry'] as String?,
    );
  }

  /// 面额人读文案：优先来源端人读字段，其次固定面额换算，最后动态规则。
  String get amountLabel {
    if (amountDisplay != null) return amountDisplay!;
    if (amountCents != null) {
      return '${(amountCents! / 100).toStringAsFixed(2)} 元';
    }
    if (amountRule != null) return amountRule!;
    return '按规则评定';
  }
}
