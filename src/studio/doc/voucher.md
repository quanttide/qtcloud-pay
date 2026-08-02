# voucher 代金券（M2）

客户端位置：`lib/screens/coupon_screen.dart`（发券页）、`lib/models/voucher.dart`

服务端模块：[internal/voucher](../../provider/internal/voucher)（对照 [docs/voucher.md](../../provider/docs/voucher.md)）

## 职责

面值抵现券的发放（批量 + 幂等）与查询。代金券本身**就是钱**，结算时直接抵减应付款项——客户端仅渲染面值表单，抵现计算在服务端 `billing`。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `coupon_screen.dart` | 发券页「代金券」页签：面值（AmountField）+ scope + 有效期 + 批次号 + 数量 |
| `StatusChip` | 券状态标签：`issued` / `used` / `expired` |
| `IdempotencyField` | 发放批次号（必填） |

## 模型（JSON 契约与服务端一致）

```dart
class Voucher {
  final int id;
  final String accountId;   // json: account_id
  final int amount;         // 面值（分），等价现金
  final String scope;       // all / cloud / course / data / product
  final String? productId;  // json: product_id（scope=product 时）
  final DateTime expiresAt; // json: expires_at
  final String status;      // issued / used / expired
  final DateTime? usedAt;   // json: used_at
  final String? orderId;    // json: order_id
  final DateTime createdAt; // json: created_at

  Voucher.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        accountId = json['account_id'],
        amount = json['amount'],
        scope = json['scope'],
        productId = json['product_id'],
        expiresAt = DateTime.parse(json['expires_at']),
        status = json['status'],
        usedAt = json['used_at'] != null ? DateTime.parse(json['used_at']) : null,
        orderId = json['order_id'],
        createdAt = DateTime.parse(json['created_at']);
}
```

## API

| 方法 | 路径 | 请求 | 响应 |
|------|------|------|------|
| POST | `/accounts/{id}/vouchers` | `{"amount","scope","product_id?","expires_at","count","batch_no","note?"}` | 200 `{"account_id","batch_no","count"}` |
| GET | `/accounts/{id}/vouchers` | — | 200 `{"account_id","vouchers": [...]}` |

## 关键点（联调）

- `batch_no` 幂等：同批次重复提交不重发
- v0.1.0 不做部分使用（未用完不退还）：结算时整张核销，客户端按「已使用」展示，不展示部分使用状态
- 发放生成发券交易（`type=issue`），与 coupon 同一流水
- 枚举与 scope 值必须与服务端一致（见 [conventions](conventions.md)）

## 测试

widget 测试：面值必填与转分、发放结果 `{account_id, batch_no, count}` 展示、代金券列表与状态 Chip 渲染。
