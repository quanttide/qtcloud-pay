# order 订单与结算（M3）

客户端位置：`lib/screens/order_screen.dart`、`lib/models/order.dart`、`lib/widgets/settle_detail_panel.dart`

服务端模块：[internal/order](../../provider/internal/order)（对照 [docs/order.md](../../provider/docs/order.md)）

## 职责

下单并结算的入口：提交订单（幂等键 = 商户订单号）、展示结算明细。结算编排（核销券 → 扣余额 → 写交易）全在服务端单事务内完成，客户端只提交与展示。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `order_screen.dart` | 下单表单：客户/账户（AccountPicker）+ 商品（`product_id`、`scope`）+ 金额（AmountField）+ 订单号（IdempotencyField）；提交后展示订单与结算明细 |
| `SettleDetailPanel` | 逐项渲染 `settle_detail`（优惠券 → 代金券 → 余额）及余额变化 |

## 模型（JSON 契约与服务端一致）

```dart
class Order {
  final String id;          // 商户订单号（幂等键）
  final String customerId;  // json: customer_id
  final String accountId;   // json: account_id
  final String? productId;  // json: product_id
  final String? scope;      // 业务类型：cloud / course / data
  final int amount;         // 订单金额（分）
  final String status;      // created / settled
  final List<Deduction> settleDetail; // json: settle_detail 结算计划快照（逐项抵扣）
  final DateTime createdAt; // json: created_at
  final DateTime? settledAt;// json: settled_at

  Order.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        customerId = json['customer_id'],
        accountId = json['account_id'],
        productId = json['product_id'],
        scope = json['scope'],
        amount = json['amount'],
        status = json['status'],
        settleDetail = (json['settle_detail'] as List? ?? [])
            .map((e) => Deduction.fromJson(e)).toList(),
        createdAt = DateTime.parse(json['created_at']),
        settledAt = json['settled_at'] != null
            ? DateTime.parse(json['settled_at']) : null;
}
```

`Deduction` 见 [billing.md](billing.md)：`{kind, ref_id?, amount}`。

## API

| 方法 | 路径 | 请求 | 响应 |
|------|------|------|------|
| POST | `/orders` | `{"order_id","customer_id","account_id","product_id?","scope?","amount"}` | 201 `Order` |
| GET | `/orders/{id}` | — | 200 `Order` |

## 关键点（联调）

- `order_id` 幂等：重复 `POST /orders` 返回已有订单，客户端提示「该订单已结算」，不重复扣款
- **余额不足 → 422，订单不落库**：客户端不创建本地订单记录，仅提示余额不足并引导充值
- 结算明细逐项核对：`kind=coupon` 显示券 ID、`kind=voucher` 显示券 ID、`kind=balance` 显示余额扣减；抵扣额与订单金额求和相等（联调验收点）
- 单事务保证余额、券状态、交易、订单四者一致——客户端展示任何一项时以本订单返回为准

## 测试

widget 测试：下单表单必填校验（订单号/账户/金额）、`SettleDetailPanel` 渲染三种抵扣类型、422 余额不足提示。
