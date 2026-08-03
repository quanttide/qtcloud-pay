# 设计约束与实现约定

适用于全部模块的横切约定。各模块文档见[文档导航](index.md)。

## 关键设计约束

1. **账本写入唯一入口**：所有账本变更（充值/发券/消费/核销）必须经 `transaction` 模块写入，配合幂等键与唯一约束，保证不丢、不重、可查
2. **同事务更新**：余额、券状态与交易在同一数据库事务内更新（不错）；各模块 repository 共享同一 `*gorm.DB` 连接，跨模块写（结算）由 `order` 服务在单个事务内协调
3. **金额整数分**：全链路整数分存储，不做浮点金额
4. **渠道独立演进**：`channel` 不依赖账本模块，账本跑通前不扩展渠道能力
5. **存储双引擎**：开发环境 SQLite、生产环境 PostgreSQL，由 GORM（类似 SQLAlchemy 的 ORM）统一调度——repository 只写一套 GORM 实现，方言（sqlite/postgres）由 `cmd/server` 按配置（`DB_DRIVER` / `DATABASE_URL`）在启动时选择；迁移用 GORM AutoMigrate，生产环境后续引入版本化迁移
6. **渠道与账本刻意平行（v0.1.0 MVP 边界）**：`channel` 不写 `order` 表、支付成功不入账；外部支付只是旁挂的可选模块（`-channel` flag 默认为空）。生产 FC 部署目前即纯账本 API（terraform 无渠道环境变量）。业务闭环（支付→入账）明确推迟到 v0.2.0（ROADMAP T5/F3）。v0.1.0 的「支付」语义限定为**余额/券抵扣**（`order.Settle`）
7. **配置全走环境变量、零配置文件、零启动校验**：对齐 FC 环境变量注入（`DB_DRIVER`/`DATABASE_URL`/渠道密钥等）；账本模块无配置依赖，渠道缺配置在 `NewProvider` 时即报错，不会带病启动
8. **无后台任务**：券过期用**惰性流转**（读取时更新状态），不做定时任务；对账为按需调用，无调度器

## 通用实现约定

### 存储初始化与方言切换

`cmd/server/main.go` 按配置选择 GORM 方言，启动时 AutoMigrate 注册全部模型：

```go
var db *gorm.DB
switch os.Getenv("DB_DRIVER") {
case "postgres":
    db, err = gorm.Open(postgres.Open(os.Getenv("DATABASE_URL")))
default: // sqlite（开发默认）
    db, err = gorm.Open(sqlite.Open("qtcloud-pay.db"))
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
| 退款（多退） | 退款凭证号 `voucher_no` | `transaction.idempotency_key`（`refund:{voucher_no}`） |
| 发券 | 发放批次号 `batch_no` | 发券交易幂等键 `transaction.idempotency_key`（`issue:coupon:{batch_no}` / `issue:voucher:{batch_no}`）；`batch_no` 本身为普通索引，作防御性检查 |
| 结算 | 商户订单号 `order_id` | `order.id`；账本消费/核销交易 `settle:{order_id}[:redeem:{kind}:{ref}]` |

说明：发券批次号按类型命名空间区分（优惠券与代金券各自自增，批次号可能相同），幂等由发券交易的全局唯一幂等键保证；`batch_no` 不做唯一约束，因为同一批次含多张券。

幂等键命名规则：`{业务}:{业务号}` 前缀按业务类型隔离键空间（`recharge:`/`refund:`/`issue:{kind}:`/`settle:`），同一业务号在不同业务中互不冲突；键空间预留扩展，新业务接入时沿用。

**冲突回滚视为成功**：所有幂等写入（充值/退款/发券/结算）同构——事务内先查后插，唯一约束冲突时**整体回滚并返回成功**（`ErrDuplicateKey → nil`），调用方不再重试。这是「不丢、不重、可查」的落地语义：并发下后到者回滚自己的全部修改，让先到者生效。

### 并发正确性依赖

- **账户行锁 = 同账户全部结算的串行化点**：`order.Settle` 先锁账户行（`FOR UPDATE`），同账户的结算、充值、退款全部串行；券核销因此无需依赖券锁（同账户串行），`coupon/voucher.Use` 的 `GetForUpdate` 是双保险而非主依赖
- **READ COMMITTED 下的「锁后重读」依赖**：先无锁查（Exists/Get，快照）→ `FOR UPDATE` 重读 → 计算——拿锁后的余额/券状态才可信；锁前读到的一切只作存在性判断

### 金额与折扣计算

- 金额表示与 JSON 转换契约（全链路整数分、`pkg/money.Money`、严格校验）见工具库 [Money 使用指南](https://github.com/quanttide/quanttide-pay/blob/main/packages/quanttide-pay-toolkit/packages/go/docs/user-guide/money.md)；以下仅列 provider 特有约定
- 例外：`POST /reconcile/bank` 的银行流水 CSV 金额为分（`amount_cents`，财务工具格式）
- **已知例外（v0.1.0 遗留，见 ROADMAP F2）**：`internal/channel` 渠道层仍用 float64 元（`PayRequest.Amount` 等）且存在 `int(x*100)` 截断精度缺陷——渠道层与账本层金额表示不一致，**新代码一律以分传输，勿照抄渠道层写法**
- 折扣券按整数百分比：9 折 = rate 90 = 省 10%，`折扣 = 应付 × (100 − rate) / 100`（向下取整）
- 满减券：应付 ≥ threshold 时减 amount
- 代金券：抵扣 `min(面值, 剩余应付)`
