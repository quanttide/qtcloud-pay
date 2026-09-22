# transfer 代金券转赠

包：`internal/transfer`（transport / service / repository / model + gorm/）

## 职责

转赠机制用于实训基地“发展下线”和“积分制”场景。系统不直接把一张代金券从 A 账户改到 B 账户，而是记录一条可审计链路：

1. 购买人账户记一笔余额消费流水（`transaction.type=consume`），余额随之扣减。
2. 生成 `pending` 转赠记录，关联 `source_transaction_id`。
3. 负责人审核后进入 `approved` 或 `rejected`。
4. `approved` 时向受赠人再发行一张新代金券，关联 `issued_voucher_id`。

## 模型

```go
type VoucherTransfer struct {
    ID                  int64
    FromAccountID       string
    ToAccountID         string
    SourceTransactionID int64
    Amount              int64  // 分
    TaxRateBP           int    // 万分比，当前 0
    Status              string // pending / approved / rejected
    ReviewedBy          string
    ReviewedAt          *time.Time
    IssuedVoucherID     *int64
    RefundTransactionID *int64
    RefundedBy          string
    RefundedAt          *time.Time
    Note                string
    IdempotencyKey      string // 唯一
}
```

再发行金额按整数分计算：`amount × (10000 - tax_rate_bp) / 10000`，向下取整；当前税率默认 0。

## API

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/transfers` | `transfer:write` | 创建转赠购买，余额不足返回 422，整体回滚 |
| GET | `/transfers` | `transfer:read` | 列表，支持 `status` / `account_id` 过滤 |
| GET | `/transfers/{id}` | `transfer:read` | 详情，含三方审计 ID |
| POST | `/transfers/{id}/review` | `transfer:review` | 审核，`approved` 时再发行代金券 |
| POST | `/transfers/{id}/refund` | `transfer:refund` | 对 rejected 转赠办理人工退款留痕 |

创建请求：

```json
{
  "from_account_id": "acc_x",
  "to_account_id": "acc_y",
  "amount": {"amount": 50000, "currency": "CNY"},
  "note": "实训基地积分转赠",
  "idempotency_key": "transfer-20260914-001"
}
```

审核请求：

```json
{
  "decision": "approved",
  "reviewed_by": "赵子奕",
  "note": "通过"
}
```

人工退款请求：

```json
{
  "refunded_by": "赵子奕",
  "note": "线下退款已处理"
}
```

## 拒绝与退款

`rejected` 表示交易作废：系统不向受赠人发券，购买人已扣金额应按原支付路径退回。退款由管理员人工操作，系统只负责留痕和账本一致性。

人工退款流程：

1. 审核人将转赠置为 `rejected`。
2. 管理员完成线下退款或内部处理后，调用 `POST /transfers/{id}/refund`。
3. 系统锁定购买人账户，将 `amount` 全额加回余额，追加一条正向入账流水，写入 `refund_transaction_id`、`refunded_by`、`refunded_at`。
4. 已退款的转赠重复调用退款接口时返回既有记录，不重复加钱。

购买流程提示文案（供前端/接入方展示）：

> 审核未通过时，已支付金额将按原支付路径退回；您可修改后重新提交购买申请。

重新提交无需复用原单：申请人使用新的 `idempotency_key` 创建新转赠记录；原记录保持 `rejected` 终态，退款留痕通过 `refund_transaction_id` 追溯。

## 幂等与失败语义

- 创建幂等：`voucher_transfers.idempotency_key` 唯一；购买流水幂等键为 `transfer:{idempotency_key}`。
- 审核幂等：`approved` / `rejected` 为终态，重复审核返回既有结果，不重复发行。
- 退款幂等：`refund_transaction_id` 非空时重复请求返回既有记录，不重复加回余额；退款流水幂等键为 `transfer-refund:{transfer_id}`。
- 退款限制：仅 `rejected` 可退款；`pending` / `approved` 退款返回 400。
- 余额不足：返回 422，既不写消费流水，也不写转赠记录。
- 冲正补救：历史交易只追加不修改；错误补救后续走反向交易或补偿记录，不改历史行。

## 对账

`GET /reconcile/consistency` 同时校验：

- 转赠购买流水存在，且账户、类型、金额与转赠记录一致。
- `pending` / `rejected` 不应有关联代金券。
- `approved` 必须有关联代金券，且受赠账户与再发行金额一致。
- `rejected` 且已关联退款时，退款流水必须存在，且账户为购买人、金额为 `amount`、类型为正向入账；`rejected` 但未退款是正常待处理状态，不作为差异。
