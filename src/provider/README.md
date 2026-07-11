# provider

qtcloud-pay 支付提供商抽象层 — 定义统一的支付网关接口，支持支付宝、微信支付等多种支付渠道。

## 接口

```go
type Provider interface {
    Name() string
    Pay(req *PayRequest) (*PayResponse, error)
    Query(orderID string) (*OrderStatus, error)
    Refund(req *RefundRequest) (*RefundResponse, error)
}
```

| 方法 | 说明 |
|------|------|
| `Pay` | 发起支付，返回支付链接/交易号 |
| `Query` | 查询订单状态 |
| `Refund` | 申请退款 |

## 包结构

```
provider/
├── go.mod
├── README.md
├── ROADMAP.md
└── provider.go       ← Provider 接口定义
```

## 使用方式

```go
import "github.com/quanttide/qtcloud-pay/src/provider"

var p provider.Provider

// 发起支付
resp, err := p.Pay(&provider.PayRequest{
    OrderID:   "20250711001",
    Amount:    99.99,
    Subject:   "商品名称",
    NotifyURL: "https://example.com/notify",
})

// 查询订单
status, err := p.Query("20250711001")

// 申请退款
refundResp, err := p.Refund(&provider.RefundRequest{
    OrderID:      "20250711001",
    RefundAmount: 99.99,
    Reason:       "用户退款",
})
```
