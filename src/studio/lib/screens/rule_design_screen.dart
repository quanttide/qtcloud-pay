import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../models/rule_draft.dart';
import '../services/local_draft_store.dart';
import '../widgets/amount_field.dart';

/// 规则设计页：代金券发放规则离线草稿（本地 JSON 存储，不联网）。
///
/// 设计产物是服务端计价规则集 payload 的 issuance 段：草稿校验通过后
/// 导出 PUT 请求体，联调时由管理员提交到 /admin/voucher-pricing-rules/{id}。
class RuleDesignScreen extends StatefulWidget {
  const RuleDesignScreen({super.key, this.store});

  /// 可注入的本地草稿仓（测试用）；默认存 ~/.qtcloud-pay-studio/rule_drafts.json。
  final LocalDraftStore? store;

  @override
  State<RuleDesignScreen> createState() => _RuleDesignScreenState();
}

class _RuleDesignScreenState extends State<RuleDesignScreen> {
  late final LocalDraftStore _store;
  List<RuleDraft> _drafts = [];
  String? _selectedId;
  String? _loadWarning;

  @override
  void initState() {
    super.initState();
    _store = widget.store ?? LocalDraftStore();
    _drafts = _store.load();
    if (_drafts.isEmpty) {
      _loadWarning = null;
    } else {
      _selectedId = _drafts.first.id;
    }
  }

  RuleDraft? get _selected {
    for (final d in _drafts) {
      if (d.id == _selectedId) return d;
    }
    return null;
  }

  void _persist() {
    final ok = _store.save(_drafts);
    if (!ok && mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('本地保存失败（检查磁盘权限）')),
      );
    }
    setState(() {});
  }

  Future<void> _newDraft() async {
    final controller = TextEditingController();
    final id = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('新建规则草稿'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '规则集 ID（如 qtclass-voucher-pricing）',
            border: OutlineInputBorder(),
          ),
        ),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          FilledButton(
              onPressed: () => Navigator.pop(ctx, controller.text.trim()),
              child: const Text('创建')),
        ],
      ),
    );
    if (id == null || id.isEmpty) return;
    if (_drafts.any((d) => d.id == id)) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text('已存在同 ID 草稿：$id')));
      }
      return;
    }
    final draft = RuleDraft(id: id);
    // 预置一条空渠道，进入页面即可开始填。
    draft.channels.add(RuleDraftChannel());
    _drafts.add(draft);
    _selectedId = id;
    _persist();
  }

  void _deleteDraft(String id) {
    _drafts.removeWhere((d) => d.id == id);
    if (_selectedId == id) {
      _selectedId = _drafts.isEmpty ? null : _drafts.first.id;
    }
    _persist();
  }

  Future<void> _editChannel(RuleDraft draft, {int? index}) async {
    final channel = index != null
        ? draft.channels[index]
        : RuleDraftChannel(countPerEvent: 1);
    final saved = await showDialog<RuleDraftChannel>(
      context: context,
      builder: (ctx) => _ChannelDialog(initial: channel),
    );
    if (saved == null) return;
    if (index != null) {
      draft.channels[index] = saved;
    } else {
      draft.channels.add(saved);
    }
    draft.updatedAt = DateTime.now();
    _persist();
  }

  Future<void> _export(RuleDraft draft) async {
    final problems = draft.validate();
    if (problems.isNotEmpty) {
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text('草稿还有 ${problems.length} 处不满足校验'),
          content: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                for (final p in problems) Text('· $p'),
                const SizedBox(height: 8),
                Text(RuleDraft.uploadNote,
                    style: Theme.of(ctx).textTheme.bodySmall),
              ],
            ),
          ),
          actions: [
            TextButton(
                onPressed: () => Navigator.pop(ctx), child: const Text('回去改')),
          ],
        ),
      );
      return;
    }
    const encoder = JsonEncoder.withIndent('  ');
    final body = encoder.convert(draft.toPutBody());
    await Clipboard.setData(ClipboardData(text: body));
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('已复制 PUT /admin/voucher-pricing-rules/${draft.id} 请求体'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final draft = _selected;
    return Row(
      children: [
        // 左栏：草稿列表。
        SizedBox(
          width: 260,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.all(12),
                child: SizedBox(
                  width: double.infinity,
                  child: FilledButton.icon(
                    onPressed: _newDraft,
                    icon: const Icon(Icons.add),
                    label: const Text('新建草稿'),
                  ),
                ),
              ),
              if (_loadWarning != null)
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  child: Text(_loadWarning!,
                      style: Theme.of(context).textTheme.bodySmall),
                ),
              Expanded(
                child: _drafts.isEmpty
                    ? const Center(child: Text('还没有草稿'))
                    : ListView(
                        children: [
                          for (final d in _drafts)
                            ListTile(
                              selected: d.id == _selectedId,
                              title: Text(d.id),
                              subtitle: Text(
                                  '${d.channels.length} 个渠道 · ${d.updatedAt.toLocal().toString().split(' ').first}'),
                              trailing: IconButton(
                                icon: const Icon(Icons.delete_outline),
                                onPressed: () => _deleteDraft(d.id),
                              ),
                              onTap: () =>
                                  setState(() => _selectedId = d.id),
                            ),
                        ],
                      ),
              ),
            ],
          ),
        ),
        const VerticalDivider(width: 1),
        // 右栏：选中草稿的编辑区。
        Expanded(
          child: draft == null
              ? const Center(child: Text('新建或选择一个草稿开始设计发放规则'))
              : _DraftEditor(
                  key: ValueKey(draft.id),
                  draft: draft,
                  onEditChannel: (i) => _editChannel(draft, index: i),
                  onAddChannel: () => _editChannel(draft),
                  onRemoveChannel: (i) {
                    draft.channels.removeAt(i);
                    draft.updatedAt = DateTime.now();
                    _persist();
                  },
                  onExport: () => _export(draft),
                  onMetaChanged: _persist,
                ),
        ),
      ],
    );
  }
}

