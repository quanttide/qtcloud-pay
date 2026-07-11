package wechat

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"  //nolint:revive
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	gwx "github.com/go-pay/gopay/wechat/v3"
)

// test key/cert material — generated once
var (
	testKeyPEM = func() string {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		der, _ := x509.MarshalPKCS8PrivateKey(key)
		return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	}()

	testCertPEM = func() string {
		key, _ := rsa.GenerateKey(rand.Reader, 2048)
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject:      pkix.Name{CommonName: "test"},
			NotBefore:    time.Now().Add(-1 * time.Hour),
			NotAfter:     time.Now().Add(1 * time.Hour),
		}
		certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	}()

	testConfigObj = &Config{
			AppID:     "wx1234567890abcdef",
			MchID:     "1234567890",
			APIv3Key:  "test-api-v3-key-1234567890abcdef", // exactly 32 bytes for AES-256
			MchKey:    testKeyPEM,
			MchCert:   testCertPEM,
			NotifyURL: "https://example.com/notify",
		}
)

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

// ── New ──

func TestNew_Valid(t *testing.T) {
	c, err := New(testConfigObj)
	if err != nil {
		t.Fatal(err)
	}
	if c.cfg.AppID != "wx1234567890abcdef" {
		t.Errorf("AppID = %q", c.cfg.AppID)
	}
}

func TestNew_InvalidKey(t *testing.T) {
	cfg := *testConfigObj
	cfg.MchKey = "bad"
	_, err := New(&cfg)
	if err == nil {
		t.Fatal("expected error for bad key")
	}
}

func TestNew_InvalidCert(t *testing.T) {
	cfg := *testConfigObj
	cfg.MchCert = "bad"
	_, err := New(&cfg)
	if err == nil {
		t.Fatal("expected error for bad cert")
	}
}

// ── JSAPIPay ──

func TestJSAPIPay_Success(t *testing.T) {
	c, err := New(testConfigObj)
	if err != nil {
		t.Fatal(err)
	}
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			if strings.Contains(req.URL.Path, "jsapi") {
				return jsonResponse(200, map[string]string{"prepay_id": "wx111111111111111"}), nil
			}
			return jsonResponse(200, map[string]string{}), nil
		},
	})

	rsp, err := c.JSAPIPay(context.Background(), &JSAPIPayRequest{
		OpenID: "o123", OutTradeNo: "ORD001", Total: 100, Description: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if rsp.AppID != "wx1234567890abcdef" {
		t.Errorf("AppID = %q", rsp.AppID)
	}
	if rsp.Package != "prepay_id=wx111111111111111" {
		t.Errorf("Package = %q", rsp.Package)
	}
	if rsp.SignType != "RSA" {
		t.Errorf("SignType = %q", rsp.SignType)
	}
	if rsp.PaySign == "" {
		t.Error("PaySign should not be empty")
	}
}

func TestJSAPIPay_ServerError(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(500, map[string]string{"code": "SYSTEM_ERROR"}), nil
		},
	})
	_, err := c.JSAPIPay(context.Background(), &JSAPIPayRequest{
		OpenID: "o123", OutTradeNo: "ORD001", Total: 100,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── QueryOrder ──

func TestQueryOrder_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"transaction_id": "tx001",
				"out_trade_no":   "ORD001",
				"trade_state":    "SUCCESS",
				"amount":         map[string]any{"total": 100, "payer_total": 100, "currency": "CNY"},
				"success_time":   "2025-07-11T10:00:00+08:00",
			}), nil
		},
	})

	r, err := c.QueryOrder(context.Background(), "tx001")
	if err != nil {
		t.Fatal(err)
	}
	if r.TransactionID != "tx001" {
		t.Errorf("TransactionID = %q", r.TransactionID)
	}
	if r.TradeState != "SUCCESS" {
		t.Errorf("TradeState = %q", r.TradeState)
	}
	if r.Total != 100 {
		t.Errorf("Total = %d", r.Total)
	}
}

func TestQueryOrder_NotFound(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(500, map[string]string{"code": "ORDER_NOT_EXIST"}), nil
		},
	})
	_, err := c.QueryOrder(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestQueryOrderByOutTradeNo_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"transaction_id": "tx002",
				"out_trade_no":   "ORD002",
				"trade_state":    "NOTPAY",
				"amount":         map[string]any{"total": 200, "payer_total": 200, "currency": "CNY"},
			}), nil
		},
	})

	r, err := c.QueryOrderByOutTradeNo(context.Background(), "ORD002")
	if err != nil {
		t.Fatal(err)
	}
	if r.OutTradeNo != "ORD002" {
		t.Errorf("OutTradeNo = %q", r.OutTradeNo)
	}
	if r.TradeState != "NOTPAY" {
		t.Errorf("TradeState = %q", r.TradeState)
	}
}

// ── ParseNotify ──

func TestParseNotify_MissingHeaders(t *testing.T) {
	c, _ := New(testConfigObj)
	_, err := c.ParseNotify([]byte(`{}`), http.Header{})
	if err == nil {
		t.Fatal("expected error for missing headers")
	}
}

