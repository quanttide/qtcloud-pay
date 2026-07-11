package provider

import (
	"context"
	"fmt"

	"github.com/quanttide/qtcloud-pay/src/provider/alipay"
	"github.com/quanttide/qtcloud-pay/src/provider/wechat"
)

// WechatPay 微信支付适配器
type WechatPay struct {
	client *wechat.Client
}

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
	return &OrderStatus{
		TradeID:  resp.TransactionID,
		OrderID:  resp.OutTradeNo,
		Status:   resp.TradeState,
		Amount:   float64(resp.Total) / 100,
		PaidAt:   resp.SuccessTime,
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
	return &RefundResponse{
		RefundID: resp.RefundID,
		Status:   resp.Status,
	}, nil
}

// AlipayPay 支付宝适配器
type AlipayPay struct {
	client *alipay.Client
}

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
	tradeStatusMap := map[string]string{
		"WAIT_BUYER_PAY": "PENDING",
		"TRADE_SUCCESS":  "SUCCESS",
		"TRADE_FINISHED": "SUCCESS",
		"TRADE_CLOSED":   "CLOSED",
	}
	status := tradeStatusMap[resp.TradeStatus]
	if status == "" {
		status = "UNKNOWN"
	}
	return &OrderStatus{
		TradeID:  resp.TradeNo,
		OrderID:  resp.OutTradeNo,
		Status:   status,
		Amount:   parseAmount(resp.TotalAmount),
		PaidAt:   resp.PayTime,
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
	_ = resp
	return &RefundResponse{
		RefundID: req.OrderID,
		Status:   "SUCCESS",
	}, nil
}

func parseAmount(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// Ensure adapters implement Provider
var _ Provider = (*WechatPay)(nil)
var _ Provider = (*AlipayPay)(nil)
