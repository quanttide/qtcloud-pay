// Package alipay 支付宝网页支付实现
// 适用于 PC 端卖课场景。
package alipay

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
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
	cfg       *Config
	client    *http.Client
	privKey   *rsa.PrivateKey
	pubKey    *rsa.PublicKey
	apiURL    string
}

// New 创建支付宝客户端
func New(cfg *Config) (*Client, error) {
	privKey, err := parsePrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("alipay: parse private key: %w", err)
	}
	var pubKey *rsa.PublicKey
	if cfg.PublicKey != "" {
		pubKey, err = parsePublicKey(cfg.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("alipay: parse public key: %w", err)
		}
	}
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		privKey:  privKey,
		pubKey:   pubKey,
		apiURL: "https://openapi.alipay.com/gateway.do",
	}, nil
}

// PagePay 统一收单下单并支付页面（alipay.trade.page.pay）
// 返回一个自动提交的 HTML 表单，后端只需将其返回给浏览器即可跳转支付宝收银台。
func (c *Client) PagePay(req *PagePayRequest) (string, error) {
	biz := map[string]string{
		"out_trade_no": req.OutTradeNo,
		"product_code": "FAST_INSTANT_TRADE_PAY",
		"total_amount": fmt.Sprintf("%.2f", req.TotalAmount),
		"subject":      req.Subject,
	}
	if req.Body != "" {
		biz["body"] = req.Body
	}
	bizContent, _ := json.Marshal(biz)

	params := c.baseParams("alipay.trade.page.pay")
	params["biz_content"] = string(bizContent)
	params["return_url"] = c.cfg.ReturnURL
	params["notify_url"] = c.cfg.NotifyURL

	sign, err := c.sign(c.sortParams(params))
	if err != nil {
		return "", fmt.Errorf("alipay: sign: %w", err)
	}
	params["sign"] = sign

	// 生成自动提交表单
	var inputs string
	for k, v := range params {
		inputs += fmt.Sprintf(`<input type="hidden" name="%s" value="%s" />`, k, htmlEscape(v))
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><body><form id="alipay" action="%s" method="POST">%s</form><script>document.getElementById("alipay").submit()</script></body></html>`,
		c.apiURL, inputs), nil
}

// WapPay 手机网页支付（alipay.trade.wap.pay）
func (c *Client) WapPay(req *WapPayRequest) (string, error) {
	biz := map[string]string{
		"out_trade_no": req.OutTradeNo,
		"product_code": "QUICK_WAP_WAY",
		"total_amount": fmt.Sprintf("%.2f", req.TotalAmount),
		"subject":      req.Subject,
		"quit_url":     req.QuitURL,
	}
	bizContent, _ := json.Marshal(biz)

	params := c.baseParams("alipay.trade.wap.pay")
	params["biz_content"] = string(bizContent)
	params["return_url"] = c.cfg.ReturnURL
	params["notify_url"] = c.cfg.NotifyURL

	sign, err := c.sign(c.sortParams(params))
	if err != nil {
		return "", fmt.Errorf("alipay: sign: %w", err)
	}
	params["sign"] = sign

	var inputs string
	for k, v := range params {
		inputs += fmt.Sprintf(`<input type="hidden" name="%s" value="%s" />`, k, htmlEscape(v))
	}
	return fmt.Sprintf(`<!DOCTYPE html><html><body><form id="alipay" action="%s" method="POST">%s</form><script>document.getElementById("alipay").submit()</script></body></html>`,
		c.apiURL, inputs), nil
}

// QueryOrder 交易查询（alipay.trade.query）
func (c *Client) QueryOrder(ctx context.Context, outTradeNo string) (*OrderResult, error) {
	biz := map[string]string{"out_trade_no": outTradeNo}
	bizContent, _ := json.Marshal(biz)
	params := c.baseParams("alipay.trade.query")
	params["biz_content"] = string(bizContent)

	resp, err := c.call(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("alipay: query order: %w", err)
	}
	return parseQueryResponse(resp)
}

// Refund 交易退款（alipay.trade.refund）
func (c *Client) Refund(ctx context.Context, req *RefundRequest) (*RefundResult, error) {
	biz := map[string]string{
		"out_trade_no":   req.OutTradeNo,
		"refund_amount":  fmt.Sprintf("%.2f", req.RefundAmount),
	}
	if req.Reason != "" {
		biz["refund_reason"] = req.Reason
	}
	if req.OutRequestNo != "" {
		biz["out_request_no"] = req.OutRequestNo
	}
	bizContent, _ := json.Marshal(biz)
	params := c.baseParams("alipay.trade.refund")
	params["biz_content"] = string(bizContent)

	resp, err := c.call(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("alipay: refund: %w", err)
	}
	return parseRefundResponse(resp)
}

// VerifyNotify 验证异步通知签名并返回通知参数
// 支付宝异步通知通过 POST 表单形式发送
func (c *Client) VerifyNotify(form url.Values) (bool, error) {
	sign := form.Get("sign")
	if sign == "" {
		return false, fmt.Errorf("alipay: notify missing sign")
	}
	// 剔除 sign 和 sign_type 后排序拼接
	content := c.verifyNotifyParams(form)
	return c.verifyRSA2(content, sign)
}

// --- 内部 ---

func (c *Client) baseParams(method string) map[string]string {
	return map[string]string{
		"app_id":      c.cfg.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
	}
}

func (c *Client) call(ctx context.Context, params map[string]string) ([]byte, error) {
	sign, err := c.sign(c.sortParams(params))
	if err != nil {
		return nil, err
	}
	params["sign"] = sign

	data := url.Values{}
	for k, v := range params {
		data.Set(k, v)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// 解析响应 JSON
	var raw struct {
		Response json.RawMessage `json:"alipay_trade_query_response"`
		Sign     string          `json:"sign"`
	}
	// 支付宝响应格式：{"方法名+Response": {...}, "sign": "..."}
	// 动态解析
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("alipay: decode response: %w", err)
	}
	// 找到第一个 key 为 Response 的字段
	for k, v := range m {
		if strings.HasSuffix(k, "_response") {
			raw.Response = v
		} else if k == "sign" {
			json.Unmarshal(v, &raw.Sign)
		}
	}
	if raw.Response == nil {
		return nil, fmt.Errorf("alipay: no response field: %s", string(body))
	}
	// 验签
	if c.pubKey != nil && raw.Sign != "" {
		content := string(raw.Response)
		ok, err := c.verifyRSA2(content, raw.Sign)
		if err != nil || !ok {
			return nil, fmt.Errorf("alipay: verify sign failed")
		}
	}
	return raw.Response, nil
}

func (c *Client) sign(content string) (string, error) {
	h := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.privKey, crypto.SHA256, h[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func (c *Client) verifyRSA2(content, sign string) (bool, error) {
	if c.pubKey == nil {
		return true, nil // 无公钥时跳过验签
	}
	sig, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false, err
	}
	h := sha256.Sum256([]byte(content))
	err = rsa.VerifyPKCS1v15(c.pubKey, crypto.SHA256, h[:], sig)
	return err == nil, err
}

// sortParams 排序并拼接成待签名字符串
func (c *Client) sortParams(params map[string]string) string {
	return c.sortAndJoin(params)
}

func (c *Client) sortAndJoin(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var pairs []string
	for _, k := range keys {
		v := params[k]
		if v != "" {
			pairs = append(pairs, k+"="+v)
		}
	}
	return strings.Join(pairs, "&")
}

// verifyNotifyParams 对 url.Values 做排序拼接（用于异步通知验签）
func (c *Client) verifyNotifyParams(form url.Values) string {
	params := make(map[string]string)
	for k, vs := range form {
		if len(vs) > 0 {
			params[k] = vs[0]
		}
	}
	return c.sortAndJoin(params)
}

// --- 辅助 ---

func parsePrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	// 尝试 PKCS8 再尝试 PKCS1
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err == nil {
		return key.(*rsa.PrivateKey), nil
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("no PEM block")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		return key.(*rsa.PublicKey), nil
	}
	return x509.ParsePKCS1PublicKey(block.Bytes)
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func parseQueryResponse(b []byte) (*OrderResult, error) {
	var raw struct {
		OutTradeNo   string `json:"out_trade_no"`
		TradeNo      string `json:"trade_no"`
		TradeStatus  string `json:"trade_status"`
		TotalAmount  string `json:"total_amount"`
		SendPayDate  string `json:"send_pay_date"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return &OrderResult{
		OutTradeNo:  raw.OutTradeNo,
		TradeNo:     raw.TradeNo,
		TradeStatus: raw.TradeStatus,
		TotalAmount: raw.TotalAmount,
		PayTime:     raw.SendPayDate,
	}, nil
}

func parseRefundResponse(b []byte) (*RefundResult, error) {
	var raw struct {
		OutTradeNo   string `json:"out_trade_no"`
		TradeNo      string `json:"trade_no"`
		RefundStatus string `json:"fund_change"` // Y/N
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return &RefundResult{
		OutTradeNo: raw.OutTradeNo,
		TradeNo:    raw.TradeNo,
		Status:     raw.RefundStatus,
	}, nil
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
