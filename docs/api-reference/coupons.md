# 优惠券 API

## 发放优惠券（批量）

```http
POST /accounts/{id}/coupons
Content-Type: application/json

{
  "type": "discount",
  "rate": 90,
  "scope": "course",
  "expires_at": "2026-08-04T00:00:00Z",
  "count": 10,
  "batch_no": "GT-B-001"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | 是 | `discount`（折扣券，`rate` 为力度，9 折 = 90）或 `full_reduction`（满减券，`threshold`+`amount`） |
| `rate` | int | 按类型 | discount 必填：折扣力度（1–99） |
| `threshold` | number | 按类型 | full_reduction 必填：满减门槛（元） |
| `amount` | number | 按类型 | full_reduction 必填：减免金额（元） |
| `scope` | string | 是 | 适用范围：`all` / `course` / `data` / `cloud` / `product` |
| `product_id` | string | 否 | scope=`product` 时必填：指定商品 |
| `expires_at` | string | 是 | 过期时间（RFC3339） |
| `count` | int | 是 | 发放数量（≥1） |
| `batch_no` | string | 是 | 批次号（幂等键） |
| `note` | string | 否 | 备注 |

成功响应（200）：`{"account_id": "acc_3f2a..."}`。重复提交同批次号返回成功但不重复发放。

错误：`400`（参数非法）、`404`（账户不存在）。

## 查询优惠券

```http
GET /accounts/{id}/coupons
```

成功响应（200）：

```json
{
  "coupons": [
    {"id": 1, "type": "discount", "rate": 90, "scope": "course", "status": "issued",
     "expires_at": "2026-08-04T00:00:00Z", "order_id": "", "created_at": "..."}
  ]
}
```

状态：`issued`（已发放）/ `used`（已使用，带 `order_id` 关联）/ `expired`（已过期，查询时惰性流转）。
