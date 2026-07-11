package provider

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/alipay"
	"github.com/quanttide/qtcloud-pay/src/provider/wechat"
)

func generateTestKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func generateTestCertPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	// Also need private key for wechat client
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	_ = privDER
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func testWechatConfig(t *testing.T) *wechat.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	return &wechat.Config{
		AppID: "wx123", MchID: "mch123",
		APIv3Key: "test-api-v3-key-1234567890abcd",
		MchKey:   privPEM, MchCert: certPEM,
		NotifyURL: "https://example.com/notify",
	}
}

func testAlipayConfig(t *testing.T) *alipay.Config {
	t.Helper()
	return &alipay.Config{
		AppID: "2021000000000001", PrivateKey: generateTestKeyPEM(t),
		NotifyURL: "https://example.com/notify",
	}
}

func TestNewWechatPay(t *testing.T) {
	p, err := NewWechatPay(testWechatConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "wechat" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestNewWechatPay_Invalid(t *testing.T) {
	cfg := testWechatConfig(t)
	cfg.MchKey = "bad"
	_, err := NewWechatPay(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewAlipayPay(t *testing.T) {
	p, err := NewAlipayPay(testAlipayConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if p.Name() != "alipay" {
		t.Errorf("Name = %q", p.Name())
	}
}

func TestNewAlipayPay_Invalid(t *testing.T) {
	cfg := testAlipayConfig(t)
	cfg.PrivateKey = "bad"
	_, err := NewAlipayPay(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWechatPay_Pay(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"prepay_id": "wxPREPAY123"})
	}))
	defer ts.Close()

	p := mustNewWechatPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})

	resp, err := p.Pay(context.Background(), &PayRequest{
		OrderID: "ORD001", Amount: 99.99, Subject: "课程",
		Metadata: map[string]any{"openid": "o-test-openid"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RawResponse == nil {
		t.Error("RawResponse should not be nil")
	}
	jsapiResp, ok := resp.RawResponse.(*wechat.JSAPIPayResponse)
	if !ok {
		t.Fatalf("RawResponse type = %T", resp.RawResponse)
	}
	if jsapiResp.AppID != "wx123" {
		t.Errorf("AppID = %q", jsapiResp.AppID)
	}
}

func TestWechatPay_Pay_NoMetadata(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"prepay_id": "wxPREPAY123"})
	}))
	defer ts.Close()

	p := mustNewWechatPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})

	_, err := p.Pay(context.Background(), &PayRequest{
		OrderID: "ORD001", Amount: 99.99, Subject: "课程",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWechatPay_Query(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"transaction_id": "tx001", "out_trade_no": "ORD001",
			"trade_state": "SUCCESS",
			"amount":      map[string]any{"total": 9999, "payer_total": 9999},
			"success_time": "2025-07-11T10:00:00+08:00",
		})
	}))
	defer ts.Close()

	p := mustNewWechatPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})

	status, err := p.Query(context.Background(), "ORD001")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "SUCCESS" {
		t.Errorf("Status = %q", status.Status)
	}
	if status.Amount != 99.99 {
		t.Errorf("Amount = %f", status.Amount)
	}
	if status.TradeID != "tx001" {
		t.Errorf("TradeID = %q", status.TradeID)
	}
}

func TestWechatPay_Refund(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"refund_id": "rf001", "out_refund_no": "ORD001-REFUND", "status": "SUCCESS",
		})
	}))
	defer ts.Close()

	p := mustNewWechatPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})

	resp, err := p.Refund(context.Background(), &RefundRequest{
		OrderID: "ORD001", RefundAmount: 99.99, Reason: "退款",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RefundID != "rf001" {
		t.Errorf("RefundID = %q", resp.RefundID)
	}
	if resp.Status != "SUCCESS" {
		t.Errorf("Status = %q", resp.Status)
	}
}

func TestAlipayPay_Pay(t *testing.T) {
	p := mustNewAlipayPay(t)
	resp, err := p.Pay(context.Background(), &PayRequest{
		OrderID: "ORD001", Amount: 99.99, Subject: "课程",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.PayURL == "" {
		t.Error("PayURL should not be empty")
	}
	if !contains(resp.PayURL, "ORD001") {
		t.Error("PayURL should contain order ID")
	}
}

func TestAlipayPay_Query(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_query_response": map[string]any{
				"out_trade_no": "ORD001", "trade_no": "tx001",
				"trade_status": "TRADE_SUCCESS", "total_amount": "99.99",
			},
			"sign": "",
		})
	}))
	defer ts.Close()

	p := mustNewAlipayPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})
	p.client.SetAPIURL(ts.URL)

	status, err := p.Query(context.Background(), "ORD001")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "SUCCESS" {
		t.Errorf("Status = %q", status.Status)
	}
	if status.TradeID != "tx001" {
		t.Errorf("TradeID = %q", status.TradeID)
	}
}

func TestAlipayPay_Query_UnknownStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_query_response": map[string]any{
				"out_trade_no": "ORD001", "trade_no": "tx001",
				"trade_status": "WAIT_BUYER_PAY", "total_amount": "99.99",
			},
			"sign": "",
		})
	}))
	defer ts.Close()

	p := mustNewAlipayPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})
	p.client.SetAPIURL(ts.URL)

	status, err := p.Query(context.Background(), "ORD001")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "PENDING" {
		t.Errorf("Status = %q, want PENDING", status.Status)
	}
}

