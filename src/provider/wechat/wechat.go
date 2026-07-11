// Package wechat 微信支付 JSAPI 实现（V3 API）
// 适用于公众号/小程序卖课场景。
// 底层委托 github.com/go-pay/gopay/wechat/v3。
package wechat

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"

	"github.com/go-pay/gopay"
	gopaywechat "github.com/go-pay/gopay/wechat/v3"
)

// Config 微信支付配置
type Config struct {
	AppID     string // 公众号/小程序 AppID
	MchID     string // 商户号
	APIv3Key  string // API v3 密钥（用于解密通知）
	MchCert   string // 商户证书 PEM
	MchKey    string // 商户私钥 PEM
	NotifyURL string // 支付结果通知地址
}

// Client 微信支付客户端
type Client struct {
	cfg     *Config
	gclient *gopaywechat.ClientV3
}

// SetTransport 设置 HTTP 传输层（用于测试）
func (c *Client) SetTransport(_ http.RoundTripper) {}

// New 创建微信支付客户端
func New(cfg *Config) (*Client, error) {
	// 从证书 PEM 中提取序列号
	serial, err := parseCertSerial(cfg.MchCert)
	if err != nil {
		return nil, fmt.Errorf("wechat: parse cert serial: %w", err)
	}
	// gopay NewClientV3(mchid, serialNo, apiV3Key, privateKey)
	gclient, err := gopaywechat.NewClientV3(cfg.MchID, serial, cfg.APIv3Key, cfg.MchKey)
	if err != nil {
		return nil, fmt.Errorf("wechat: new gopay client: %w", err)
	}
	return &Client{cfg: cfg, gclient: gclient}, nil
}

// JSAPIPay JSAPI 下单，返回前端调起支付所需的参数
func (c *Client) JSAPIPay(ctx context.Context, req *JSAPIPayRequest) (*JSAPIPayResponse, error) {
	bm := make(gopay.BodyMap)
	bm.Set("appid", c.cfg.AppID).
		Set("description", req.Description).
		Set("out_trade_no", req.OutTradeNo).
		Set("notify_url", c.cfg.NotifyURL).
		SetBodyMap("amount", func(bm gopay.BodyMap) {
			bm.Set("total", req.Total).
				Set("currency", "CNY")
		}).
		SetBodyMap("payer", func(bm gopay.BodyMap) {
			bm.Set("openid", req.OpenID)
		})

	rsp, err := c.gclient.V3TransactionJsapi(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("wechat: jsapi pay: %w", err)
	}
	if rsp.Code != 0 {
		return nil, fmt.Errorf("wechat: jsapi pay failed: code=%d error=%s", rsp.Code, rsp.Error)
	}

	// 使用 gopay 生成前端调起支付参数
	jsapi, err := c.gclient.PaySignOfJSAPI(c.cfg.AppID, rsp.Response.PrepayId)
	if err != nil {
		return nil, fmt.Errorf("wechat: generate jsapi pay sign: %w", err)
	}
	return &JSAPIPayResponse{
		AppID:     jsapi.AppId,
		Timestamp: jsapi.TimeStamp,
		NonceStr:  jsapi.NonceStr,
		Package:   jsapi.Package,
		SignType:  jsapi.SignType,
		PaySign:   jsapi.PaySign,
	}, nil
}

// QueryOrder 根据微信支付交易号查询订单
func (c *Client) QueryOrder(ctx context.Context, transactionID string) (*OrderResult, error) {
	rsp, err := c.gclient.V3TransactionQueryOrder(ctx, gopaywechat.TransactionId, transactionID)
	if err != nil {
		return nil, fmt.Errorf("wechat: query order: %w", err)
	}
	if rsp.Code != 0 {
		return nil, fmt.Errorf("wechat: query order failed: code=%d error=%s", rsp.Code, rsp.Error)
	}
	return orderResultFromRsp(rsp), nil
}

// QueryOrderByOutTradeNo 根据商户订单号查询订单
func (c *Client) QueryOrderByOutTradeNo(ctx context.Context, outTradeNo string) (*OrderResult, error) {
	rsp, err := c.gclient.V3TransactionQueryOrder(ctx, gopaywechat.OutTradeNo, outTradeNo)
	if err != nil {
		return nil, fmt.Errorf("wechat: query order by out trade no: %w", err)
	}
	if rsp.Code != 0 {
		return nil, fmt.Errorf("wechat: query order by out trade no failed: code=%d error=%s", rsp.Code, rsp.Error)
	}
	return orderResultFromRsp(rsp), nil
}

