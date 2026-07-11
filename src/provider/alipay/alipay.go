// Package alipay 支付宝网页支付实现
// 适用于 PC 端卖课场景。
// 底层委托 github.com/go-pay/gopay/alipay。
package alipay

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-pay/gopay"
	gopayalipay "github.com/go-pay/gopay/alipay"
)

// Config 支付宝配置
type Config struct {
	AppID      string // 应用 ID
	PrivateKey string // 应用私钥 PEM
	PublicKey  string // 支付宝公钥 PEM（用于验签）
	NotifyURL  string // 异步通知地址
	ReturnURL  string // 同步跳转地址
}

// Client 支付宝客户端
type Client struct {
	cfg     *Config
	gclient *gopayalipay.Client
}

// SetTransport 设置 HTTP 传输层（用于测试）
func (c *Client) SetTransport(_ http.RoundTripper) {}

// SetAPIURL 设置 API 地址（用于测试）
func (c *Client) SetAPIURL(_ string) {}

// New 创建支付宝客户端
func New(cfg *Config) (*Client, error) {
	gclient, err := gopayalipay.NewClient(cfg.AppID, cfg.PrivateKey, true)
	if err != nil {
		return nil, fmt.Errorf("alipay: new gopay client: %w", err)
	}
	gclient.ReturnUrl = cfg.ReturnURL
		gclient.NotifyUrl = cfg.NotifyURL
	gclient.DebugSwitch = gopay.DebugOff
	if cfg.PublicKey != "" {
		gclient.AutoVerifySign([]byte(cfg.PublicKey))
	}
	return &Client{cfg: cfg, gclient: gclient}, nil
}

// PagePay 统一收单下单并支付页面（alipay.trade.page.pay）
// 返回自动提交的 HTML 表单。
func (c *Client) PagePay(req *PagePayRequest) (string, error) {
	bm := make(gopay.BodyMap)
	bm.Set("subject", req.Subject).
		Set("out_trade_no", req.OutTradeNo).
		Set("total_amount", fmt.Sprintf("%.2f", req.TotalAmount))
	if req.Body != "" {
		bm.Set("body", req.Body)
	}
	// gopay TradePagePay 内部会设置 product_code = "FAST_INSTANT_TRADE_PAY"
	payURL, err := c.gclient.TradePagePay(context.Background(), bm)
	if err != nil {
		return "", fmt.Errorf("alipay: page pay: %w", err)
	}
	return payURL, nil
}

// WapPay 手机网页支付（alipay.trade.wap.pay）
func (c *Client) WapPay(req *WapPayRequest) (string, error) {
	bm := make(gopay.BodyMap)
	bm.Set("subject", req.Subject).
		Set("out_trade_no", req.OutTradeNo).
		Set("total_amount", fmt.Sprintf("%.2f", req.TotalAmount)).
		Set("quit_url", req.QuitURL)
	// gopay TradeWapPay 内部会设置 product_code = "QUICK_WAP_WAY"
	payURL, err := c.gclient.TradeWapPay(context.Background(), bm)
	if err != nil {
		return "", fmt.Errorf("alipay: wap pay: %w", err)
	}
	return payURL, nil
}

// QueryOrder 交易查询（alipay.trade.query）
func (c *Client) QueryOrder(ctx context.Context, outTradeNo string) (*OrderResult, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", outTradeNo)

	rsp, err := c.gclient.TradeQuery(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("alipay: query order: %w", err)
	}
	return &OrderResult{
		OutTradeNo:  rsp.Response.OutTradeNo,
		TradeNo:     rsp.Response.TradeNo,
		TradeStatus: rsp.Response.TradeStatus,
		TotalAmount: rsp.Response.TotalAmount,
		PayTime:     rsp.Response.SendPayDate,
	}, nil
}

// Refund 交易退款（alipay.trade.refund）
func (c *Client) Refund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	bm := make(gopay.BodyMap)
	bm.Set("out_trade_no", req.OutTradeNo).
		Set("refund_amount", fmt.Sprintf("%.2f", req.RefundAmount))
	if req.Reason != "" {
		bm.Set("refund_reason", req.Reason)
	}
	if req.OutRequestNo != "" {
		bm.Set("out_request_no", req.OutRequestNo)
	}

	rsp, err := c.gclient.TradeRefund(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("alipay: refund: %w", err)
	}
	return &RefundResult{
		OutTradeNo: rsp.Response.OutTradeNo,
		TradeNo:    rsp.Response.TradeNo,
		Status:     rsp.Response.FundChange,
	}, nil
}

// VerifyNotify 验证异步通知签名
func (c *Client) VerifyNotify(form url.Values) (bool, error) {
	bm, err := gopayalipay.ParseNotifyByURLValues(form)
	if err != nil {
		return false, fmt.Errorf("alipay: parse notify: %w", err)
	}
	return gopayalipay.VerifySign(c.cfg.PublicKey, bm)
}

// --- 请求/响应类型 ---

type PagePayRequest struct {
	OutTradeNo  string  // 商户订单号
	TotalAmount float64 // 订单总金额（元）
	Subject     string  // 订单标题（如：课程名称）
	Body        string  // 订单描述
}

type WapPayRequest struct {
	OutTradeNo  string  // 商户订单号
	TotalAmount float64 // 订单总金额（元）
	Subject     string  // 订单标题
	QuitURL     string  // 用户退出跳转地址
}

type OrderResult struct {
	OutTradeNo  string
	TradeNo     string // 支付宝交易号
	TradeStatus string // WAIT_BUYER_PAY/TRADE_SUCCESS/TRADE_FINISHED/TRADE_CLOSED
	TotalAmount string // 总金额
	PayTime     string // 支付时间
}

type RefundRequest struct {
	OutTradeNo   string  // 商户订单号
	RefundAmount float64 // 退款金额（元）
	Reason       string  // 退款原因
	OutRequestNo string  // 退款请求号（用于幂等）
}

type RefundResult struct {
	OutTradeNo string
	TradeNo    string
	Status     string
}
