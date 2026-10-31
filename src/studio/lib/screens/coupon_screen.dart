import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/coupon.dart';
import '../models/pricing_rule_set.dart';
import '../models/voucher.dart';
import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/account_picker.dart';
import '../widgets/amount_field.dart';
import '../widgets/idempotency_field.dart';
import '../widgets/status_chip.dart';

/// 发券页：优惠券/代金券发放与查询、发放规则只读（Tabs）。
class CouponScreen extends StatefulWidget {
  const CouponScreen({super.key});

  @override
  State<CouponScreen> createState() => _CouponScreenState();
}

class _CouponScreenState extends State<CouponScreen> {
  @override
  Widget build(BuildContext context) {
    return const DefaultTabController(
      length: 3,
      child: Column(
        children: [
          TabBar(tabs: [
            Tab(text: '优惠券'),
            Tab(text: '代金券'),
            Tab(text: '发放规则'),
          ]),
          Expanded(
            child: TabBarView(children: [
              _CouponTab(),
              _VoucherTab(),
              _RulesTab(),
            ]),
          ),
        ],
      ),
    );
  }
}

// ---------- 优惠券 ----------

class _CouponTab extends StatefulWidget {
  const _CouponTab();

  @override
  State<_CouponTab> createState() => _CouponTabState();
}

class _CouponTabState extends State<_CouponTab> {
  final _formKey = GlobalKey<FormState>();
  String? _accountId;
  String _type = CouponType.discount;
  final _rateCtrl = TextEditingController();
  final _thresholdCtrl = TextEditingController();
  final _amountCtrl = TextEditingController();
  String _scope = 'all';
  final _productIdCtrl = TextEditingController();
  DateTime _expiresAt = DateTime.now().add(const Duration(days: 30));
  final _countCtrl = TextEditingController(text: '1');
  final _batchNoCtrl = TextEditingController();
  bool _submitting = false;
  List<Coupon>? _coupons;

  @override
  void dispose() {
    _rateCtrl.dispose();
    _thresholdCtrl.dispose();
    _amountCtrl.dispose();
    _productIdCtrl.dispose();
    _countCtrl.dispose();
    _batchNoCtrl.dispose();
    super.dispose();
  }