/// 草稿编辑区：元信息、发放渠道列表、面额规则、校验与导出。
class _DraftEditor extends StatelessWidget {
  const _DraftEditor({
    super.key,
    required this.draft,
    required this.onEditChannel,
    required this.onAddChannel,
    required this.onRemoveChannel,
    required this.onExport,
    required this.onMetaChanged,
  });

  final RuleDraft draft;
  final void Function(int index) onEditChannel;
  final VoidCallback onAddChannel;
  final void Function(int index) onRemoveChannel;
  final VoidCallback onExport;
  final VoidCallback onMetaChanged;

  @override
  Widget build(BuildContext context) {
    final problems = draft.validate();
    final text = Theme.of(context).textTheme;
    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Text('规则集 ${draft.id}', style: text.headlineSmall),
        const SizedBox(height: 12),
        // 元信息。
        Row(
          children: [
            Expanded(
              child: TextFormField(
                initialValue: draft.source,
                decoration: const InputDecoration(
                  labelText: '事实来源（如 journal qtclass/2026-09-01.md）',
                  border: OutlineInputBorder(),
                ),
                onChanged: (v) {
                  draft.source = v;
                  onMetaChanged();
                },
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: TextFormField(
                initialValue: draft.version,
                decoration: const InputDecoration(
                  labelText: '版本（如 2026-09-02）',
                  border: OutlineInputBorder(),
                ),
                onChanged: (v) {
                  draft.version = v;
                  onMetaChanged();
                },
              ),
            ),
          ],
        ),
        const SizedBox(height: 24),
        Row(
          children: [
            Expanded(child: Text('发放渠道', style: text.titleLarge)),
            FilledButton.tonalIcon(
              onPressed: onAddChannel,
              icon: const Icon(Icons.add),
              label: const Text('添加渠道'),
            ),
          ],
        ),
        const SizedBox(height: 8),
        for (var i = 0; i < draft.channels.length; i++)
          _ChannelCard(
            channel: draft.channels[i],
            onEdit: () => onEditChannel(i),
            onRemove: () => onRemoveChannel(i),
          ),
        const SizedBox(height: 16),
        TextFormField(
          initialValue: draft.bonusDenominationRule,
          decoration: const InputDecoration(
            labelText: '面额规则（bonus_denomination_rule，含追加奖励渠道时必填）',
            border: OutlineInputBorder(),
          ),
          onChanged: (v) {
            draft.bonusDenominationRule = v;
            onMetaChanged();
          },
        ),
        const SizedBox(height: 24),
        // 校验结果。
        Card(
          color: problems.isEmpty
              ? Theme.of(context).colorScheme.secondaryContainer
              : Theme.of(context).colorScheme.errorContainer,
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  problems.isEmpty
                      ? '发放段校验通过（共 ${draft.channels.length} 个渠道）'
                      : '还有 ${problems.length} 处不满足校验：',
                  style: text.titleSmall,
                ),
                for (final p in problems) ...[
                  const SizedBox(height: 4),
                  Text('· $p'),
                ],
                const SizedBox(height: 8),
                Text(RuleDraft.uploadNote, style: text.bodySmall),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),
        Row(
          children: [
            FilledButton.icon(
              onPressed: onExport,
              icon: const Icon(Icons.copy),
              label: const Text('校验通过后复制 PUT 请求体'),
            ),
            const SizedBox(width: 12),
            Text('草稿保存在本地 ${LocalDraftStore().filePath}',
                style: text.bodySmall?.copyWith(color: Colors.grey.shade600)),
          ],
        ),
      ],
    );
  }
}

