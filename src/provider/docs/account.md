# account 账户与余额（M1）

包：`internal/account`（transport / service / repository / model + gorm/）

## 职责

创建账户（客户虚拟钱包）；充值登记（对公打款入账，带幂等键）；退款登记（多退：对公退款出账，带幂等键）；余额查询。余额是交易的投影，与交易同事务维护。

## 依赖

- `transaction`：充值/退款 → 写入对应交易

## 模型

```go
type Account struct {
    ID         string // 业务号，如 acc_xxx
    CustomerID string
    Balance    int64  // 余额（分），与交易同事务维护
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

## 核心流程

### 充值

```go
func (s *Service) Recharge(ctx context.Context, accountID string,
    amount int64, voucherNo, note string) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        acc := s.repo.GetForUpdate(tx, accountID) // 锁账户，防并发重复入账
        acc.Balance += amount
        s.repo.Update(tx, acc)
        return s.txService.Append(tx, &transaction.Transaction{
            AccountID:     accountID,
            Type:          transaction.TypeRecharge,
            Amount:        amount,
            BalanceAfter:  acc.Balance,
            IdempotencyKey: "recharge:" + voucherNo,
            Note:          note,
        }) // 账本写入唯一入口
    })
}
```

## 关键点

- 幂等：重复提交同 `voucherNo` 触发 `idempotency_key` 唯一冲突，查回已有交易直接返回成功，不重复入账（不重）
- 退款与充值对称：幂等键 `refund:{voucher_no}`，余额不足整体回滚（422），余额与退款交易同事务提交
- 余额与交易同事务提交（不错）

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts` | 创建账户 |
| POST | `/accounts/{id}/recharges` | 充值登记（对公打款入账） |
| POST | `/accounts/{id}/refunds` | 退款登记（多退：对公退款出账，余额不足 422） |
| GET | `/accounts/{id}` | 账户与余额 |
| GET | `/accounts/{id}/transactions` | 交易流水（委托 transaction 模块） |

## 测试

mock repository 单测；SQLite `:memory:` 集成测充值、重复提交幂等。
