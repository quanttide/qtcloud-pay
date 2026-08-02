# 账户 API

## 创建账户

```http
POST /accounts
Content-Type: application/json

{"customer_id": "stu-1001"}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `customer_id` | string | 是 | 客户标识（业务侧唯一） |

成功响应（201）：

```json
{"id": "acc_3f2a...", "customer_id": "stu-1001", "balance": 0, "created_at": "2026-08-03T00:00:00Z"}
```

错误：`409`（customer_id 已存在）、`400`（请求体非法）。

## 查询账户与余额

```http
GET /accounts/{id}
```

成功响应（200）：

```json
{"id": "acc_3f2a...", "customer_id": "stu-1001", "balance": 20000, "updated_at": "2026-08-03T01:00:00Z"}
```

错误：`404`（账户不存在）。

## 交易流水

```http
GET /accounts/{id}/transactions?limit=20&offset=0
```

成功响应（200）：

```json
{
  "account_id": "acc_3f2a...",
  "transactions": [
    {"id": 2, "type": "consume", "amount": 10000, "balance_after": 10000, "order_id": "O-GT-1", "created_at": "..."},
    {"id": 1, "type": "recharge", "amount": 20000, "balance_after": 20000, "created_at": "..."}
  ]
}
```

按时间倒序；`limit` 默认 20、最大 100，`offset` 默认 0。

## 充值登记（付费记额度）

对公打款到账后登记入账。

```http
POST /accounts/{id}/recharges
Content-Type: application/json

{"amount": 20000, "voucher_no": "GT-001", "note": "对公打款"}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `amount` | int | 是 | 金额（分），正数 |
| `voucher_no` | string | 是 | 打款凭证号（幂等键） |
| `note` | string | 否 | 备注 |

成功响应（200）：`{"account_id": "acc_3f2a..."}`。重复提交同凭证号返回成功但不重复入账。

错误：`400`（金额非正/缺凭证号）、`404`（账户不存在）。

## 退款登记（多退）

按实际用量结算后多收的款项原路退回。

```http
POST /accounts/{id}/refunds
Content-Type: application/json

{"amount": 100000, "voucher_no": "SJ-R-001", "note": "多退"}
```

字段同充值（`amount` 正数、`voucher_no` 幂等键、`note` 可选）。

成功响应（200）：`{"account_id": "acc_3f2a..."}`。重复提交同凭证号返回成功但不重复退款。

错误：`422`（余额不足，整体回滚）、`400`、`404`。