func TestParseNotify_InvalidJSON(t *testing.T) {
	c, _ := New(testConfigObj)
	h := http.Header{}
	h.Set("Wechatpay-Timestamp", "123")
	h.Set("Wechatpay-Nonce", "abc")
	h.Set("Wechatpay-Signature", "sig")
	h.Set("Wechatpay-Serial", "ser")
	_, err := c.ParseNotify([]byte(`not-json`), h)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseNotify_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	apiV3Key := "test-api-v3-key-1234567890abcdef" // 32 bytes

	plaintext := map[string]any{
		"transaction_id": "tx001",
		"out_trade_no":   "ORD001",
		"trade_state":    "SUCCESS",
		"trade_type":     "JSAPI",
		"success_time":   "2025-07-11T10:00:00+08:00",
		"payer":          map[string]string{"openid": "o123"},
		"amount":         map[string]any{"total": 100, "payer_total": 100, "currency": "CNY"},
	}
	ptJSON, _ := json.Marshal(plaintext)

	// gopay uses raw apiV3Key bytes for AES-256-GCM; nonce MUST be 12 bytes
	block, _ := aes.NewCipher([]byte(apiV3Key))
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, 12)
	rand.Read(nonce)
	ad := "notify"
	ct := gcm.Seal(nil, nonce, ptJSON, []byte(ad))

	envelope := map[string]any{
		"resource": map[string]any{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": ad,
			"nonce":           base64.StdEncoding.EncodeToString(nonce),
		},
	}
	body, _ := json.Marshal(envelope)

	h := http.Header{}
	h.Set("Wechatpay-Timestamp", "1234567890")
	h.Set("Wechatpay-Nonce", "testnonce")
	h.Set("Wechatpay-Signature", "dummy")
	h.Set("Wechatpay-Serial", "dummy")

	result, err := c.ParseNotify(body, h)
	if err != nil {
		t.Fatal(err)
	}
	if result.TransactionID != "tx001" {
		t.Errorf("TransactionID = %q", result.TransactionID)
	}
	if result.Payer.OpenID != "o123" {
		t.Errorf("Payer.OpenID = %q", result.Payer.OpenID)
	}
	if result.Amount.Total != 100 {
		t.Errorf("Amount.Total = %d", result.Amount.Total)
	}
}

// ── Refund ──

func TestRefund_Success(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(200, map[string]any{
				"refund_id": "ref001", "out_refund_no": "ORD001-REFUND", "status": "SUCCESS",
			}), nil
		},
	})

	r, err := c.Refund(context.Background(), &RefundRequest{
		TransactionID: "tx001", OutTradeNo: "ORD001", OutRefundNo: "ORD001-REFUND",
		RefundAmount: 100, TotalAmount: 100, Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.RefundID != "ref001" || r.Status != "SUCCESS" {
		t.Errorf("got RefundID=%q Status=%q", r.RefundID, r.Status)
	}
}

func TestRefund_BizError(t *testing.T) {
	c, _ := New(testConfigObj)
	c.gclient.GetHttpClient().SetTransport(&mockTransport{
		fn: func(req *http.Request) (*http.Response, error) {
			return jsonResponse(500, map[string]string{"code": "SYSTEM_ERROR"}), nil
		},
	})
	_, err := c.Refund(context.Background(), &RefundRequest{
		TransactionID: "tx001", OutTradeNo: "ORD001",
		OutRefundNo: "REF001", RefundAmount: 100, TotalAmount: 100,
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// ── orderResultFromRsp ──

func TestOrderResultFromRsp_Nil(t *testing.T) {
	r := orderResultFromRsp(nil)
	if r.TransactionID != "" {
		t.Errorf("expected empty, got %q", r.TransactionID)
	}
}

func TestOrderResultFromRsp_NilResponse(t *testing.T) {
	r := orderResultFromRsp(&gwx.QueryOrderRsp{})
	if r.TransactionID != "" {
		t.Errorf("expected empty, got %q", r.TransactionID)
	}
}

func TestOrderResultFromRsp_NilAmount(t *testing.T) {
	r := orderResultFromRsp(&gwx.QueryOrderRsp{
		Response: &gwx.QueryOrder{
			TransactionId: "tx001",
			TradeState:    "SUCCESS",
			Amount:        nil,
		},
	})
	if r.Total != 0 {
		t.Errorf("Total = %d, want 0", r.Total)
	}
}

// ── parseCertSerial ──

func TestParseCertSerial_InvalidPEM(t *testing.T) {
	_, err := parseCertSerial("not-pem")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCertSerial_InvalidCert(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not-a-cert")})
	_, err := parseCertSerial(string(block))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseCertSerial_Valid(t *testing.T) {
	s, err := parseCertSerial(testCertPEM)
	if err != nil {
		t.Fatal(err)
	}
	if s == "" {
		t.Fatal("serial should not be empty")
	}
}

// ── misc ──

func TestSetTransportNoop(t *testing.T) {
	c, _ := New(testConfigObj)
	c.SetTransport(nil)
}

// ensure gwx import is used
var _ = gwx.TransactionId
