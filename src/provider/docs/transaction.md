# transaction 交易账本（M1）

包：`internal/transaction`（service / repository / model + gorm/）

## 职责

账本写入唯一入口（幂等键 + 唯一约束，不重）；流水查询（可查）。只插入、不更新、不删除，保证不丢、可追溯。

## 依赖

无（最底层，被所有写账本的模块依赖）。

## 模型

```go
type Transaction struct {
    ID             int64
    AccountID      string
    Type           string // recharge/consume/issue/redeem（充值/消费/发券/核销）
    Amount         int64  // 分；发券/核销为信息性记录，不参与余额求和
    BalanceAfter   int64  // 交易后余额快照，供对账与客诉
    OrderID        string // 消费/核销时关联订单
    IdempotencyKey string `gorm:"uniqueIndex"` // 幂等
    Note           string
    CreatedAt      time.Time
}
```

## 核心流程

### 写入

```go
// Append 是账本写入的唯一入口：只插入、不更新、不删除。
func (s *Service) Append(ctx context.Context, db *gorm.DB, t *Transaction) error {
    err := s.repo.Create(db, t)
    if err != nil { // 唯一冲突（幂等键已存在）→ 视为成功，返回已有记录
        return nil
    }
    return nil
}
```

## 关键点

- 余额求和约定：余额 = Σ(充值) − Σ(余额支付部分)，发券/核销交易不参与
- 快照：`BalanceAfter` 由调用方在同事务内计算（锁账户后余额已知）

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/accounts/{id}/transactions` | 流水查询（分页倒序，路由沿用账户资源） |

## 测试

重复 Append 幂等；不可变约束（无 Update/Delete 方法）。
