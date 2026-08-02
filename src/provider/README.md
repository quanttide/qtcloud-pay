# provider

qtcloud-pay 支付提供商服务，卖课场景支付接入。

按标准 Go 项目布局组织：`cmd/` 为入口，`internal/` 为私有应用和库代码（外部不可导入）。

## 接口

```go
type Provider interface {
    Name() string
    Pay(ctx context.Context, req *PayRequest) (*PayResponse, error)
    Query(ctx context.Context, orderID string) (*OrderStatus, error)
    Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
}
```

## 实现

| 渠道 | 包 | 场景 |
|------|-----|------|
| 微信支付 JSAPI | `internal/channel/wechat` | 公众号/小程序卖课 |
| 支付宝网页支付 | `internal/channel/alipay` | PC 端卖课 |

两个实现都分别暴露了原生类型的方法（如 `wechat.Client.JSAPIPay`），也提供了 `Provider` 接口的适配器（`channel.NewWechatPay` / `channel.NewAlipayPay`）。

## 结构

```
provider/
├── cmd/
│   └── server/
│       └── main.go              ← 入口：加载配置，组装依赖，启动服务
├── internal/                    ← 私有应用和库代码，外部不可导入
│   ├── account/                 ← 账户与余额（充值登记、余额查询）
│   ├── transaction/             ← 交易账本（账本写入唯一入口、流水）
│   ├── coupon/                  ← 优惠券（折扣/满减，发放与核销）
│   ├── voucher/                 ← 代金券（面值抵现，发放与抵现）
│   ├── billing/                 ← 计费规则（抵扣顺序与计算）
│   ├── order/                   ← 订单与结算（单事务编排）
│   ├── reconciliation/          ← 对账与可查（一致性校验、账单）
│   ├── channel/                 ← 支付渠道模块
│   │   ├── transport.go         ← HTTP handler（参数绑定、协议转换）
│   │   ├── service.go           ← Provider 接口
│   │   ├── adapters.go          ← 微信/支付宝适配器（实现 Provider 接口）
│   │   ├── model.go             ← 请求/响应模型（DTO）
│   │   ├── transport_test.go    ← API 测试
│   │   ├── service_test.go      ← 适配器测试（mock transport）
│   │   ├── wechat/              ← 微信支付渠道实现
│   │   │   ├── wechat.go
│   │   │   └── wechat_test.go
│   │   └── alipay/              ← 支付宝渠道实现
│   │       ├── alipay.go
│   │       └── alipay_test.go
│   └── middleware/              ← 内部中间件（请求日志）
│       └── logging.go
├── docs/                        ← 设计文档（总览 + 各模块实现）
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

各账本模块均按 `transport / service / repository / model + gorm/` 组织，repository 用 GORM 实现（开发 SQLite / 生产 PostgreSQL 方言切换），详见 [docs](docs/index.md)。

## 使用

### 运行服务

配置通过环境变量注入，渠道由 `-channel` 指定：

```sh
# 微信 JSAPI 渠道（公众号/小程序）
WECHAT_APP_ID=wx... WECHAT_MCH_ID=商户号 WECHAT_API_V3_KEY=... \
WECHAT_MCH_KEY="$(cat mch_private_key.pem)" WECHAT_MCH_CERT="$(cat mch_cert.pem)" \
WECHAT_NOTIFY_URL=https://example.com/wechat/notify \
go run ./cmd/server -addr :8080 -channel wechat
```

```sh
# 支付宝网页支付渠道（PC）
ALIPAY_APP_ID=2021... ALIPAY_PRIVATE_KEY="$(cat app_private_key.pem)" \
ALIPAY_PUBLIC_KEY="$(cat alipay_public_key.pem)" \
ALIPAY_NOTIFY_URL=https://example.com/alipay/notify \
ALIPAY_RETURN_URL=https://example.com/order/complete \
go run ./cmd/server -addr :8080 -channel alipay
```

或使用 Makefile：

```sh
make build && ./bin/provider-server -addr :8080 -channel wechat
```

### API

#### 账本核心（M1–M4）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/accounts` | 创建账户 |
| POST | `/accounts/{id}/recharges` | 充值登记（对公打款入账，凭证号幂等） |
| GET | `/accounts/{id}` | 账户与余额 |
| GET | `/accounts/{id}/transactions` | 交易流水 |
| POST | `/accounts/{id}/coupons` | 发放优惠券（批量，批次号幂等） |
| GET | `/accounts/{id}/coupons` | 查询优惠券 |
| POST | `/accounts/{id}/vouchers` | 发放代金券（批量，批次号幂等） |
| GET | `/accounts/{id}/vouchers` | 查询代金券 |
| POST | `/orders` | 下单并结算（订单号幂等） |
| GET | `/orders/{id}` | 订单与结算明细 |
| GET | `/accounts/{id}/statement` | 账单导出 |
| GET | `/reconcile/consistency` | 余额-交易一致性校验 |
| POST | `/reconcile/bank` | 对公打款核对（银行流水 CSV） |

#### 支付渠道

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/pay` | 发起支付，返回支付链接/前端调起参数 |
| GET | `/query/{order_id}` | 查询订单状态 |
| POST | `/refund` | 申请退款 |

金额单位：账本核心接口的 `amount` 均为整数分。

## 测试

三层测试，覆盖同一份测试设计（[tests.md](../../../../data/roadmap/tests.md) 的 TC-A/B/C/X）：

| 层 | 位置 | 说明 |
|----|------|------|
| 单元测试 | 各包 `*_test.go` | 服务逻辑 + GORM repository（SQLite `:memory:`），覆盖率 ≥95% |
| Go 集成测试 | `internal/itest/` | 真库 + 全模块真实组装，`httptest` 驱动 HTTP 链路 |
| Python 端到端 | `../../tests/` | **编译并启动真实二进制**，经 HTTP API 访问（`uv run pytest tests/`） |

运行：

```sh
# Go 单测 + 集成测试
make test

# Python 端到端（依赖 Go toolchain + uv）
cd ../../ && uv run pytest tests/
```

## 已实现能力

| 功能 | 微信 JSAPI | 支付宝网页支付 |
|------|-----------|---------------|
| 下单 | `JSAPIPay` → prepay_id + 前端调起参数 | `PagePay` → HTML 表单 / `WapPay` → HTML 表单 |
| 查询 | `QueryOrder` / `QueryOrderByOutTradeNo` | `QueryOrder` |
| 退款 | `Refund` | `Refund` |
| 通知解析 | `ParseNotify`（AES-GCM 解密 + 验签） | `VerifyNotify`（RSA2 验签） |