  Future<void> _load(String accountId) async {
    final store = context.read<PayStore>();
    _coupons = await store.api.listCoupons(accountId);
    if (mounted) setState(() {});
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    try {
      final store = context.read<PayStore>();
      final result = await store.api.issueCoupons(
        _accountId!,
        type: _type,
        rate: _type == CouponType.discount
            ? int.parse(_rateCtrl.text)
            : null,
        threshold: _type == CouponType.fullReduction
            ? AmountField.yuanToCents(_thresholdCtrl.text)
            : null,
        amount: _type == CouponType.fullReduction
            ? AmountField.yuanToCents(_amountCtrl.text)
            : null,
        scope: _scope,
        productId:
            _scope == 'product' && _productIdCtrl.text.trim().isNotEmpty
                ? _productIdCtrl.text.trim()
                : null,
        expiresAt: _expiresAt,
        count: int.parse(_countCtrl.text),
        batchNo: _batchNoCtrl.text.trim(),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('已发放 ${result.count} 张优惠券（批次 ${result.batchNo}）'),
        ),
      );
      await _load(_accountId!);
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('发放失败：${e.message}')));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Form(
                key: _formKey,
                child: Column(
                  children: [
                    AccountPicker(
                      onSaved: (v) => _accountId = v,
                      validator: (v) => (v == null || v.trim().isEmpty)
                          ? '请选择或输入账户 ID'
                          : null,
                    ),
                    const SizedBox(height: 16),
                    SegmentedButton<String>(
                      segments: const [
                        ButtonSegment(
                          value: CouponType.discount,
                          label: Text('折扣券'),
                        ),
                        ButtonSegment(
                          value: CouponType.fullReduction,
                          label: Text('满减券'),
                        ),
                      ],
                      selected: {_type},
                      onSelectionChanged: (s) => setState(() => _type = s.first),
                    ),
                    const SizedBox(height: 16),
                    if (_type == CouponType.discount)
                      TextFormField(
                        controller: _rateCtrl,
                        decoration: const InputDecoration(
                          labelText: '折扣率（整数百分比，90 = 9 折）',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) {
                          final n = int.tryParse(v ?? '');
                          if (n == null || n <= 0 || n >= 100) {
                            return '请输入 1-99 的整数百分比';
                          }
                          return null;
                        },
                      )
                    else ...[
                      TextFormField(
                        controller: _thresholdCtrl,
                        decoration: const InputDecoration(
                          labelText: '满减门槛（元）',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) =>
                            (v == null || v.trim().isEmpty) ? '请输入门槛' : null,
                      ),
                      const SizedBox(height: 16),
                      AmountField(controller: _amountCtrl, labelText: '减额（元）'),
                    ],
                    const SizedBox(height: 16),
                    DropdownButtonFormField<String>(
                      initialValue: _scope,
                      decoration: const InputDecoration(
                        labelText: '适用范围',
                        border: OutlineInputBorder(),
                      ),
                      items: const [
                        DropdownMenuItem(value: 'all', child: Text('全场')),
                        DropdownMenuItem(value: 'cloud', child: Text('云服务')),
                        DropdownMenuItem(value: 'course', child: Text('课程')),
                        DropdownMenuItem(value: 'data', child: Text('数据服务')),
                        DropdownMenuItem(value: 'product', child: Text('指定商品')),
                      ],
                      onChanged: (v) => setState(() => _scope = v!),
                    ),
                    if (_scope == 'product') ...[
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _productIdCtrl,
                        decoration: const InputDecoration(
                          labelText: '商品 ID',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? '指定商品须填商品 ID'
                            : null,
                      ),
                    ],
                    const SizedBox(height: 16),
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('有效期至'),
                      trailing: Text(_expiresAt.toLocal().toString().split(' ').first),
                      onTap: () async {
                        final picked = await showDatePicker(
                          context: context,
                          initialDate: _expiresAt,
                          firstDate: DateTime.now(),
                          lastDate: DateTime.now().add(const Duration(days: 365 * 3)),
                        );
                        if (picked != null) {
                          setState(() => _expiresAt = picked);
                        }
                      },
                    ),
                    const SizedBox(height: 8),
                    TextFormField(
                      controller: _countCtrl,
                      decoration: const InputDecoration(
                        labelText: '发放数量',
                        border: OutlineInputBorder(),
                      ),
                      validator: (v) {
                        final n = int.tryParse(v ?? '');
                        if (n == null || n <= 0) return '请输入正整数';
                        return null;
                      },
                    ),
                    const SizedBox(height: 16),
                    IdempotencyField(
                      controller: _batchNoCtrl,
                      labelText: '发放批次号',
                      hintText: '如 B20260803001',
                    ),
                    const SizedBox(height: 24),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        onPressed: _submitting ? null : _submit,
                        icon: _submitting
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.card_giftcard),
                        label: const Text('发放'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(height: 16),
          if (_accountId != null && _coupons != null)
            Text('账户 $_accountId 的优惠券',
                style: Theme.of(context).textTheme.titleMedium),
          if (_coupons == null)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('选择账户后可查看已发放优惠券'),
              ),
            )
          else if (_coupons!.isEmpty)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('暂无优惠券'),
              ),
            )
          else
            for (final c in _coupons!)
              ListTile(
                dense: true,
                leading: const Icon(Icons.percent),
                title: Text('${_typeLabel(c.type)} · ${c.paramLabel}'),
                subtitle: Text(
                  '${_scopeLabel(c.scope)} · 有效期至 '
                  '${c.expiresAt.toLocal().toString().split(' ').first}',
                ),
                trailing: StatusChip(status: c.status),
              ),
        ],
      ),
    );
  }

  String _typeLabel(String type) => switch (type) {
        'discount' => '折扣券',
        'full_reduction' => '满减券',
        _ => type,
      };

  String _scopeLabel(String scope) => switch (scope) {
        'all' => '全场',
        'cloud' => '云服务',
        'course' => '课程',
        'data' => '数据服务',
        'product' => '指定商品',
        _ => scope,
      };
}

// ---------- 代金券 ----------

class _VoucherTab extends StatefulWidget {
  const _VoucherTab();

  @override
  State<_VoucherTab> createState() => _VoucherTabState();
}

class _VoucherTabState extends State<_VoucherTab> {
  final _formKey = GlobalKey<FormState>();
  String? _accountId;
  final _amountCtrl = TextEditingController();
  String _scope = 'all';
  final _productIdCtrl = TextEditingController();
  DateTime _expiresAt = DateTime.now().add(const Duration(days: 30));
  final _countCtrl = TextEditingController(text: '1');
  final _batchNoCtrl = TextEditingController();
  bool _submitting = false;
  List<Voucher>? _vouchers;