class _ChannelCard extends StatelessWidget {
  const _ChannelCard({
    required this.channel,
    required this.onEdit,
    required this.onRemove,
  });

  final RuleDraftChannel channel;
  final VoidCallback onEdit;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    final amountLabel = channel.isBonus
        ? '按规则：${channel.amountRule}'
        : '面额：${(channel.amountCents / 100).toStringAsFixed(2)} 元';
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(channel.isBonus
            ? Icons.stars_outlined
            : Icons.card_giftcard_outlined),
        title: Text(channel.name.isEmpty ? '（未命名渠道）' : channel.name),
        subtitle: Text([
          if (channel.trigger.isNotEmpty) '触发：${channel.trigger}',
          amountLabel,
          '范围：${channel.scope}',
          if (channel.isBonus)
            '类型：${RuleDraftChannel.bonusTypes[channel.bonusType] ?? channel.bonusType}',
          '每次事件：${channel.countPerEvent}',
        ].join('\n')),
        trailing: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            IconButton(
                icon: const Icon(Icons.edit_outlined), onPressed: onEdit),
            IconButton(
                icon: const Icon(Icons.delete_outline), onPressed: onRemove),
          ],
        ),
        onTap: onEdit,
      ),
    );
  }
}

/// 渠道编辑对话框：普通渠道（固定面额）与追加奖励渠道（动态规则）二选一。
class _ChannelDialog extends StatefulWidget {
  const _ChannelDialog({required this.initial});

  final RuleDraftChannel initial;

  @override
  State<_ChannelDialog> createState() => _ChannelDialogState();
}

class _ChannelDialogState extends State<_ChannelDialog> {
  late final RuleDraftChannel _channel;
  late final TextEditingController _nameCtrl;
  late final TextEditingController _triggerCtrl;
  late final TextEditingController _amountCtrl; // 元（固定面额）
  late final TextEditingController _amountRuleCtrl;
  late final TextEditingController _countCtrl;
  late final TextEditingController _entryCtrl;

