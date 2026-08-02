# 贡献指南 · qtcloud-pay provider

qtcloud-pay 支付提供商服务（`src/provider`，Go 模块）的贡献指南。v0.1.0 为账本核心：在现有 `channel` 渠道模块旁新增账本各模块，`cmd/server` 统一组装。

设计文档导航见 [docs/index.md](docs/index.md)，横切设计约束与实现约定的完整版见 [docs/conventions.md](docs/conventions.md)。

## 核心设计思路

贡献者先理解设计意图，再写代码——以下约束是所有模块的公约数，任何改动不得破坏它们。

### 1. 账本写入唯一入口（不丢、不重、可查）

所有账本变更（充值/发券/消费/核销）**必须经 `transaction` 模块写入**，配合幂等键与唯一约束：

| 场景 | 幂等键 | 唯一约束位置 |
|------|--------|--------------|
| 充值 | 打款凭证号 `voucher_no` | `transaction.idempotency_key` |
| 发券 | 发放批次号 `batch_no` | `coupon.batch_no` / `voucher.batch_no` |
| 结算 | 商户订单号 `order_id` | `order.id` |

交易是不可变记录（账本）；余额、券状态都是交易的投影。**模型的核心不是钱，而是交易事实**——支付通道只是交易的一个来源（v0.2.0 回调），不改变账本本身。

### 2. 同事务更新（不错）

余额、券状态与交易在**同一数据库事务**内更新。事务由编排方开启：

- 充值：`account.service` 开事务（锁账户 → 写充值交易）
- 结算：`order.service` 开事务（核销券 → 扣余额 → 写消费/核销交易）

各模块 repository 共享同一 `*gorm.DB` 连接；行锁用 GORM `clause.Locking`——PostgreSQL 生成 `FOR UPDATE`，SQLite 单写者、写事务串行，无需行锁。

### 3. 金额整数分

全链路金额 int64 分存储，**不做浮点金额**。折扣券按整数百分比向下取整（`应付 × rate / 100`），满减券过门槛减额，代金券抵扣 `min(面值, 剩余应付)`。

### 4. 存储双引擎

开发环境 SQLite、生产环境 PostgreSQL，由 GORM 方言切换（`DB_DRIVER` / `DATABASE_URL`）统一调度。**repository 只写一套 GORM 实现**，方言差异由 `cmd/server` 启动时选择；迁移用 GORM AutoMigrate。

### 5. 渠道独立演进

`channel` 不依赖账本模块，账本跑通前不扩展渠道能力。支付对接只改变交易的产生方式（手动登记 → 系统规则 → 支付回调），不改变账本——这就是「先建模拟账户再对接支付」成立的原因。

### 6. 模块分层

`transaction` 是最底层，被所有写账本模块依赖；`billing` 是纯计算模块（给定金额与可用券/余额，输出抵扣明细），不依赖存储；`order` 依赖最多，是结算的编排者：应用计费规则 → 写消费/核销交易 → 更新余额与券状态。

## 项目结构

```
src/provider/
├── cmd/
│   └── server/
│       └── main.go              ← 组装依赖，启动服务
├── internal/
│   ├── account/                 ← 账户与余额（充值登记、余额查询）
│   │   ├── transport.go         ← HTTP handler（参数绑定、协议转换）
│   │   ├── service.go           ← 业务逻辑接口 + 实现
│   │   ├── repository.go        ← 存储接口定义
│   │   ├── model.go             ← 领域模型、DTO
│   │   ├── service_test.go      ← 单元测试（mock repository）
│   │   └── gorm/                ← repository 的 GORM 实现（方言切换）
│   ├── transaction/             ← 交易账本（账本写入唯一入口；结构同 account）
│   ├── coupon/                  ← 优惠券（结构同 account）
│   ├── voucher/                 ← 代金券（结构同 account）
│   ├── order/                   ← 订单与结算（结构同 account）
│   ├── billing/                 ← 计费规则（纯计算，无存储依赖）
│   ├── reconciliation/          ← 对账与可查
│   ├── channel/                 ← 支付渠道（现有，保持独立）
│   │   ├── transport.go         ← HTTP 端点
│   │   ├── service.go           ← Provider 接口
│   │   ├── adapters.go          ← 微信/支付宝适配器（实现 Provider 接口）
│   │   ├── model.go             ← 请求/响应模型（DTO）
│   │   ├── wechat/              ← 微信支付实现（gopay 封装）
│   │   └── alipay/              ← 支付宝支付实现（gopay 封装）
│   └── middleware/              ← 请求日志
│       └── logging.go
├── docs/                        ← 设计文档（每模块一篇，见 index.md 导航）
├── go.mod
├── go.sum
├── Makefile
├── CHANGELOG.md
└── README.md
```

