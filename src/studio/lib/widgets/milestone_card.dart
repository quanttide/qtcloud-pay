import 'package:flutter/material.dart';

/// 里程碑状态（对应工作台 §一：⬜ 未开始 / 🚧 进行中 / ✅ 已完成）。
enum MilestoneStatus {
  notStarted('⬜', Colors.grey, '未开始'),
  inProgress('🚧', Colors.orange, '进行中'),
  done('✅', Colors.green, '已完成');

  final String icon;
  final Color color;
  final String label;

  const MilestoneStatus(this.icon, this.color, this.label);
}

/// 里程碑状态卡：M1–M5 状态与验收结论。
class MilestoneCard extends StatelessWidget {
  final String id; // 如 M1
  final String name; // 如 账户与账本
  final MilestoneStatus status;
  final String? acceptance; // 验收结论

  const MilestoneCard({
    super.key,
    required this.id,
    required this.name,
    required this.status,
    this.acceptance,
  });

  @override
  Widget build(BuildContext context) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Text(status.icon, style: const TextStyle(fontSize: 20)),
        title: Text('$id $name'),
        subtitle: acceptance == null
            ? Text('状态：${status.label}',
                style: TextStyle(color: status.color))
            : Text('状态：${status.label} · ${acceptance!}',
                style: TextStyle(color: status.color)),
        trailing: Text(
          status.label,
          style: TextStyle(
            color: status.color,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}