// ParseNotify 解析并验证支付结果通知
func (c *Client) ParseNotify(body []byte, headers http.Header) (*NotifyResult, error) {
	notifyReq := &gopaywechat.V3NotifyReq{
		SignInfo: &gopaywechat.SignInfo{
			HeaderTimestamp: headers.Get("Wechatpay-Timestamp"),
			HeaderNonce:     headers.Get("Wechatpay-Nonce"),
			HeaderSignature: headers.Get("Wechatpay-Signature"),
			HeaderSerial:    headers.Get("Wechatpay-Serial"),
			SignBody:        string(body),
		},
	}
	// 解析 body JSON 填充 Resource 字段
	if err := json.Unmarshal(body, notifyReq); err != nil {
		return nil, fmt.Errorf("wechat: parse notify body: %w", err)
	}
	result, err := notifyReq.DecryptPayCipherText(c.cfg.APIv3Key)
	if err != nil {
		return nil, fmt.Errorf("wechat: decrypt notify: %w", err)
	}
	return &NotifyResult{
		TransactionID: result.TransactionId,
		OutTradeNo:    result.OutTradeNo,
		TradeState:    result.TradeState,
		TradeType:     result.TradeType,
		SuccessTime:   result.SuccessTime,
		Payer: struct {
			OpenID string `json:"openid"`
		}{
			OpenID: result.Payer.Openid,
		},
		Amount: struct {
			Total     int `json:"total"`
			PayerTotal int `json:"payer_total"`
		}{
			Total:     result.Amount.Total,
			PayerTotal: result.Amount.PayerTotal,
		},
	}, nil
}

// Refund 退款
func (c *Client) Refund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	bm := make(gopay.BodyMap)
	bm.Set("transaction_id", req.TransactionID).
		Set("out_trade_no", req.OutTradeNo).
		Set("out_refund_no", req.OutRefundNo).
		SetBodyMap("amount", func(bm gopay.BodyMap) {
			bm.Set("refund", req.RefundAmount).
				Set("total", req.TotalAmount).
				Set("currency", "CNY")
		})
	if req.Reason != "" {
		bm.Set("reason", req.Reason)
	}

	rsp, err := c.gclient.V3Refund(ctx, bm)
	if err != nil {
		return nil, fmt.Errorf("wechat: refund: %w", err)
	}
	if rsp.Code != 0 {
		return nil, fmt.Errorf("wechat: refund failed: code=%d error=%s", rsp.Code, rsp.Error)
	}
	return &RefundResult{
		RefundID:    rsp.Response.RefundId,
		OutRefundNo: rsp.Response.OutRefundNo,
		Status:      rsp.Response.Status,
	}, nil
}

// --- 辅助 ---

func parseCertSerial(pemStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", fmt.Errorf("no PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse cert: %w", err)
	}
	return cert.SerialNumber.Text(16), nil
}

func orderResultFromRsp(rsp *gopaywechat.QueryOrderRsp) *OrderResult {
	if rsp == nil || rsp.Response == nil {
		return &OrderResult{}
	}
	resp := rsp.Response
	total := 0
	if resp.Amount != nil {
		total = resp.Amount.Total
	}
	return &OrderResult{
		TransactionID: resp.TransactionId,
		OutTradeNo:    resp.OutTradeNo,
		TradeState:    resp.TradeState,
		Total:         total,
		SuccessTime:   resp.SuccessTime,
	}
}

// --- 请求/响应类型 ---

type JSAPIPayRequest struct {
	OpenID      string // 用户在公众号/小程序的 openid
	Description string // 商品描述（如：课程名称）
	OutTradeNo  string // 商户订单号
	Total       int    // 订单总金额（分）
}

type JSAPIPayResponse struct {
	AppID     string `json:"appId"`
	Timestamp string `json:"timeStamp"`
	NonceStr  string `json:"nonceStr"`
	Package   string `json:"package"`
	SignType  string `json:"signType"`
	PaySign   string `json:"paySign"`
}

type OrderResult struct {
	TransactionID string
	OutTradeNo    string
	TradeState    string // SUCCESS/NOTPAY/CLOSED/REFUND
	Total         int    // 总金额（分）
	SuccessTime   string
}

type NotifyResult struct {
	TransactionID string `json:"transaction_id"`
	OutTradeNo    string `json:"out_trade_no"`
	TradeState    string `json:"trade_state"`
	TradeType     string `json:"trade_type"`
	SuccessTime   string `json:"success_time"`
	Payer         struct {
		OpenID string `json:"openid"`
	} `json:"payer"`
	Amount struct {
		Total     int `json:"total"`
		PayerTotal int `json:"payer_total"`
	} `json:"amount"`
}

type RefundRequest struct {
	TransactionID string // 微信支付交易号（与 OutTradeNo 二选一）
	OutTradeNo    string // 商户订单号
	OutRefundNo   string // 商户退款单号
	RefundAmount  int    // 退款金额（分）
	TotalAmount   int    // 原订单总金额（分）
	Reason        string // 退款原因
}

type RefundResult struct {
	RefundID    string
	OutRefundNo string
	Status      string // SUCCESS/CLOSED/PROCESSING
}
