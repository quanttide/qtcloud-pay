import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../models/reconciliation.dart';
import '../services/pay_api.dart';
import '../services/pay_store.dart';
import '../widgets/milestone_card.dart';
import '../widgets/reconcile_diff_table.dart';

/// 总览页：里程碑状态（M1–M5）、今日待办（未对账）、快捷入口。
class DashboardScreen extends StatefulWidget {
  final void Function(int index) onNavigate;

  const DashboardScreen({super.key, required this.onNavigate});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  List<Discrepancy>? _discrepancies;
  String? _error;
  bool _loading = false;

  @override
  void initState() {
    super.initState();
    _checkConsistency();
  }

  Future<void> _checkConsistency() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final store = context.read<PayStore>();
      final result = await store.api.checkConsistency();
      setState(() => _discrepancies = result);
    } on ApiException catch (e) {
      setState(() => _error = '对账提醒不可用：${e.message}');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('账本核心工作台 v0.1.0',
              style: Theme.of(context)
                  .textTheme
                  .headlineMedium
                  ?.copyWith(fontWeight: FontWeight.bold)),
          const SizedBox(height: 4),
          Text('里程碑状态 · 今日待办 · 快捷入口',
              style: TextStyle(color: Colors.grey.shade600)),
          const SizedBox(height: 24),
          Text('里程碑进度', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          _milestone(MilestoneStatus.inProgress, 'M1', '账户与账本'),
          _milestone(MilestoneStatus.notStarted, 'M2', '优惠券与代金券'),
          _milestone(MilestoneStatus.notStarted, 'M3', '订单与计费规则'),
          _milestone(MilestoneStatus.notStarted, 'M4', '对账与可查'),
          _milestone(MilestoneStatus.notStarted, 'M5', '支付通道对接（v0.2.0）'),
          const SizedBox(height: 24),
          Text('今日待办', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          if (_loading)
            const Center(child: CircularProgressIndicator())
          else if (_error != null)
            ListTile(
              leading: const Icon(Icons.info_outline),
              title: Text(_error!),
              trailing: TextButton(
                onPressed: _checkConsistency,
                child: const Text('重试'),
              ),
            )
          else if (_discrepancies == null || _discrepancies!.isEmpty)
            const ListTile(
              leading: Icon(Icons.check_circle, color: Colors.green),
              title: Text('对账：余额与交易一致，无差异'),
            )
          else
            Column(
              children: [
                ListTile(
                  leading: Icon(Icons.warning_amber,
                      color: Colors.orange.shade800),
                  title: Text('对账：${_discrepancies!.length} 个账户余额不一致'),
                ),
                ReconcileDiffTable(
                  discrepancies: _discrepancies!,
                  onJump: (accountId) =>
                      widget.onNavigate(1), // 跳转账户页定位
                ),
              ],
            ),
          const SizedBox(height: 24),
          Text('快捷入口', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: [
              _action(context, Icons.add_card, '充值登记', () => widget.onNavigate(3)),
              _action(context, Icons.card_giftcard, '发券', () => widget.onNavigate(4)),
              _action(context, Icons.receipt_long, '订单结算', () => widget.onNavigate(5)),
              _action(context, Icons.fact_check, '对账', () => widget.onNavigate(6)),
            ],
          ),
        ],
      ),
    );
  }

  Widget _milestone(MilestoneStatus status, String id, String name) {
    return MilestoneCard(id: id, name: name, status: status);
  }

  Widget _action(
      BuildContext context, IconData icon, String label, VoidCallback onTap) {
    return ActionChip(
      avatar: Icon(icon, size: 18),
      label: Text(label),
      onPressed: onTap,
    );
  }
}