## 配置

配置通过环境变量注入，由 `cmd/server/main.go` 加载：

| 变量 | 用途 |
|------|------|
| `DB_DRIVER` / `DATABASE_URL` | 存储方言：开发默认 SQLite，生产 `postgres` + 连接串 |
| `WECHAT_*`（APP_ID, MCH_ID, API_V3_KEY, MCH_CERT, MCH_KEY, NOTIFY_URL） | 微信商户平台配置 |
| `ALIPAY_*`（APP_ID, PRIVATE_KEY, PUBLIC_KEY, NOTIFY_URL, RETURN_URL） | 支付宝应用配置 |

## 运行测试

```bash
# Go 单元测试（模块内）
go test ./... -v

# 仅 API 端点测试
go test -run TestAPI_ ./... -v

# Python 封装测试（qtcloud-pay 仓库根目录，断言 ≥13 个 Go API 测试通过）
uv sync --dev
uv run pytest tests/ -v
```

## 测试策略

| 层级 | 技术 | 覆盖范围 |
|------|------|----------|
| 子包单元测试 | mockTransport 拦截 HTTP | alipay/wechat 各 API 方法的正常/异常/传输错误 |
| 适配器测试 | mockTransport + httptest 服务器 | 适配器数据格式转换、金额单位换算、错误传递 |
| API 集成测试 | apiMockProvider + httptest | 3 端点 × 正常/客户端错误/服务端错误/错误方法 |
| 场景级集成测试 | `internal/itest`：SQLite `:memory:` + 全模块组装 | 账本旅程端到端（见下） |
| Python 封装 | `subprocess` 调用 `go test` | 验证 Go API 测试全部通过 |

`internal/itest` 按 tests.md §测试环境：SQLite `:memory:` 真库 + AutoMigrate 全部模型（对齐 `cmd/server` 的依赖注入组装），以真实 HTTP API 调用序列驱动端到端链路；不 mock、不单独测纯函数，**每个旅程结束都核对「余额 = 交易按方向求和」「券状态与订单明细逐笔对得上」**。存放于 `internal/` 下独立包，由 `go test ./...` 统一运行。

## 新增账本模块

新增功能 = 新增模块：

1. 在 `internal/` 下创建模块目录，按既有骨架组织：`transport.go` / `service.go` / `repository.go` / `model.go` + `gorm/`（纯计算模块如 `billing` 无存储，可省略 repository/gorm）
2. 编写 `docs/<module>.md`，并登记到 `docs/index.md` 的「文档导航」与「模块总览」表
3. 按依赖关系图接线——**账本写入一律经 `transaction` 模块**，同事务更新余额与券状态
4. 补充测试：单元测试（mock repository）+ 场景级集成测试（补入 `internal/itest` 旅程）
5. 在 `cmd/server/main.go` 组装 service 与 handler，运行 `go test ./...` 确认全部通过

扩展支付渠道（`channel`）遵循同一骨架：子包实现 Client（支付/查询/退款）→ `adapters.go` 实现 `Provider` 接口 → `service_test.go` 补适配器测试 → `main.go` 的 `newProvider` 加渠道分支与环境变量。

## 版本管理

Go 模块版本由 **git tag** 决定（`go.mod` 只声明模块路径），标签格式 `provider/vX.Y.Z`，仅记录 `CHANGELOG.md`。发布：

```bash
qtcloud-devops release publish --version provider/v0.0.1 -y
```

该命令自动处理 CHANGELOG 追加、创建并推送 git tag、创建 GitHub Release。仓库根项目版本（`vX.Y.Z`，记录 `pyproject.toml`）由仓库根目录管理。

## 部署

```bash
make build
./bin/provider-server -addr :8080 -channel wechat
```

服务默认监听 `:8080`，渠道由 `-channel` 指定（wechat/alipay），配置从环境变量读取。
