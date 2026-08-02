# order 订单与结算（M3）

包：`internal/order`（transport / service / repository / model + gorm/）

## 职责

订单生命周期与结算入口：下单并结算——调用 `billing` 确定抵扣 → 写消费/核销交易 → 更新余额与券状态，全程单事务。

## 依赖

`billing`、`coupon`、`voucher`、`account`、`transaction`

## 模型

```go
type Order struct {
    ID           string          // 商户订单号（幂等键）
    CustomerID   string
    AccountID    string
    ProductID    string
    Amount       int64           // 订单金额（分）
    Status       string          // created / settled
    SettleDetail json.RawMessage // 结算计划快照（逐项抵扣）
    CreatedAt    time.Time
    SettledAt    *time.Time
}
```

## 核心流程

### 结算（单事务编排）

```go
func (s *Service) Settle(ctx context.Context, req *SettleRequest) error {
    return s.db.Transaction(func(tx *gorm.DB) error {
        if s.repo.Get(tx, req.OrderID) != nil {
            return nil // 幂等：已结算过，直接返回
        }
        acc := s.accountRepo.GetForUpdate(tx, req.AccountID) // 锁账户：同账户结算串行化
        coupons := s.couponSvc.Available(tx, acc, req.ProductID)  // 过滤 scope/有效期
        vouchers := s.voucherSvc.Available(tx, acc, req.ProductID)
        plan, err := s.billing.Calculate(req.Amount, coupons, vouchers, acc.Balance)
        if err != nil {
            return err // 余额不足等，整体回滚
        }
        for _, d := range plan { // 执行：核销券、扣余额
            switch d.Kind {
            case KindCoupon:
                s.couponSvc.Use(tx, d.RefID, req.OrderID)
            case KindVoucher:
                s.voucherSvc.Use(tx, d.RefID, req.OrderID)
            case KindBalance:
                acc.Balance -= d.Amount
            }
        }
        s.accountRepo.Update(tx, acc)
        s.txSvc.Append(tx, consumeTx)   // 余额部分：一条 consume
        for _, d := range redeemPlans { // 券抵扣：每张一条 redeem
            s.txSvc.Append(tx, redeemTx)
        }
        return s.repo.Create(tx, orderWithDetail)
    })
}
```

## 关键点

- 幂等：`order.id` 唯一，重复 `POST /orders` 返回已有订单（不重结算）
- 单事务保证余额、券状态、交易、订单四者一致（不错）
- 余额不足：返回业务错误（HTTP 422），订单不落库（v0.1.0 简化，不引入 failed 状态）

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/orders` | 下单并结算 |
| GET | `/orders/{id}` | 订单与结算明细 |

## 测试

mock 各依赖单测编排顺序；SQLite `:memory:` 集成测完整链路（充值 → 发券 → 消费 → 余额与账本正确）。
