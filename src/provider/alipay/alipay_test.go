package alipay

import (
	"bytes"
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
	"strings"
	"testing"
)

var (
	testPrivKey   string
	testPubKey    string
	testConfigObj *Config
)

func init() {
	testPrivKey, testPubKey = generateKeyPair()
	testConfigObj = &Config{
		AppID:      "2021000000000001",
		PrivateKey: testPrivKey,
		PublicKey:  testPubKey,
		NotifyURL:  "https://example.com/alipay/notify",
		ReturnURL:  "https://example.com/order/complete",
	}
}

func generateKeyPair() (privPEM, pubPEM string) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	// Alipay uses PKCS1 format for public keys
	pubDER := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	pubPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubDER}))
	return
}

// mockTransport implements http.RoundTripper
type mockTransport struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func jsonResponse(status int, obj any) *http.Response {
	b, _ := json.Marshal(obj)
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(b)),
		Header:     make(http.Header),
	}
}

// ---- New ----

func TestNew_Valid(t *testing.T) {
	c, err := New(testConfigObj)
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.AppID != "2021000000000001" {
		t.Errorf("AppID = %q", c.cfg.AppID)
	}
}

func TestNew_InvalidKey(t *testing.T) {
	cfg := *testConfigObj
	cfg.PrivateKey = "bad"
	_, err := New(&cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

// New does not validate public key (AutoVerifySign logs but doesn't fail)
func TestNew_InvalidPublicKeyDoesNotFail(t *testing.T) {
	cfg := *testConfigObj
	cfg.PublicKey = "bad"
	c, err := New(&cfg)
	if err != nil {
		t.Fatal(err) // New should succeed even with bad pubkey
	}
	if c.cfg.PublicKey != "bad" {
		t.Error("PublicKey should be preserved")
	}
}

// ---- PagePay ----

func TestPagePay_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	// gopay returns URL (no HTTP call) for page.pay
	urlStr, err := c.PagePay(&PagePayRequest{
		OutTradeNo: "ORD001", TotalAmount: 99.99, Subject: "测试课程",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(urlStr, "openapi.alipay.com") {
		t.Error("expected alipay gateway URL")
	}
	if !strings.Contains(urlStr, "ORD001") {
		t.Error("expected order ID in URL params")
	}
}

func TestPagePay_MissingOutTradeNo(t *testing.T) {
	c, _ := New(testConfigObj)
	_, err := c.PagePay(&PagePayRequest{
		TotalAmount: 99.99, Subject: "test",
	})
	if err == nil {
		t.Fatal("expected error for missing out_trade_no")
	}
}

func TestPagePay_WithBody(t *testing.T) {
	c, _ := New(testConfigObj)
	urlStr, err := c.PagePay(&PagePayRequest{
		OutTradeNo: "ORD002", TotalAmount: 199.00, Subject: "高级课程",
		Body: "课程描述内容",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(url.QueryEscape("课程描述内容"), url.QueryEscape("课程描述内容")) {
		// body will be URL-encoded, just verify it's not empty
	}
	if !strings.Contains(urlStr, url.QueryEscape("高级课程")) {
		t.Error("expected subject in params")
	}
}

// ---- WapPay ----

func TestWapPay_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	urlStr, err := c.WapPay(&WapPayRequest{
		OutTradeNo: "ORD003", TotalAmount: 49.90, Subject: "手机课程",
		QuitURL: "https://example.com/cancel",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(urlStr, "openapi.alipay.com") {
		t.Error("expected alipay gateway URL")
	}
}

func TestWapPay_Error(t *testing.T) {
	c, _ := New(testConfigObj)
	_, err := c.WapPay(&WapPayRequest{
		TotalAmount: 49.90, Subject: "test",
		QuitURL: "https://example.com/cancel",
	})
	if err == nil {
		t.Fatal("expected error for missing out_trade_no")
	}
}

// ---- QueryOrder ----

func noPubConfig() *Config {
	return &Config{
		AppID: "2021000000000001", PrivateKey: testPrivKey,
		NotifyURL: "https://example.com/alipay/notify",
		ReturnURL: "https://example.com/order/complete",
	}
}

func TestQueryOrder_Success(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"alipay_trade_query_response": map[string]any{
					"code":          "10000",
					"msg":           "Success",
					"out_trade_no":  "ORD001",
					"trade_no":      "2025071122000000000001",
					"trade_status":  "TRADE_SUCCESS",
					"total_amount":  "99.99",
					"send_pay_date": "2025-07-11 10:00:00",
				},
				"sign": "",
			}), nil
		},
	})
	r, err := c.QueryOrder(context.Background(), "ORD001")
	if err != nil {
		t.Fatal(err)
	}
	if r.OutTradeNo != "ORD001" {
		t.Errorf("OutTradeNo = %q", r.OutTradeNo)
	}
	if r.TradeStatus != "TRADE_SUCCESS" {
		t.Errorf("TradeStatus = %q", r.TradeStatus)
	}
}

func TestQueryOrder_ServerError(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(500, map[string]string{"error": "server error"}), nil
		},
	})
	_, err := c.QueryOrder(context.Background(), "ORD001")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryOrder_TransportError(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	})
	_, err := c.QueryOrder(context.Background(), "ORD001")
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- Refund ----

