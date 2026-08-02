package channel

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