func TestAlipayPay_Query_UnknownTradeStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_query_response": map[string]any{
				"out_trade_no": "ORD001", "trade_no": "tx001",
				"trade_status": "SOME_UNKNOWN_STATUS", "total_amount": "0.00",
			},
			"sign": "",
		})
	}))
	defer ts.Close()

	p := mustNewAlipayPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})
	p.client.SetAPIURL(ts.URL)

	status, err := p.Query(context.Background(), "ORD001")
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "UNKNOWN" {
		t.Errorf("Status = %q, want UNKNOWN", status.Status)
	}
}

func TestAlipayPay_Refund(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_refund_response": map[string]any{
				"out_trade_no": "ORD001", "trade_no": "tx001",
			},
			"sign": "",
		})
	}))
	defer ts.Close()

	p := mustNewAlipayPay(t)
	p.client.SetTransport(&providerTransport{base: ts.URL})
	p.client.SetAPIURL(ts.URL)

	resp, err := p.Refund(context.Background(), &RefundRequest{
		OrderID: "ORD001", RefundAmount: 50.00, Reason: "部分退款",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.RefundID != "ORD001" {
		t.Errorf("RefundID = %q", resp.RefundID)
	}
	if resp.Status != "SUCCESS" {
		t.Errorf("Status = %q", resp.Status)
	}
}

func TestWechatPay_Pay_Error(t *testing.T) {
	p := mustNewWechatPay(t)
	p.client.SetTransport(&errorTransport{})
	_, err := p.Pay(context.Background(), &PayRequest{
		OrderID: "ORD001", Amount: 99.99, Subject: "课程",
		Metadata: map[string]any{"openid": "o-test"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWechatPay_Query_Error(t *testing.T) {
	p := mustNewWechatPay(t)
	p.client.SetTransport(&errorTransport{})
	_, err := p.Query(context.Background(), "ORD001")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWechatPay_Refund_Error(t *testing.T) {
	p := mustNewWechatPay(t)
	p.client.SetTransport(&errorTransport{})
	_, err := p.Refund(context.Background(), &RefundRequest{
		OrderID: "ORD001", RefundAmount: 99.99, Reason: "退款",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAlipayPay_Query_Error(t *testing.T) {
	p := mustNewAlipayPay(t)
	p.client.SetTransport(&errorTransport{})
	p.client.SetAPIURL("http://error")
	_, err := p.Query(context.Background(), "ORD001")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAlipayPay_Refund_Error(t *testing.T) {
	p := mustNewAlipayPay(t)
	p.client.SetTransport(&errorTransport{})
	p.client.SetAPIURL("http://error")
	_, err := p.Refund(context.Background(), &RefundRequest{
		OrderID: "ORD001", RefundAmount: 50.00, Reason: "退款",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		s    string
		want float64
	}{
		{"99.99", 99.99},
		{"0.01", 0.01},
		{"100", 100},
		{"0", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := parseAmount(tt.s)
		if got != tt.want {
			t.Errorf("parseAmount(%q) = %f, want %f", tt.s, got, tt.want)
		}
	}
}

func mustNewWechatPay(t *testing.T) *WechatPay {
	t.Helper()
	p, err := NewWechatPay(testWechatConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func mustNewAlipayPay(t *testing.T) *AlipayPay {
	t.Helper()
	p, err := NewAlipayPay(testAlipayConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("transport error")
}

type providerTransport struct {
	base string
}

func (rt *providerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := rt.base + req.URL.Path
	if req.URL.RawQuery != "" {
		u += "?" + req.URL.RawQuery
	}
	newReq, _ := http.NewRequest(req.Method, u, req.Body)
	newReq.Header = req.Header
	return http.DefaultTransport.RoundTrip(newReq)
}

func contains(s, substr string) bool {
	// simple contains check
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkWechatPay(b *testing.B) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"prepay_id": "wxPREPAY123"})
	}))
	defer ts.Close()

	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))

	p, err := NewWechatPay(&wechat.Config{
		AppID: "wx123", MchID: "mch123",
		APIv3Key: "test-api-v3-key-1234567890abcd",
		MchKey: privPEM, MchCert: certPEM,
		NotifyURL: "https://example.com/notify",
	})
	if err != nil {
		b.Fatal(err)
	}
	p.client.SetTransport(&providerTransport{base: ts.URL})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Pay(context.Background(), &PayRequest{
			OrderID: fmt.Sprintf("ORD%05d", i), Amount: 99.99, Subject: "课程",
			Metadata: map[string]any{"openid": "o-test"},
		})
	}
}

func BenchmarkAlipayPay(b *testing.B) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))
	p, err := NewAlipayPay(&alipay.Config{
		AppID: "2021000000000001", PrivateKey: privPEM,
		NotifyURL: "https://example.com/notify",
	})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Pay(context.Background(), &PayRequest{
			OrderID: fmt.Sprintf("ORD%05d", i), Amount: 99.99, Subject: "课程",
		})
	}
}