  @override
  void initState() {
    super.initState();
    // 复制一份，取消不污染原对象。
    _channel = RuleDraftChannel(
      name: widget.initial.name,
      trigger: widget.initial.trigger,
      amountCents: widget.initial.amountCents,
      amountRule: widget.initial.amountRule,
      scope: widget.initial.scope,
      bonusType: widget.initial.bonusType,
      countPerEvent: widget.initial.countPerEvent,
      entry: widget.initial.entry,
      expiresAtRule: widget.initial.expiresAtRule,
    );
    _nameCtrl = TextEditingController(text: _channel.name);
    _triggerCtrl = TextEditingController(text: _channel.trigger);
    _amountCtrl = TextEditingController(
        text: _channel.amountCents > 0
            ? (_channel.amountCents / 100).toStringAsFixed(2)
            : '');
    _amountRuleCtrl = TextEditingController(text: _channel.amountRule);
    _countCtrl = TextEditingController(text: '${_channel.countPerEvent}');
    _entryCtrl = TextEditingController(text: _channel.entry);
  }

  @override
  void dispose() {
    _nameCtrl.dispose();
    _triggerCtrl.dispose();
    _amountCtrl.dispose();
    _amountRuleCtrl.dispose();
    _countCtrl.dispose();
    _entryCtrl.dispose();
    super.dispose();
  }

  void _submit() {
    _channel
      ..name = _nameCtrl.text.trim()
      ..trigger = _triggerCtrl.text.trim()
      ..amountCents = _channel.isBonus
          ? 0
          : (_amountCtrl.text.trim().isEmpty
              ? 0
              : AmountField.yuanToCents(_amountCtrl.text))
      ..amountRule = _amountRuleCtrl.text.trim()
      ..countPerEvent = int.tryParse(_countCtrl.text.trim()) ??
          _countCtrl.text.trim()
      ..entry = _entryCtrl.text.trim();
    final problems = _channel.validate(_channel.name.isEmpty ? '渠道' : _channel.name);
    if (problems.isNotEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(problems.first)),
      );
      return;
    }
    Navigator.pop(context, _channel);
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: Text(widget.initial.name.isEmpty ? '添加发放渠道' : '编辑发放渠道'),
      content: SizedBox(
        width: 480,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: _nameCtrl,
                decoration: const InputDecoration(
                  labelText: '渠道名称（如 课堂实训任务）',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _triggerCtrl,
                decoration: const InputDecoration(
                  labelText: '触发条件（如 实训任务验收通过）',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: _channel.bonusType,
                decoration: const InputDecoration(
                  labelText: '渠道类型',
                  border: OutlineInputBorder(),
                ),
                items: [
                  const DropdownMenuItem(value: '', child: Text('普通渠道（固定面额）')),
                  for (final e in RuleDraftChannel.bonusTypes.entries)
                    DropdownMenuItem(
                        value: e.key, child: Text('追加奖励·${e.value}')),
                ],
                onChanged: (v) => setState(() => _channel.bonusType = v ?? ''),
              ),
              const SizedBox(height: 12),
              if (_channel.isBonus)
                TextField(
                  controller: _amountRuleCtrl,
                  decoration: const InputDecoration(
                    labelText: '面额规则（如 仅限 100 元或 500 元面额）',
                    border: OutlineInputBorder(),
                  ),
                )
              else
                TextField(
                  controller: _amountCtrl,
                  decoration: const InputDecoration(
                    labelText: '面值（元，如 100）',
                    border: OutlineInputBorder(),
                  ),
                ),
              const SizedBox(height: 12),
              DropdownButtonFormField<String>(
                initialValue: _channel.scope,
                decoration: const InputDecoration(
                  labelText: '适用范围',
                  border: OutlineInputBorder(),
                ),
                items: [
                  for (final s in RuleDraftChannel.scopes)
                    DropdownMenuItem(
                        value: s,
                        child: Text(s == 'all' ? '全场' : s)),
                ],
                onChanged: (v) => setState(() => _channel.scope = v ?? 'all'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _countCtrl,
                decoration: const InputDecoration(
                  labelText: '每次事件发放张数（数字或描述，如「多张券组合」）',
                  border: OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _entryCtrl,
                decoration: const InputDecoration(
                  labelText: '依据（如 追加奖励规则（补充）第 1 条）',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
            onPressed: () => Navigator.pop(context), child: const Text('取消')),
        FilledButton(onPressed: _submit, child: const Text('保存')),
      ],
    );
  }
}
