import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';


/// 抵扣顺序配置项（v0.1.0 默认顺序，由服务端 BillingRule.priority 表达）。
class _DefaultRule {
  final int priority;
  final String kind;
  final String label;
  final String desc;

  const _DefaultRule(this.priority, this.kind, this.label, this.desc);
}

const _defaultRules = [
  _DefaultRule(1, 'coupon', '优惠券（满减）', '满足门槛（≤ 剩余应付）中力度最大的一张'),
  _DefaultRule(2, 'coupon', '优惠券（折扣）', '按 rate 对剩余应付打折（向下取整）'),
  _DefaultRule(3, 'voucher', '代金券', '逐张抵扣 min(面值, 剩余应付)'),
  _DefaultRule(4, 'balance', '余额', '补足剩余；余额不足则结算拒绝（422）'),
];

/// 本地变更登记条目。
class _ChangeLog {
  final String id;
  final DateTime at;
  final String item;
  final String oldValue;
  final String newValue;

  _ChangeLog({
    required this.id,
    required this.at,
    required this.item,
    required this.oldValue,
    required this.newValue,
  });
}

/// 参数配置页：抵扣顺序展示、券模板、变更登记。
class SettingsScreen extends StatefulWidget {
  const SettingsScreen({super.key});

  @override
  State<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends State<SettingsScreen> {
  final _uuid = const Uuid();
  final List<_ChangeLog> _logs = [];
  final _formKey = GlobalKey<FormState>();
  final _itemCtrl = TextEditingController();
  final _oldCtrl = TextEditingController();
  final _newCtrl = TextEditingController();

  @override
  void dispose() {
    _itemCtrl.dispose();
    _oldCtrl.dispose();
    _newCtrl.dispose();
    super.dispose();
  }

  void _addLog() {
    if (!_formKey.currentState!.validate()) return;
    setState(() {
      _logs.insert(
        0,
        _ChangeLog(
          id: _uuid.v4(),
          at: DateTime.now(),
          item: _itemCtrl.text.trim(),
          oldValue: _oldCtrl.text.trim(),
          newValue: _newCtrl.text.trim(),
        ),
      );
      _itemCtrl.clear();
      _oldCtrl.clear();
      _newCtrl.clear();
    });
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: ConstrainedBox(
        constraints: const BoxConstraints(maxWidth: 720),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('参数配置', style: Theme.of(context).textTheme.headlineMedium),
            const SizedBox(height: 4),
            Text('抵扣顺序由服务端 BillingRule.priority 配置，不改代码可调；本地变更登记用于留痕',
                style: TextStyle(color: Colors.grey.shade600)),
            const SizedBox(height: 24),
            Text('计费抵扣顺序（v0.1.0 默认）',
                style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 8),
            Card(
              child: Column(
                children: [
                  for (final r in _defaultRules)
                    ListTile(
                      leading: CircleAvatar(
                        radius: 14,
                        backgroundColor: Colors.blueGrey.shade100,
                        child: Text('${r.priority}',
                            style: const TextStyle(fontSize: 12)),
                      ),
                      title: Text(r.label),
                      subtitle: Text(r.desc),
                    ),
                ],
              ),
            ),
            const SizedBox(height: 24),
            Text('券参数模板', style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  children: [
                    _templateRow(context, '折扣券', 'rate 整数百分比（90 = 9 折），按剩余应付打折'),
                    const Divider(),
                    _templateRow(
                        context, '满减券', 'threshold 门槛 + amount 减额（分），满足门槛即减'),
                    const Divider(),
                    _templateRow(
                        context, '代金券', 'amount 面值（分），直接抵现 min(面值, 剩余应付)'),
                    const Divider(),
                    _templateRow(context, '适用范围 scope',
                        'all 全场 / cloud 云服务 / course 课程 / data 数据服务 / product 指定商品'),
                  ],
                ),
              ),
            ),
            const SizedBox(height: 24),
            Text('参数变更登记', style: Theme.of(context).textTheme.titleLarge),
            const SizedBox(height: 8),
            Card(
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Form(
                  key: _formKey,
                  child: Column(
                    children: [
                      TextFormField(
                        controller: _itemCtrl,
                        decoration: const InputDecoration(
                          labelText: '变更项（如 抵扣顺序/折扣率/面值）',
                          border: OutlineInputBorder(),
                        ),
                        validator: (v) =>
                            (v == null || v.trim().isEmpty) ? '请输入变更项' : null,
                      ),
                      const SizedBox(height: 12),
                      Row(
                        children: [
                          Expanded(
                            child: TextFormField(
                              controller: _oldCtrl,
                              decoration: const InputDecoration(
                                labelText: '旧值',
                                border: OutlineInputBorder(),
                              ),
                              validator: (v) =>
                                  (v == null || v.trim().isEmpty) ? '必填' : null,
                            ),
                          ),
                          const SizedBox(width: 12),
                          Expanded(
                            child: TextFormField(
                              controller: _newCtrl,
                              decoration: const InputDecoration(
                                labelText: '新值',
                                border: OutlineInputBorder(),
                              ),
                              validator: (v) =>
                                  (v == null || v.trim().isEmpty) ? '必填' : null,
                            ),
                          ),
                        ],
                      ),
                      const SizedBox(height: 12),
                      Align(
                        alignment: Alignment.centerRight,
                        child: FilledButton.icon(
                          onPressed: _addLog,
                          icon: const Icon(Icons.add),
                          label: const Text('登记'),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
            const SizedBox(height: 8),
            if (_logs.isEmpty)
              Text('暂无变更登记',
                  style: TextStyle(color: Colors.grey.shade600))
            else
              for (final log in _logs)
                ListTile(
                  dense: true,
                  leading: const Icon(Icons.edit_note),
                  title: Text('${log.item}：${log.oldValue} → ${log.newValue}'),
                  subtitle: Text('${log.at} · #${log.id.substring(0, 8)}'),
                ),
          ],
        ),
      ),
    );
  }

  Widget _templateRow(BuildContext context, String title, String desc) {
    return Row(
      children: [
        SizedBox(
          width: 120,
          child: Text(title,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  fontWeight: FontWeight.w600)),
        ),
        Expanded(
          child: Text(desc, style: TextStyle(color: Colors.grey.shade700)),
        ),
      ],
    );
  }
}
