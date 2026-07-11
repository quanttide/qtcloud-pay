# provider

qtcloud-pay 支付提供商抽象层，卖课场景支付接入。

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
| 微信支付 JSAPI | `wechat` | 公众号/小程序卖课 |
| 支付宝网页支付 | `alipay` | PC 端卖课 |

两个实现都分别暴露了原生类型的方法（如 `wechat.Client.JSAPIPay`），也提供了 `Provider` 接口的适配器（`provider.NewWechatPay` / `provider.NewAlipayPay`）。

## 结构

```
provider/
├── provider.go       ← 接口 + 数据结构
├── adapters.go       ← Provider 接口适配器
├── wechat/
│   └── wechat.go     ← 微信支付 JSAPI 实现
├── alipay/
│   └── alipay.go     ← 支付宝网页支付实现
├── go.mod
├── README.md
└── ROADMAP.md
```

## 使用

### 直接使用原生客户端

```go
import "github.com/quanttide/qtcloud-pay/src/provider/wechat"

client, _ := wechat.New(&wechat.Config{
    AppID:     "wx...",
    MchID:     "商户号",
    APIv3Key:  "API v3 密钥",
    MchKey:    `-----BEGIN PRIVATE KEY-----...`,
    MchCert:   `-----BEGIN CERTIFICATE-----...`,
    NotifyURL: "https://example.com/wechat/notify",
})

// JSAPI 下单
resp, _ := client.JSAPIPay(ctx, &wechat.JSAPIPayRequest{
    OpenID:      "o...",
    OutTradeNo:  "ORD20250711001",
    Total:       9999,       // 分
    Description: "Python 入门课",
})
// resp 包含 appId/timeStamp/nonceStr/package/paySign，直接给前端
```

```go
import "github.com/quanttide/qtcloud-pay/src/provider/alipay"

client, _ := alipay.New(&alipay.Config{
    AppID:      "2021...",
    PrivateKey: `-----BEGIN PRIVATE KEY-----...`,
    PublicKey:  `-----BEGIN PUBLIC KEY-----...`, // 支付宝公钥，用于验签
    NotifyURL:  "https://example.com/alipay/notify",
    ReturnURL:  "https://example.com/order/complete",
})

// PC 网页支付，返回 HTML 表单
html, _ := client.PagePay(&alipay.PagePayRequest{
    OutTradeNo:  "ORD20250711001",
    TotalAmount: 99.99,
    Subject:     "Python 入门课",
})
// html 直接响应给浏览器，自动跳转支付宝
```

### 使用 Provider 接口

```go
import "github.com/quanttide/qtcloud-pay/src/provider"

p, _ := provider.NewWechatPay(&wechat.Config{...})
resp, _ := p.Pay(ctx, &provider.PayRequest{
    OrderID:  "ORD20250711001",
    Amount:   99.99,
    Subject:  "Python 入门课",
    Metadata: map[string]any{"openid": "o..."},
})
```

## 已实现能力

| 功能 | 微信 JSAPI | 支付宝网页支付 |
|------|-----------|---------------|
| 下单 | `JSAPIPay` → prepay_id + 前端调起参数 | `PagePay` → HTML 表单 / `WapPay` → HTML 表单 |
| 查询 | `QueryOrder` / `QueryOrderByOutTradeNo` | `QueryOrder` |
| 退款 | `Refund` | `Refund` |
| 通知解析 | `ParseNotify`（AES-GCM 解密 + 验签） | `VerifyNotify`（RSA2 验签） |
