package provider

import "context"

// Provider 支付提供商接口
type Provider interface {
	// Name 返回提供商名称
	Name() string
	// Pay 发起支付
	Pay(ctx context.Context, req *PayRequest) (*PayResponse, error)
	// Query 查询订单
	Query(ctx context.Context, orderID string) (*OrderStatus, error)
	// Refund 申请退款
	Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error)
}

// PayRequest 支付请求
type PayRequest struct {
	OrderID    string            // 商户订单号
	Amount     float64           // 金额（元）
	Subject    string            // 商品标题
	NotifyURL  string            // 异步通知地址
	ReturnURL  string            // 同步跳转地址
	Metadata   map[string]any    // 额外参数（如微信 openid）
}

// PayResponse 支付响应
type PayResponse struct {
	TradeID     string // 支付平台交易号
	PayURL      string // 支付链接
	RawResponse any    // 原始响应
}

// OrderStatus 订单状态
type OrderStatus struct {
	TradeID     string  // 支付平台交易号
	OrderID     string  // 商户订单号
	Status      string  // 状态
	Amount      float64 // 实付金额
	PaidAt      string  // 支付时间
}

// RefundRequest 退款请求
type RefundRequest struct {
	OrderID     string  // 商户订单号
	TradeID     string  // 支付平台交易号
	RefundAmount float64 // 退款金额
	Reason      string  // 退款原因
}

// RefundResponse 退款响应
type RefundResponse struct {
	RefundID    string // 退款单号
	Status      string // 退款状态
}
