# 代金券 API

## 发放代金券（批量）

```http
POST /accounts/{id}/vouchers
Content-Type: application/json

{
  "amount": 2000,
  "scope": "all",
  "expires_at": "2026-08-04T00:00:00Z",
  "count": 1,
  "batch_no": "GT-V-001"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `amount` | int | 是 | 面值（分），正数 |
| `scope` | string | 是 | 适用范围：`all` / `course` / `data` / `cloud` / `product` |
| `product_id` | string | 否 | scope=`product` 时必填：指定商品 |
| `expires_at` | string | 是 | 过期时间（RFC3339） |
| `count` | int | 是 | 发放数量（≥1） |
| `batch_no` | string | 是 | 批次号（幂等键） |
| `note` | string | 否 | 备注 |

成功响应（200）：`{"account_id": "acc_3f2a..."}`。重复提交同批次号返回成功但不重复发放。

错误：`400`（参数非法）、`404`（账户不存在）。

## 查询代金券

```http
GET /accounts/{id}/vouchers
```

成功响应（200）：

```json
{
  "vouchers": [
    {"id": 1, "amount": 2000, "scope": "all", "status": "issued",
     "expires_at": "2026-08-04T00:00:00Z", "order_id": "", "created_at": "..."}
  ]
}
```

状态：`issued` / `used`（带 `order_id` 关联）/ `expired`（查询时惰性流转）。
