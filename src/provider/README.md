# provider

qtcloud-pay 支付提供商抽象层，卖课场景支付接入。

## 接口

```go
type Provider interface {
    Name() string
    Pay(req *PayRequest) (*PayResponse, error)
    Query(orderID string) (*OrderStatus, error)
    Refund(req *RefundRequest) (*RefundResponse, error)
}
```

## 当前重点

| 渠道 | 场景 |
|------|------|
| 微信支付 JSAPI | 公众号/小程序卖课 |
| 支付宝网页支付 | PC 端卖课 |

## 结构

```
provider/
├── provider.go    ← 接口定义
├── go.mod
├── README.md
└── ROADMAP.md     ← 卖课场景路线图
```
