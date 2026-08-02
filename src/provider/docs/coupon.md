# coupon 优惠券（M2）

包：`internal/coupon`（transport / service / repository / model + gorm/）

## 职责

折扣券/满减券的发放（幂等）、过期流转、结算时核销。优惠券本身**不代表钱**，是一条抵扣规则。

## 依赖

- `transaction`：发放 → 写入发券交易

## 模型

```go
type Coupon struct {
    ID        int64
    AccountID string
    BatchNo   string    // 幂等：一批只发一次
    Type      string    // discount（折扣券）/ full_reduction（满减券）
    Rate      int       // 折扣券：整数百分比（90 = 9 折）
    Threshold int64     // 满减券：门槛（分）
    Amount    int64     // 满减券：减额（分）
    Scope     string    // all / cloud / course / data / product
    ProductID string
    ExpiresAt time.Time
    Status    string    // issued / used / expired
    UsedAt    *time.Time
    OrderID   string
    CreatedAt time.Time
}
```

## 核心流程

### 发放

批量发放：`count` + `batchNo`，同批次生成 count 张券，共一条发券交易（type=issue，note 记批次）；同 `batchNo` 重复提交不重发。

### 核销（供结算调用）

```go
// Use 校验 status=issued 且未过期，置 used、关联订单。抵扣额由结算计划决定。
func (s *Service) Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error {
    c := s.repo.GetForUpdate(db, id) // 防并发重复核销
    if c.Status != StatusIssued || time.Now().After(c.ExpiresAt) {
        return ErrUnavailable
    }
    c.Status, c.UsedAt, c.OrderID = StatusUsed, now(), orderID
    return s.repo.Update(db, c)
}
```

## 关键点

- 状态机：`issued → used`（结算核销）/ `issued → expired`（超过有效期）
- 过期惰性流转：不做定时任务，读取与结算时校验 `expiresAt`，发现过期时更新状态
- 抵扣额计算（供 billing 使用）：折扣券 `应付 × rate / 100` 向下取整；满减券门槛内减 amount

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts/{id}/coupons` | 发放（批量 + 幂等） |
| GET | `/accounts/{id}/coupons` | 查询 |

## 测试

发放幂等、过期惰性流转、Use 状态校验与并发重复核销。
