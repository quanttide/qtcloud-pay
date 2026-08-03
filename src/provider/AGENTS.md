# AGENTS（qtcloud-pay · src/provider）

面向在 provider（Go 模块）内工作的编码 agent 的指令。**动手前先读「关键文件」**；上级纪律见仓库根 [AGENTS.md](../../../AGENTS.md) 与 [CONTRIBUTING.md](../../../CONTRIBUTING.md)。

## 本 scope 是什么

qtcloud-pay 支付云服务端（Go）：账本核心（账户/交易/优惠券/代金券/计费/订单/对账）+ 支付渠道（微信/支付宝）。`internal/` 下按业务域分模块（scope），纯逻辑契约已提炼至工具库，本模块只留实体、gorm 存储、事务编排与渠道适配。

## 关键文件（按优先级阅读）

| 文件 | 作用 | 何时必读 |
|------|------|----------|
| `README.md` | 结构、接口、API、测试分层、部署 | 每次工作前 |
| `CONTRIBUTING.md` | 核心设计思路（6 条约束）、模块分层、新增模块流程 | 每次改代码前 |
| `ROADMAP.md` | **缺陷清单 F1–F8**、v0.2.0 T1–T5、待定决策 | 改代码前核对相关条目 |
| `docs/index.md` | 文档导航与模块总览 | 定位模块文档 |
| `docs/conventions.md` | 横切设计约束（账本写入唯一入口、同事务更新、金额整数分、存储双引擎、渠道独立演进、无后台任务） | 任何改动前 |
| `docs/<module>.md` | 各模块设计（account/transaction/coupon/voucher/billing/order/reconciliation/channel） | 改对应模块前 |
| `Makefile` | `make build/test/vet/lint/run/clean` | 构建测试时 |
| `CHANGELOG.md` | 版本变更记录 | 提交前核对 |
| `cmd/server/main.go` | 依赖组装、方言选择、AutoMigrate、渠道装配 | 新增模块/渠道时 |
| 工具库 `packages/go/pkg/*` | 契约实现（status/idempotency/billing/ledger/money/httpapi/middleware） | 涉及契约语义时 |

## 契约纪律（最高优先级）

1. **禁止端侧发明语义**：状态（`pkg/status`）、幂等键（`pkg/idempotency`）、计费（`pkg/billing`）、交易类型（`pkg/ledger`）一律引用工具库契约；本模块不定义等价常量/算法
2. **未知码必须报错**：渠道原始码解析遇未知值返回错误，不用 UNKNOWN 兜底
3. **金额整数分**：全链路 int64 分，禁止浮点；元/分转换只在 transport 边界（`money.Cents`）
4. **纯逻辑进工具库**：发现可提炼主干，先回工具库按 fixtures 流程（`tests/fixtures/` 先行）实施，再回来改引用

## 核心设计约束（docs/conventions.md 摘要）

- 账本写入**唯一入口** `transaction` 模块；余额、券状态与交易**同事务**更新
- 存储双引擎：SQLite（开发）/ PostgreSQL（生产），repository 只写一套 GORM，方言由 `cmd/server` 启动时选择
- `channel` 不依赖账本模块；v0.1.0 渠道不写 `order` 表（业务闭环推迟 v0.2.0 T5/F3）
- 无后台任务：券过期惰性流转，对账按需调用

## 已知状态（动手前核对 ROADMAP）

- **F1–F8 未开始**：F1 鉴权（生产阻塞）、F2 微信金额截断、F3 通知回调、F4 退款参数、F5 支付宝退款查询、F6 渠道 400、F7 证书序列号、F8 密钥落 tfstate
- v0.2.0：T1 商品目录、T2 计费规则 CRUD、T3 券策略参数化（等 P0 契约）、T4 结算报表、T5 回调自动入账

## 验证

```bash
go test ./... && go vet ./...   # 模块根运行；场景级集成测试在 internal/itest
```

提交前核对：ROADMAP 相关条目状态、CHANGELOG 是否需更新、测试通过。
