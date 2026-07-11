# 开发运维指南 · qtcloud-pay

## 本地开发

### 环境要求

- Go >= 1.26
- Python >= 3.12（运行集成测试）

### 项目结构

```
qtcloud-pay/
├── src/provider/          ← Go 支付提供者
│   ├── alipay/            ← 支付宝支付实现（gopay 封装）
│   ├── wechat/            ← 微信支付实现（gopay 封装）
│   ├── api.go             ← HTTP 端点（handler + middleware）
│   ├── api_test.go        ← API 集成测试（13 个用例）
│   ├── provider.go        ← Provider 接口 + 数据模型
│   ├── adapters.go        ← 微信/支付宝适配器（实现 Provider 接口）
│   ├── provider_test.go   ← 适配器测试（mock transport）
│   └── go.mod
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

1. 在 `src/provider/` 下创建子包（如 `unionpay/`）
2. 实现子包的 Client，提供支付/查询/退款方法
3. 在 `adapters.go` 中实现 `Provider` 接口
4. 在 `provider_test.go` 中添加适配器测试（mock transport + mock server）
5. 运行 `go test ./...` 确认全部通过

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

## 配置

当前硬编码配置见各子包 Config 结构体：

| 子包 | Config 字段 | 用途 |
|------|-------------|------|
| `alipay` | AppID, PrivateKey, PublicKey, NotifyURL, ReturnURL | 支付宝应用配置 |
| `wechat` | AppID, MchID, APIv3Key, MchCert, MchKey, NotifyURL | 微信商户平台配置 |

适配器构造时传入 Config，后续可通过环境变量或配置文件注入（待实现）。

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
go build -o qtcloud-pay .
```

### 启动服务

```bash
./qtcloud-pay
```

服务默认监听 `:8080`（当前 `main.go` 尚在实现中，`Server.Start()` 已就绪）。
