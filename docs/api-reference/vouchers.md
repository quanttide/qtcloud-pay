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
| `amount` | number | 是 | 面值（元），正数 |
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

## 录入或更新代金券计价规则集

```http
PUT /admin/voucher-pricing-rules/{id}
Content-Type: application/json
X-Admin-Token: <ADMIN_TOKEN>

{
  "source": "payment-engineering/qtclass/voucher-pricing.json",
  "version": "2026-09-01",
  "payload": {
    "issuance": {"channels": []},
    "redemption": {"scenarios": []},
    "billing_semantics": {"voucher_is_money": true}
  }
}
```

规则集用于录入实训基地等外部计价事实，幂等键为路径中的 `{id}`；重复 `PUT` 同一 `{id}` 会更新 `source/version/payload`，不会影响已发放代金券或订单结算。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source` | string | 否 | 事实来源 |
| `version` | string | 否 | 规则版本或更新时间 |
| `payload` | object | 是 | 机器可读规则事实 |

payload 校验：

- 发行渠道 `voucher.amount_cents` 必须为正整数分，`scope` 必须为 `all` / `course` / `data` / `cloud` / `product`。
- 一对一咨询使用 `pricing_model=per_hour_by_rank` 和 `rank_prices_cents` 表示服务者职级档位价。
- 超额申请额度使用 `pricing_model=per_count_flat` 和 `quotas.free_limit` / `quotas.exceed_price_cents` 表示流程配额。
- `billing_semantics.voucher_is_money` 必须为 `true`；开放问题原样保存在 payload。

成功响应（200）返回完整规则集；`403` 表示未配置或未携带正确 `X-Admin-Token`。

## 查询代金券计价规则集

```http
GET /admin/voucher-pricing-rules/{id}
X-Admin-Token: <ADMIN_TOKEN>
```

```http
GET /admin/voucher-pricing-rules
X-Admin-Token: <ADMIN_TOKEN>
```

成功响应（200）返回单个规则集或 `{"rule_sets":[...]}`。
