# billing 计费规则（M3）

客户端位置：`lib/screens/settings_screen.dart`、`lib/models/billing_rule.dart`

服务端模块：[internal/billing](../../provider/internal/billing)（对照 [docs/billing.md](../../provider/docs/billing.md)）

## 职责

抵扣顺序的查看与配置展示。抵扣计算是服务端纯函数（[`billing.Calculate`](../../provider/internal/billing/service.go)），客户端**不实现任何抵扣逻辑**，只展示配置与结算结果。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `settings_screen.dart` | 参数配置页：按 `priority` 展示抵扣顺序（coupon → voucher → balance）、规则列表、变更登记 |
| `SettleDetailPanel` | 结算明细面板（供订单结算页复用）：逐项渲染 `Deduction` |

## 模型（JSON 契约与服务端一致）

```dart
class BillingRule {
  final int id;
  final int priority;    // 执行顺序（小者先执行）
  final String kind;     // coupon / voucher / balance
  final String? condition; // JSON 条件（scope、min_amount 等）

  BillingRule.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        priority = json['priority'],
        kind = json['kind'],
        condition = json['condition'];
}

/// 结算计划中的一项抵扣（即订单 settle_detail 的元素）。
class Deduction {
  final String kind;  // coupon / voucher / balance
  final int refId;    // json: ref_id 券 ID（balance 时为 0）
  final int amount;   // 抵扣额（分）

  Deduction.fromJson(Map<String, dynamic> json)
      : kind = json['kind'],
        refId = json['ref_id'] ?? 0,
        amount = json['amount'];
}
```

## 抵扣顺序（v0.1.0 默认，全部由 `BillingRule.priority` 表达，不改代码可调）

1. 满减券：满足门槛（≤ 剩余应付）中力度最大的一张
2. 折扣券：按 `rate` 对剩余应付打折（向下取整）
3. 代金券：逐张抵扣 `min(面值, 剩余应付)`
4. 余额：补足剩余

## API

无独立端点；抵扣顺序随订单结算明细（`order.settle_detail`）与规则配置接口提供，客户端只读展示。

## 关键点（联调）

- 参数配置页展示的顺序必须与服务端 `BillingRule.priority` 排序一致，联调时先跑一笔结算对照 `settle_detail`
- 结算拒绝（余额不足 → 422）时订单不落库，客户端订单列表不出现该笔
- 抵扣金额全为整数分，`SettleDetailPanel` 用 `MoneyText` 展示

## 测试

widget 测试：规则按 priority 排序渲染；`Deduction` 三种 kind 的差异化展示（券显示 ref_id、余额显示 0）。
