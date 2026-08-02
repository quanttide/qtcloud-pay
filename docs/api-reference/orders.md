# 订单 API

## 下单并结算

按[计费规则](../user-guide/billing.md)自动抵扣（满减券 → 折扣券 → 代金券 → 余额），同事务更新余额、券状态与账本。

```http
POST /orders
Content-Type: application/json

{
  "order_id": "O-GT-1",
  "account_id": "acc_3f2a...",
  "scope": "course",
  "amount": 100.00
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `order_id` | string | 是 | 商户订单号（幂等键） |
| `account_id` | string | 是 | 账户 ID |
| `scope` | string | 是 | 业务范围：`course` / `data` / `cloud`（券按此匹配） |
| `amount` | number | 是 | 订单金额（元，两位小数），正数 |
| `product_id` | string | 否 | 商品 ID（指定商品券按此匹配） |
| `customer_id` | string | 否 | 客户标识（信息性） |

成功响应（201）含结算明细（抵扣行）：

```json
{
  "id": "O-GT-1",
  "status": "settled",
  "settle_detail": [
    {"kind": "coupon", "ref_id": 1, "amount": 10.00},
    {"kind": "voucher", "ref_id": 1, "amount": 20.00},
    {"kind": "balance", "ref_id": 0, "amount": 70.00}
  ]
}
```

`settle_detail` 的 `kind`：`coupon`（满减/折扣抵扣）/ `voucher`（代金券抵现）/ `balance`（余额支付）。重复提交同订单号返回同一订单，不重复扣款。

错误：`422`（余额不足，整体回滚：无订单、无扣减、无交易写入）、`400`、`404`（账户不存在）、`409`（券不可用）。

## 查询订单

```http
GET /orders/{id}
```

成功响应（200）：订单详情，含 `settle_detail` 结算明细快照。

错误：`404`（订单不存在）。
