# 对账 API

## 账单导出

```http
GET /accounts/{id}/statement
```

成功响应（200）：

```json
{
  "account_id": "acc_3f2a...",
  "opening_balance": 0,
  "closing_balance": 10000,
  "entries": [
    {"id": 1, "type": "recharge", "amount": 20000, "running_balance": 20000, "created_at": "..."},
    {"id": 2, "type": "consume", "amount": 10000, "running_balance": 10000, "created_at": "..."}
  ],
  "generated_at": "2026-08-03T01:00:00Z"
}
```

按时间正序，含每笔交易的运行余额；`closing_balance − opening_balance = Σ交易净变动`。

错误：`404`（账户不存在）。

## 余额-交易一致性校验

```http
GET /reconcile/consistency
```

逐账户校验 `余额 = Σ充值 − Σ退款 − Σ余额支付`。

成功响应（200）：

```json
{"discrepancies": []}
```

`discrepancies` 非空时逐项为 `{account_id, balance, expected}`——即余额字段与交易求和不一致的账户。

## 对公打款核对

```http
POST /reconcile/bank
Content-Type: multipart/form-data（字段名 file）
```

上传银行流水 CSV 与充值交易按凭证号比对。CSV 格式：`voucher_no,amount_cents,date`（首行为表头时可省略；金额为整数分）。

成功响应（200）：

```json
{
  "total": 2,
  "matched": [{"row": {"voucher_no": "GT-001", "amount_cents": 20000, "date": "2026-08-03"}, "transaction_id": 1}],
  "unmatched": [{"row": {"voucher_no": "GT-999", "amount_cents": 5000, "date": "2026-08-03"}, "reason": "未找到对应充值交易"}]
}
```

`unmatched.reason`：`未找到对应充值交易` / `金额不一致`。

错误：`400`（CSV 格式非法）。
