# 开发运维指南 · qtcloud-pay

## 本地开发

### 环境要求

- Go >= 1.26
- Python >= 3.12（运行集成测试）

### 项目结构

```
qtcloud-pay/
├── src/provider/          ← Go 支付提供者
│   ├── cmd/
│   │   └── server/
│   │       └── main.go    ← 入口：加载配置，组装依赖，启动服务
│   ├── internal/          ← 私有应用和库代码，外部不可导入
│   │   ├── channel/       ← 支付渠道模块
│   │   │   ├── transport.go   ← HTTP 端点
│   │   │   ├── service.go     ← Provider 接口
│   │   │   ├── adapters.go    ← 微信/支付宝适配器（实现 Provider 接口）
│   │   │   ├── model.go       ← 请求/响应模型（DTO）
│   │   │   ├── wechat/        ← 微信支付实现（gopay 封装）
│   │   │   ├── alipay/        ← 支付宝支付实现（gopay 封装）
│   │   │   └── *_test.go      ← 单元测试
│   │   └── middleware/        ← 内部中间件（请求日志）
│   │       └── logging.go
│   └── Makefile
├── tests/                 ← Python 集成测试
│   └── test_api.py        ← 调用 Go 测试的封装
├── docs/                  ← 文档
├── pyproject.toml
└── README.md
```

### 运行测试

#### Go 单元测试

```bash
cd src/provider
go test ./... -v
```

#### 仅运行 API 端点测试

```bash
go test -run TestAPI_ ./... -v
```

预期输出：13 个 PASS（3 端点 × 4-5 场景）。

#### Python 封装测试

```bash
# 项目根目录
uv sync --dev
uv run pytest tests/ -v
```

该测试调用 `go test -run TestAPI_` 并断言 ≥13 个测试通过。

### 添加新支付提供商

1. 在 `internal/channel/` 下创建子包（如 `unionpay/`）
2. 实现子包的 Client，提供支付/查询/退款方法
3. 在 `adapters.go` 中实现 `Provider` 接口
4. 在 `service_test.go` 中添加适配器测试（mock transport + mock server）
5. 在 `cmd/server/main.go` 的 `newProvider` 中添加渠道分支与环境变量
6. 运行 `go test ./...` 确认全部通过

## Provider 接口

所有支付提供商适配器需实现以下接口：

```go
type Provider interface {
    Name() string
    Pay(ctx context.Context, req *PayRequest) (*PayResponse, error)
    Query(ctx context.Context, orderID string) (*OrderStatus, error)
    Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
}
```

## 版本管理

### 版本策略

| 范围 | 标签格式 | 示例 | 说明 |
|------|----------|------|------|
| 根项目 | `vX.Y.Z` | `v0.0.1` | 应用整体版本，记录 `pyproject.toml` |
| provider | `provider/vX.Y.Z` | `provider/v0.0.1` | Go 模块版本，仅记录 `CHANGELOG` |

### Go 模块版本

Go 模块的版本不由 `go.mod` 文件声明，而是由 **git tag** 决定。`go.mod` 只声明模块路径：

```go
module github.com/quanttide/qtcloud-pay/src/provider
```

模块代码位于 `internal/`（外部不可导入），仅作为应用交付；tag 仍用于记录版本与发布。

### 发布流程

使用 `qtcloud-devops release` 命令：

```bash
# 发布 provider 模块
qtcloud-devops release publish --version provider/v0.0.1 -y

# 发布根项目
qtcloud-devops release publish --version v0.0.1 -y
```

该命令自动处理：
- CHANGELOG 追加条目
- `pyproject.toml` 版本号更新（仅根项目）
- 创建并推送 git tag
- 创建 GitHub Release

## 配置

配置通过环境变量注入，由 `cmd/server/main.go` 的 `newProvider` 加载：

| 渠道 | 环境变量 | 用途 |
|------|----------|------|
| `wechat` | WECHAT_APP_ID, WECHAT_MCH_ID, WECHAT_API_V3_KEY, WECHAT_MCH_CERT, WECHAT_MCH_KEY, WECHAT_NOTIFY_URL | 微信商户平台配置 |
| `alipay` | ALIPAY_APP_ID, ALIPAY_PRIVATE_KEY, ALIPAY_PUBLIC_KEY, ALIPAY_NOTIFY_URL, ALIPAY_RETURN_URL | 支付宝应用配置 |

## 测试策略

| 层级 | 技术 | 覆盖范围 |
|------|------|----------|
| 子包单元测试 | mockTransport 拦截 HTTP | alipay/wechat 各 API 方法的正常/异常/传输错误 |
| 适配器测试 | mockTransport + httptest 服务器 | 适配器数据格式转换、金额单位换算、错误传递 |
| API 集成测试 | apiMockProvider + httptest | 3 端点 × 正常/客户端错误/服务端错误/错误方法 |
| Python 封装 | `subprocess` 调用 `go test` | 验证 Go API 测试全部通过 |

子包测试使用 `gclient.GetHttpClient().SetTransport(mockTransport)` 拦截实际出网请求；
适配器测试通过 `providerTransport` 将请求重定向到 `httptest.NewServer` 的本地 mock。

## 部署

### 构建

```bash
cd src/provider
make build
```

### 启动服务

```bash
./bin/provider-server -addr :8080 -channel wechat
```

服务默认监听 `:8080`，渠道由 `-channel` 指定（wechat/alipay），配置从环境变量读取。
