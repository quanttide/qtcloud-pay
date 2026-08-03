# billing 计费规则（M3）

包：`internal/billing`（service / repository / model + gorm/）

## 职责

抵扣顺序配置与抵扣计算。给定订单金额与可用券/余额，输出逐项抵扣明细。**纯计算模块**——是全系统最易变、最需要测的部分。

## 依赖

- **纯计算已提炼至工具库**：`quanttide-pay-toolkit/packages/go/pkg/billing`（`Calculate`/类型/错误契约），本包经类型别名转发，`order` 调用方零改动
- 规则存储（`BillingRule` gorm 模型）留在服务端

## 模型

```go
type BillingRule struct {
    ID        int64
    Priority  int    // 执行顺序
    Kind      string // coupon / voucher / balance
    Condition string // JSON 条件（scope、min_amount 等）
}
```

## 核心流程

### 结算计算（纯函数，见工具库 `pkg/billing`）

```go
type Deduction struct {
    Kind   string // coupon / voucher / balance
    RefID  int64  // 券 ID（balance 时为 0）
    Amount int64  // 抵扣额（分）
}

func Calculate(amount int64, coupons []Coupon, vouchers []Voucher,
    balance int64) ([]Deduction, error)
```

v0.1.0 默认顺序（全部由 BillingRule.priority 表达，不改代码可调）：

1. 满减券：满足门槛（≤ 剩余应付）中力度最大的一张
2. 折扣券：按 rate 优惠（9 折 = rate 90 = 省 10%），`应付 × (100 − rate) / 100` 向下取整
3. 代金券：逐张抵扣 `min(面值, 剩余应付)`
4. 余额：补足剩余

边界：余额不足 → 返回错误（结算拒绝）；无任何可用券 → 全额余额。

## API

无独立端点，供 `order` 结算调用。

## 测试

表驱动覆盖组合：无券 / 仅满减 / 仅折扣 / 满减+折扣 / 代金券 / 混合 / 余额不足——全系统最需要测的纯逻辑。
