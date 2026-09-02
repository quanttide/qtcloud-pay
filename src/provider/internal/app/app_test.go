package app

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func TestOpen_SQLite(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	for _, m := range []any{
		&account.Account{}, &transaction.Transaction{},
		&coupon.Coupon{}, &voucher.Voucher{}, &voucher.PricingRuleSet{},
		&order.Order{}, &billing.BillingRule{},
	} {
		if !db.Migrator().HasTable(m) {
			t.Errorf("table missing: %T", m)
		}
	}
}

func TestOpen_PostgresInvalidDSN(t *testing.T) {
	if _, err := Open("postgres", "://bad"); err == nil {
		t.Fatal("expected error for invalid postgres dsn")
	}
}

func TestOpenDB_DefaultDSN(t *testing.T) {
	// 不设置 DB_SQLITE_DSN → 使用默认 qtcloud-pay.db（在临时目录中）
	t.Chdir(t.TempDir())
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", "")
	db, err := OpenDB()
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	if !db.Migrator().HasTable(&account.Account{}) {
		t.Error("accounts table missing")
	}
}

func TestOpenDB_PostgresInvalidDSN(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "://bad")
	if _, err := OpenDB(); err == nil {
		t.Fatal("expected error for invalid postgres dsn")
	}
}

func TestBuildMux_LedgerRoutes(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	mux, err := BuildMux(db, "", "")
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 创建账户
	resp, err := http.Post(ts.URL+"/accounts", "application/json",
		bytes.NewReader([]byte(`{"customer_id":"cust_1"}`)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create account status = %d, want 201", resp.StatusCode)
	}

	// 未挂渠道时 /pay 不存在 → 404
	resp2, _ := http.Get(ts.URL + "/pay")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("/pay status = %d, want 404", resp2.StatusCode)
	}
}

func TestBuildMux_VoucherPricingRuleRoutes(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	mux, err := BuildMux(db, "", "secret")
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	payload := `{"issuance":{"channels":[{"name":"课堂实训任务","trigger":"实训任务验收通过","voucher":{"amount_cents":10000,"scope":"all","expires_at_rule":"发放时确定"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"extra_application_quota","name":"超额申请额度","pricing_model":"per_count_flat","quotas":[{"application_type":"project_proposal","name":"立项申请","free_limit":1,"exceed_price_cents":10000}]}]},"billing_semantics":{"voucher_is_money":true}}`
	body, _ := json.Marshal(map[string]any{
		"source":  "payment-engineering/qtclass/voucher-pricing.json",
		"version": "2026-09-01",
		"payload": json.RawMessage(payload),
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/admin/voucher-pricing-rules/qtclass", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT rule set status = %d, want 200", resp.StatusCode)
	}
}

func TestBuildMux_AlipayChannel(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALIPAY_APP_ID", "2021000000000001")
	t.Setenv("ALIPAY_PRIVATE_KEY", generateTestKeyPEM(t))

	mux, err := BuildMux(db, "alipay", "")
	if err != nil {
		t.Fatalf("BuildMux: %v", err)
	}
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// 渠道路由已挂载：/pay 返回 200（PagePay 无需出网）
	body, _ := json.Marshal(map[string]any{
		"OrderID": "ORD-1", "Amount": 99.99, "Subject": "课程",
	})
	resp, err := http.Post(ts.URL+"/pay", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/pay status = %d, want 200", resp.StatusCode)
	}
}

func TestBuildMux_BadChannel(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildMux(db, "unionpay", ""); err == nil {
		t.Fatal("expected error for unsupported channel")
	}
}

func TestNewProvider(t *testing.T) {
	// alipay 成功
	t.Setenv("ALIPAY_APP_ID", "2021000000000001")
	t.Setenv("ALIPAY_PRIVATE_KEY", generateTestKeyPEM(t))
	p, err := NewProvider("alipay")
	if err != nil || p.Name() != "alipay" {
		t.Fatalf("NewProvider(alipay) = %v, %v", p, err)
	}

	// wechat 成功（需要商户证书与私钥）
	privPEM, certPEM := generateTestCertPEM(t)
	t.Setenv("WECHAT_APP_ID", "wx123")
	t.Setenv("WECHAT_MCH_ID", "mch123")
	t.Setenv("WECHAT_API_V3_KEY", "test-api-v3-key-1234567890abcd")
	t.Setenv("WECHAT_MCH_KEY", privPEM)
	t.Setenv("WECHAT_MCH_CERT", certPEM)
	t.Setenv("WECHAT_NOTIFY_URL", "https://example.com/notify")
	p, err = NewProvider("wechat")
	if err != nil || p.Name() != "wechat" {
		t.Fatalf("NewProvider(wechat) = %v, %v", p, err)
	}

	if _, err := NewProvider("bad"); err == nil {
		t.Error("unsupported channel should error")
	}
}

// generateTestKeyPEM 生成测试私钥 PEM。
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

// generateTestCertPEM 生成测试商户证书与私钥 PEM。
func generateTestCertPEM(t *testing.T) (privPEM, certPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	privDER, _ := x509.MarshalPKCS8PrivateKey(key)
	privPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER}))

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}))
	return privPEM, certPEM
}

var _ = filepath.Join
