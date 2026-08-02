# coupon 优惠券（M2）

客户端位置：`lib/screens/coupon_screen.dart`（发放页）、`lib/models/coupon.dart`

服务端模块：[internal/coupon](../../provider/internal/coupon)（对照 [docs/coupon.md](../../provider/docs/coupon.md)）

## 职责

折扣券/满减券的发放（批量 + 幂等）与查询。优惠券本身**不代表钱**，是抵扣规则——客户端按类型渲染参数表单，不做抵扣计算。

## 页面与组件

| 页面/组件 | 说明 |
|-----------|------|
| `coupon_screen.dart` | 发券页：类型选择（`discount` 折扣券 / `full_reduction` 满减券）→ 按类型展示参数表单 + 批次号（幂等键）+ `count` 批量数量 |
| `StatusChip` | 券状态标签：`issued` 已发放 / `used` 已使用 / `expired` 已过期 |
| `IdempotencyField` | 发放批次号（必填） |
| `AmountField` | 满减券门槛/减额（元→分） |

## 模型（JSON 契约与服务端一致）

```dart
class Coupon {
  final int id;
  final String accountId;   // json: account_id
  final String type;        // discount / full_reduction
  final int? rate;          // 折扣券：整数百分比（90 = 9 折）
  final int? threshold;     // 满减券：门槛（分）
  final int? amount;        // 满减券：减额（分）
  final String scope;       // all / cloud / course / data / product
  final String? productId;  // json: product_id（scope=product 时）
  final DateTime expiresAt; // json: expires_at
  final String status;      // issued / used / expired
  final DateTime? usedAt;   // json: used_at
  final String? orderId;    // json: order_id
  final DateTime createdAt; // json: created_at

  Coupon.fromJson(Map<String, dynamic> json)
      : id = json['id'],
        accountId = json['account_id'],
        type = json['type'],
        rate = json['rate'],
        threshold = json['threshold'],
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
| POST | `/accounts/{id}/coupons` | `{"type","rate?","threshold?","amount?","scope","product_id?","expires_at","count","batch_no","note?"}` | 200 `{"account_id","batch_no","count"}` |
| GET | `/accounts/{id}/coupons` | — | 200 `{"account_id","coupons": [...]}` |

参数随类型变化：`discount` 传 `rate`；`full_reduction` 传 `threshold` + `amount`；`scope=product` 时传 `product_id`。

## 关键点（联调）

- `batch_no` 幂等：同批次重复提交不重发；服务端对重复批次返回成功，客户端提示「该批次已发放」
- 发券本身生成一条发券交易（`type=issue`，note 记批次）——发券页成功后可在流水页查到
- 过期惰性流转：服务端读取/结算时校验 `expiresAt` 并置 `expired`，客户端无需本地定时器，仅按返回状态展示
- 枚举与 scope 值必须与服务端一致（见 [conventions](conventions.md) 枚举表），`scope` 下拉选项直接映射

## 测试

widget 测试：类型切换渲染对应参数、批次号必填、`scope=product` 时商品号必填、券状态 Chip 渲染。