  @override
  void dispose() {
    _amountCtrl.dispose();
    _productIdCtrl.dispose();
    _countCtrl.dispose();
    _batchNoCtrl.dispose();
    super.dispose();
  }

  Future<void> _load(String accountId) async {
    final store = context.read<PayStore>();
    _vouchers = await store.api.listVouchers(accountId);
    if (mounted) setState(() {});
  }

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;
    setState(() => _submitting = true);
    try {
      final store = context.read<PayStore>();
      final result = await store.api.issueVouchers(
        _accountId!,
        amount: AmountField.yuanToCents(_amountCtrl.text),
        scope: _scope,
        productId:
            _scope == 'product' && _productIdCtrl.text.trim().isNotEmpty
                ? _productIdCtrl.text.trim()
                : null,
        expiresAt: _expiresAt,
        count: int.parse(_countCtrl.text),
        batchNo: _batchNoCtrl.text.trim(),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('已发放 ${result.count} 张代金券（批次 ${result.batchNo}）'),
        ),
      );
      await _load(_accountId!);
    } on ApiException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('发放失败：${e.message}')));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Form(
                key: _formKey,
                child: Column(
                  children: [
                    AccountPicker(
                      onSaved: (v) => _accountId = v,
                      validator: (v) => (v == null || v.trim().isEmpty)
                          ? '请选择或输入账户 ID'
                          : null,
                    ),
                    const SizedBox(height: 16),
                    AmountField(controller: _amountCtrl, labelText: '面值（元）'),
                    const SizedBox(height: 16),
                    DropdownButtonFormField<String>(
                      initialValue: _scope,
                      decoration: const InputDecoration(
                        labelText: '适用范围',
                        border: OutlineInputBorder(),
                      ),
                      items: const [
                        DropdownMenuItem(value: 'all', child: Text('全场')),
                        DropdownMenuItem(value: 'cloud', child: Text('云服务')),
                        DropdownMenuItem(value: 'course', child: Text('课程')),
                        DropdownMenuItem(value: 'data', child: Text('数据服务')),
                        DropdownMenuItem(value: 'product', child: Text('指定商品')),
                      ],
                      onChanged: (v) => setState(() => _scope = v!),
                    ),
                    if (_scope == 'product') ...[
                      const SizedBox(height: 16),
                      TextFormField(
                        controller: _productIdCtrl,
                        decoration: const InputDecoration(
                          labelText: '商品 ID',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) => (v == null || v.trim().isEmpty)
                            ? '指定商品须填商品 ID'
                            : null,
                      ),
                    ],
                    const SizedBox(height: 16),
                    ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('有效期至'),
                      trailing: Text(_expiresAt.toLocal().toString().split(' ').first),
                      onTap: () async {
                        final picked = await showDatePicker(
                          context: context,
                          initialDate: _expiresAt,
                          firstDate: DateTime.now(),
                          lastDate: DateTime.now().add(const Duration(days: 365 * 3)),
                        );
                        if (picked != null) {
                          setState(() => _expiresAt = picked);
                        }
                      },
                    ),
                    const SizedBox(height: 8),
                    TextFormField(
                      controller: _countCtrl,
                      decoration: const InputDecoration(
                        labelText: '发放数量',
                        border: OutlineInputBorder(),
                      ),
                      validator: (v) {
                        final n = int.tryParse(v ?? '');
                        if (n == null || n <= 0) return '请输入正整数';
                        return null;
                      },
                    ),
                    const SizedBox(height: 16),
                    IdempotencyField(
                      controller: _batchNoCtrl,
                      labelText: '发放批次号',
                      hintText: '如 B20260803001',
                    ),
                    const SizedBox(height: 24),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        onPressed: _submitting ? null : _submit,
                        icon: _submitting
                            ? const SizedBox(
                                width: 16,
                                height: 16,
                                child: CircularProgressIndicator(strokeWidth: 2),
                              )
                            : const Icon(Icons.confirmation_number),
                        label: const Text('发放'),
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
          const SizedBox(height: 16),
          if (_vouchers == null)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('选择账户后可查看已发放代金券'),
              ),
            )
          else if (_vouchers!.isEmpty)
            const Center(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('暂无代金券'),
              ),
            )
          else
            for (final v in _vouchers!)
              ListTile(
                dense: true,
                leading: const Icon(Icons.confirmation_number),
                title: Text('面值 ${(v.amount / 100).toStringAsFixed(2)} 元'),
                subtitle: Text(
                  '${_scopeLabel(v.scope)} · 有效期至 '
                  '${v.expiresAt.toLocal().toString().split(' ').first}',
                ),
                trailing: StatusChip(status: v.status),
              ),
        ],
      ),
    );
  }

  String _scopeLabel(String scope) => switch (scope) {
        'all' => '全场',
        'cloud' => '云服务',
        'course' => '课程',
        'data' => '数据服务',
        'product' => '指定商品',
        _ => scope,
      };
}

