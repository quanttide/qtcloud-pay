# 设计约束与实现约定

适用于全部模块的横切约定。各模块文档见[文档导航](index.md)。

## 关键设计约束

1. **账本写入唯一入口**：所有账本变更（充值/发券/消费/核销）必须经 `transaction` 模块写入，配合幂等键与唯一约束，保证不丢、不重、可查
2. **同事务更新**：余额、券状态与交易在同一数据库事务内更新（不错）；各模块 repository 共享同一 `*gorm.DB` 连接，跨模块写（结算）由 `order` 服务在单个事务内协调
3. **金额整数分**：全链路整数分存储，不做浮点金额
4. **渠道独立演进**：`channel` 不依赖账本模块，账本跑通前不扩展渠道能力
5. **存储双引擎**：开发环境 SQLite、生产环境 PostgreSQL，由 GORM（类似 SQLAlchemy 的 ORM）统一调度——repository 只写一套 GORM 实现，方言（sqlite/postgres）由 `cmd/server` 按配置（`DB_DRIVER` / `DATABASE_URL`）在启动时选择；迁移用 GORM AutoMigrate，生产环境后续引入版本化迁移

## 通用实现约定

### 存储初始化与方言切换

`cmd/server/main.go` 按配置选择 GORM 方言，启动时 AutoMigrate 注册全部模型：

```go
var db *gorm.DB
switch os.Getenv("DB_DRIVER") {
case "postgres":
    db, err = gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")))
default: // sqlite（开发默认）
    db, err = gorm.Open(sqlite.Open("qtcloud.db"))
}
db.AutoMigrate(
    &account.Account{}, &transaction.Transaction{},
    &coupon.Coupon{}, &voucher.Voucher{},
    &order.Order{}, &billing.BillingRule{},
)
```

### 事务传递约定

- repository 接口方法统一以 `*gorm.DB` 为首参——`*gorm.Tx` 内嵌 `*gorm.DB`，事务与共享连接可直接互换
- 跨模块写账本的方法（充值、发券、结算）在单个数据库事务内完成，事务由编排方开启：
  - 充值：`account.service` 开事务（锁账户 → 写充值交易）
  - 结算：`order.service` 开事务（核销券 → 扣余额 → 写消费/核销交易）
- 行锁：`GetForUpdate` 用 GORM `clause.Locking`——PostgreSQL 生成 `FOR UPDATE`；SQLite 单写者、写事务串行，无需行锁

### 幂等键约定

| 场景 | 幂等键 | 唯一约束位置 |
|------|--------|--------------|
| 充值 | 打款凭证号 `voucher_no` | `transaction.idempotency_key`（`recharge:{voucher_no}`） |
| 发券 | 发放批次号 `batch_no` | 发券交易幂等键 `transaction.idempotency_key`（`issue:coupon:{batch_no}` / `issue:voucher:{batch_no}`）；`batch_no` 本身为普通索引，作防御性检查 |
| 结算 | 商户订单号 `order_id` | `order.id`；账本消费/核销交易 `settle:{order_id}[:redeem:{kind}:{ref}]` |

说明：发券批次号按类型命名空间区分（优惠券与代金券各自自增，批次号可能相同），幂等由发券交易的全局唯一幂等键保证；`batch_no` 不做唯一约束，因为同一批次含多张券。

### 金额与折扣计算

- 全链路金额 int64（分），无浮点
- 折扣券按整数百分比：`折扣 = 应付 × rate / 100`（向下取整）
- 满减券：应付 ≥ threshold 时减 amount
- 代金券：抵扣 `min(面值, 剩余应付)`
