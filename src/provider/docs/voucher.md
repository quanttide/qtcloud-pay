# voucher 代金券（M2）

包：`internal/voucher`（transport / service / repository / model + gorm/）

## 职责

面值抵现券的发放（幂等）、过期流转、结算时抵现。代金券本身**就是钱**，结算时直接抵减应付款项。

新增 `PricingRuleSet` 用于录入实训基地等外部计价事实（发行渠道、核销定价、开放问题）。规则集只作为配置快照落库和管理，不改变 v0.1.0 既有发券/结算执行路径。

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

计价规则集：

```go
type PricingRuleSet struct {
    ID      string // 如 qtclass-voucher-pricing
    Source  string // 事实来源
    Version string // 规则版本或更新时间
    Payload string // 原始机器 JSON，金额字段为 *_cents
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
| PUT | `/admin/voucher-pricing-rules/{id}` | 幂等录入/更新计价规则集（需 `X-Admin-Token`） |
| GET | `/admin/voucher-pricing-rules/{id}` | 查询计价规则集（需 `X-Admin-Token`） |
| GET | `/admin/voucher-pricing-rules` | 查询规则集列表（需 `X-Admin-Token`） |

规则集校验：

- 固定发行渠道 `voucher.amount_cents` 必须为正整数分，`scope` 必须为现有代金券范围（`all/cloud/course/data/product`）
- 追加奖励渠道使用 `bonus_type` 记录首次完成、产出评定、群内互动三类触发；动态等额或评定制金额写入 `voucher.amount_rule`，面额约束写入 `bonus_denomination_rule`
- 一对一咨询保留“服务者职级档位”维度，按 `rank_prices_cents` 录入
- 超额申请额度保留“流程配额”维度，按 `free_limit` + `exceed_price_cents` 录入
- `billing_semantics.voucher_is_money` 必须为 true；开放问题随 payload 保留，不阻塞上线

## 测试

面值 ≥ 应付 / 面值 < 应付两分支、幂等。
