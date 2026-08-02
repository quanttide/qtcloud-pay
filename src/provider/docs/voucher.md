# voucher 代金券（M2）

包：`internal/voucher`（transport / service / repository / model + gorm/）

## 职责

面值抵现券的发放（幂等）、过期流转、结算时抵现。代金券本身**就是钱**，结算时直接抵减应付款项。

## 依赖

- `transaction`：发放 → 写入发券交易

## 模型

同 Coupon，去掉 `rate`/`threshold`，固定面值 `Amount`（分）：

```go
type Voucher struct {
    ID        int64
    AccountID string
    	BatchNo   string    // 普通索引：幂等由发券交易幂等键保证（一批多张券共用批次号）
    Amount    int64     // 面值（分）
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

批量 + `batchNo` 幂等，共一条发券交易（type=issue，note 记批次）。

幂等：同 `batchNo` 重复提交不重发——由发券交易幂等键 `issue:voucher:{batch_no}`（全局唯一）保证；`batch_no` 为普通索引，`CountByBatch` 作防御性检查（同一批次多张券共用一个批次号，不能建唯一索引）。

### 抵现（供结算调用）

`Use` 校验 status=issued 且未过期后置 used、关联订单；抵扣额 = `min(面值, 剩余应付)`，由结算计划计算。

## 关键点

- 状态机与过期惰性流转同 coupon
- v0.1.0 不做部分使用（未用完不退还，属力度设计，规则引擎后置）

## API

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts/{id}/vouchers` | 发放（批量 + 幂等） |
| GET | `/accounts/{id}/vouchers` | 查询 |

## 测试

面值 ≥ 应付 / 面值 < 应付两分支、幂等。