// ---------- 发放规则（只读） ----------

/// 发放规则页签：代金券计价规则集只读展示（发放渠道、触发条件、面额）。
///
/// 监管视角入口：只展示「规则允许怎么发」，不提供任何发放操作。
class _RulesTab extends StatefulWidget {
  const _RulesTab();

  @override
  State<_RulesTab> createState() => _RulesTabState();
}

class _RulesTabState extends State<_RulesTab>
    with AutomaticKeepAliveClientMixin {
  List<PricingRuleSet>? _ruleSets;
  String? _error;

  @override
  bool get wantKeepAlive => true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _ruleSets = null;
      _error = null;
    });
    try {
      final store = context.read<PayStore>();
      _ruleSets = await store.api.listVoucherPricingRules();
    } on ApiException catch (e) {
      _error = e.message;
    }
    if (mounted) setState(() {});
  }

  @override
  Widget build(BuildContext context) {
    super.build(context);
    if (_error != null) {
      return Center(
        child: ListTile(
          leading: const Icon(Icons.error_outline),
          title: Text('发放规则加载失败：$_error'),
          trailing: TextButton(onPressed: _load, child: const Text('重试')),
        ),
      );
    }
    if (_ruleSets == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_ruleSets!.isEmpty) {
      return const Center(
        child: Padding(
          padding: EdgeInsets.all(16),
          child: Text('暂无发放规则集（管理员可通过 PUT /admin/voucher-pricing-rules 录入）'),
        ),
      );
    }
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        for (final rs in _ruleSets!) ...[
          _RuleSetCard(ruleSet: rs),
          const SizedBox(height: 16),
        ],
      ],
    );
  }
}

class _RuleSetCard extends StatelessWidget {
  const _RuleSetCard({required this.ruleSet});

  final PricingRuleSet ruleSet;

  @override
  Widget build(BuildContext context) {
    final channels = ruleSet.issuanceChannels
        .map(IssuanceChannel.fromJson)
        .toList();
    final text = Theme.of(context).textTheme;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(ruleSet.id, style: text.titleMedium),
                ),
                if (ruleSet.version != null)
                  Chip(
                    label: Text('版本 ${ruleSet.version}'),
                    visualDensity: VisualDensity.compact,
                  ),
              ],
            ),
            if (ruleSet.source != null) ...[
              const SizedBox(height: 4),
              Text('事实来源：${ruleSet.source}',
                  style: text.bodySmall?.copyWith(color: Colors.grey.shade600)),
            ],
            const SizedBox(height: 12),
            Text('发放渠道', style: text.titleSmall),
            const SizedBox(height: 4),
            if (channels.isEmpty)
              const Text('（规则集中无发放渠道）')
            else
              for (final ch in channels) _ChannelTile(channel: ch),
            if (ruleSet.bonusDenominationRule != null) ...[
              const SizedBox(height: 8),
              Text('面额规则：${ruleSet.bonusDenominationRule}',
                  style: text.bodySmall),
            ],
            const SizedBox(height: 4),
            Text(
              '更新于 ${ruleSet.updatedAt.toLocal().toString().split(' ').first}',
              style: text.bodySmall?.copyWith(color: Colors.grey.shade500),
            ),
          ],
        ),
      ),
    );
  }
}

class _ChannelTile extends StatelessWidget {
  const _ChannelTile({required this.channel});

  final IssuanceChannel channel;

  @override
  Widget build(BuildContext context) {
    final subtitles = <String>[
      if (channel.trigger.isNotEmpty) '触发：${channel.trigger}',
      '面额：${channel.amountLabel}',
      if (channel.scope.isNotEmpty) '范围：${channel.scope}',
      if (channel.bonusType != null) '类型：追加奖励（${channel.bonusType}）',
      if (channel.countPerEvent != null) '每次事件：${channel.countPerEvent}',
      if (channel.entry != null) '依据：${channel.entry}',
    ];
    return ListTile(
      dense: true,
      contentPadding: EdgeInsets.zero,
      leading: const Icon(Icons.card_giftcard_outlined),
      title: Text(channel.name),
      subtitle: Text(subtitles.join('\n')),
    );
  }
}