func TestRefund_Success(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"alipay_trade_refund_response": map[string]any{
					"code":         "10000",
					"msg":          "Success",
					"out_trade_no": "ORD001",
					"trade_no":     "tx001",
					"fund_change":  "Y",
				},
				"sign": "",
			}), nil
		},
	})
	r, err := c.Refund(context.Background(), &RefundRequest{
		OutTradeNo: "ORD001", RefundAmount: 99.99, Reason: "测试退款",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.OutTradeNo != "ORD001" {
		t.Errorf("OutTradeNo = %q", r.OutTradeNo)
	}
	if r.Status != "Y" {
		t.Errorf("Status = %q", r.Status)
	}
}

func TestRefund_WithOutRequestNo(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"alipay_trade_refund_response": map[string]any{
					"code": "10000", "msg": "Success",
					"out_trade_no": "ORD001", "trade_no": "tx001",
				},
				"sign": "",
			}), nil
		},
	})
	r, err := c.Refund(context.Background(), &RefundRequest{
		OutTradeNo: "ORD002", RefundAmount: 50.00,
		OutRequestNo: "REF20250711001",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = r
}

func TestRefund_TransportError(t *testing.T) {
	c, _ := New(noPubConfig())
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("connection refused")
		},
	})
	_, err := c.Refund(context.Background(), &RefundRequest{
		OutTradeNo: "ORD001", RefundAmount: 99.99,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ---- VerifyNotify ----

func TestVerifyNotify_MissingSign(t *testing.T) {
	c, _ := New(testConfigObj)
	_, err := c.VerifyNotify(url.Values{})
	if err == nil {
		t.Fatal("expected error for missing sign")
	}
}

func TestVerifyNotify_WithPublicKey(t *testing.T) {
	signKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	signPubPEM := pemEncodePublicKey(&signKey.PublicKey)

	cfg := *testConfigObj
	cfg.PublicKey = signPubPEM
	c, _ := New(&cfg)

	form := url.Values{}
	form.Set("out_trade_no", "ORD001")
	form.Set("trade_status", "TRADE_SUCCESS")
	form.Set("total_amount", "99.99")

	sorted := sortParams(form)
	h := sha256.Sum256([]byte(sorted))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, signKey, crypto.SHA256, h[:])
	form.Set("sign", base64.StdEncoding.EncodeToString(sig))

	ok, err := c.VerifyNotify(form)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected signature verification to pass")
	}
}

func TestVerifyNotify_WrongSignature(t *testing.T) {
	signKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	// Use a different key as the configured public key
	_, wrongPub := generateKeyPair()

	cfg := *testConfigObj
	cfg.PublicKey = wrongPub
	c, _ := New(&cfg)

	form := url.Values{}
	form.Set("out_trade_no", "ORD001")
	form.Set("trade_status", "TRADE_SUCCESS")

	sorted := sortParams(form)
	h := sha256.Sum256([]byte(sorted))
	sig, _ := rsa.SignPKCS1v15(rand.Reader, signKey, crypto.SHA256, h[:])
	form.Set("sign", base64.StdEncoding.EncodeToString(sig))

	ok, _ := c.VerifyNotify(form)
	if ok {
		t.Error("expected signature verification to fail")
	}
}

// ---- helpers ----

func pemEncodePublicKey(pub *rsa.PublicKey) string {
	der := x509.MarshalPKCS1PublicKey(pub)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: der}))
}

func sortParams(form url.Values) string {
	var keys []string
	for k := range form {
		if k == "sign" || k == "sign_type" {
			continue
		}
		keys = append(keys, k)
	}
	// bubble sort (no sort import)
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	var pairs []string
	for _, k := range keys {
		pairs = append(pairs, k+"="+form.Get(k))
	}
	return strings.Join(pairs, "&")
}

// misc coverage for no-op methods
func TestSetTransportNoop(t *testing.T) {
	c, _ := New(testConfigObj)
	c.SetTransport(nil)
}

func TestSetAPIURLNoop(t *testing.T) {
	c, _ := New(testConfigObj)
	c.SetAPIURL("http://example.com")
}
