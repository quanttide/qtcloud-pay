package channel

import (
	"context"
	"strconv"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/alipay"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/channel/wechat"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/status"
)

// WechatPay 微信支付适配器。
type WechatPay struct {
	client *wechat.Client
}

// NewWechatPay 创建微信支付适配器。
func NewWechatPay(cfg *wechat.Config) (*WechatPay, error) {
	client, err := wechat.New(cfg)
	if err != nil {
		return nil, err
	}
	return &WechatPay{client: client}, nil
}

func (w *WechatPay) Name() string { return "wechat" }

func (w *WechatPay) Pay(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	openID, _ := req.Metadata["openid"].(string)
	resp, err := w.client.JSAPIPay(ctx, &wechat.JSAPIPayRequest{
		OpenID:      openID,
		OutTradeNo:  req.OrderID,
		Total:       int(req.Amount * 100), // 元转分
		Description: req.Subject,
	})
	if err != nil {
		return nil, err
	}
	return &PayResponse{
		// JSAPI 下单响应不含交易号，需通过 Query 或异步通知获取。
		TradeID:     "",
		PayURL:      "",
		RawResponse: resp,
	}, nil
}

func (w *WechatPay) Query(ctx context.Context, orderID string) (*OrderStatus, error) {
	resp, err := w.client.QueryOrderByOutTradeNo(ctx, orderID)
	if err != nil {
		return nil, err
	}
	parsed, err := status.ParseWechatTradeState(resp.TradeState)
	if err != nil {
		return nil, err
	}
	return &OrderStatus{
		TradeID: resp.TransactionID,
		OrderID: resp.OutTradeNo,
		Status:  parsed,
		Amount:  float64(resp.Total) / 100,
		PaidAt:  resp.SuccessTime,
	}, nil
}

func (w *WechatPay) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	refundAmt := int(req.RefundAmount * 100)
	resp, err := w.client.Refund(ctx, &wechat.RefundRequest{
		OutTradeNo:   req.OrderID,
		OutRefundNo:  req.OrderID + "-REFUND",
		RefundAmount: refundAmt,
		TotalAmount:  refundAmt,
		Reason:       req.Reason,
	})
	if err != nil {
		return nil, err
	}
	parsed, err := status.ParseWechatRefundStatus(resp.Status)
	if err != nil {
		return nil, err
	}
	return &RefundResponse{
		RefundID: resp.RefundID,
		Status:   parsed,
	}, nil
}

// AlipayPay 支付宝适配器。
type AlipayPay struct {
	client *alipay.Client
}

// NewAlipayPay 创建支付宝适配器。
func NewAlipayPay(cfg *alipay.Config) (*AlipayPay, error) {
	client, err := alipay.New(cfg)
	if err != nil {
		return nil, err
	}
	return &AlipayPay{client: client}, nil
}

func (a *AlipayPay) Name() string { return "alipay" }

func (a *AlipayPay) Pay(ctx context.Context, req *PayRequest) (*PayResponse, error) {
	html, err := a.client.PagePay(&alipay.PagePayRequest{
		OutTradeNo:  req.OrderID,
		TotalAmount: req.Amount,
		Subject:     req.Subject,
	})
	if err != nil {
		return nil, err
	}
	return &PayResponse{
		TradeID:     "",
		PayURL:      html,
		RawResponse: html,
	}, nil
}

func (a *AlipayPay) Query(ctx context.Context, orderID string) (*OrderStatus, error) {
	resp, err := a.client.QueryOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	parsed, err := status.ParseAlipayTradeStatus(resp.TradeStatus)
	if err != nil {
		return nil, err
	}
	return &OrderStatus{
		TradeID: resp.TradeNo,
		OrderID: resp.OutTradeNo,
		Status:  parsed,
		Amount:  parseAmount(resp.TotalAmount),
		PaidAt:  resp.PayTime,
	}, nil
}

func (a *AlipayPay) Refund(ctx context.Context, req *RefundRequest) (*RefundResponse, error) {
	resp, err := a.client.Refund(ctx, &alipay.RefundRequest{
		OutTradeNo:   req.OrderID,
		RefundAmount: req.RefundAmount,
		Reason:       req.Reason,
	})
	if err != nil {
		return nil, err
	}
	// TODO: 支付宝退款响应无独立退款单号，暂以商户订单号代替；成功后固定返回 succeeded。
	_ = resp
	return &RefundResponse{
		RefundID: req.OrderID,
		Status:   status.RefundStatusSucceeded,
	}, nil
}

// parseAmount 解析支付宝金额字符串（元），解析失败返回 0。
func parseAmount(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// 编译期断言适配器实现了 Provider 接口。
var _ Provider = (*WechatPay)(nil)
var _ Provider = (*AlipayPay)(nil)
