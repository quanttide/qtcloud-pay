package app

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/security"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transfer"
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
		&order.Order{}, &billing.BillingRule{}, &transfer.VoucherTransfer{},
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

func TestBuildHandler_PermissionMatrix(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	secStore := security.NewStore(db)
	if err := secStore.SetUserRoles(context.Background(), "viewer-user", []string{security.RoleViewer}); err != nil {
		t.Fatal(err)
	}
	if err := secStore.SetUserRoles(context.Background(), "operator-user", []string{security.RoleOperator}); err != nil {
		t.Fatal(err)
	}
	verifier, err := security.NewTokenVerifier("test-secret-key", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	viewerToken, err := verifier.SignServiceToken("viewer-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	operatorToken, err := verifier.SignServiceToken("operator-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := BuildHandler(db, "", SecurityConfig{SecretKey: "test-secret-key", AdminToken: "admin-token"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	assertStatus := func(name, method, path, token, body string, want int) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s status = %d, want %d", name, resp.StatusCode, want)
		}
	}

	assertStatus("anonymous read", http.MethodGet, "/accounts/missing", "", "", http.StatusUnauthorized)
	assertStatus("viewer write", http.MethodPost, "/accounts", viewerToken, `{"customer_id":"cust_viewer"}`, http.StatusForbidden)
	assertStatus("operator write", http.MethodPost, "/accounts", operatorToken, `{"customer_id":"cust_operator"}`, http.StatusCreated)
	assertStatus("viewer read", http.MethodGet, "/accounts/missing", viewerToken, "", http.StatusNotFound)
}

func TestBuildHandler_AdminTokenBypassAndBadSecret(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BuildHandler(db, "", SecurityConfig{}); err == nil {
		t.Fatal("expected missing SECRET_KEY error")
	}
	verifier, err := security.NewTokenVerifier("right-secret", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	token, err := verifier.SignServiceToken("operator-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := BuildHandler(db, "", SecurityConfig{SecretKey: "wrong-secret", AdminToken: "admin-token"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/accounts", bytes.NewReader([]byte(`{"customer_id":"cust_bad"}`)))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad secret token status = %d, want 401", resp.StatusCode)
	}

	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/accounts", bytes.NewReader([]byte(`{"customer_id":"cust_admin"}`)))
	req2.Header.Set("X-Admin-Token", "admin-token")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusCreated {
		t.Fatalf("admin token status = %d, want 201", resp2.StatusCode)
	}
}

func TestBuildHandler_ServiceTokenUsesSecretKey(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := BuildHandler(db, "", SecurityConfig{SecretKey: "real-secret-key", AdminToken: "admin-token"})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/admin/security/service-tokens", bytes.NewReader([]byte(`{"subject":"svc-no-role","ttl_seconds":300}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "admin-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("service token status = %d, want 201", resp.StatusCode)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/accounts/missing", nil)
	req2.Header.Set("Authorization", "Bearer "+body.Token)
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("service token no role status = %d, want 403", resp2.StatusCode)
	}
}

func TestBuildHandler_AuthTokenOwnerRead(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := BuildHandler(db, "", SecurityConfig{
		SecretKey:     "test-secret-key",
		AdminToken:    "admin-token",
		AuthPublicJWK: testRSAJWK(&key.PublicKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	ownerAccount := createAccountWithAdmin(t, ts.URL, "auth-user-1")
	otherAccount := createAccountWithAdmin(t, ts.URL, "auth-user-2")
	tr := transfer.VoucherTransfer{
		FromAccountID:       ownerAccount,
		ToAccountID:         otherAccount,
		SourceTransactionID: 1,
		Amount:              100,
		TaxRateBP:           0,
		Status:              transfer.StatusPending,
		IdempotencyKey:      "owner-read-transfer",
	}
	if err := db.Create(&tr).Error; err != nil {
		t.Fatal(err)
	}
	ownerToken := signRS256TestToken(t, key, "auth-user-1")

	assertStatus := func(name, method, path, token, body string, want int) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader([]byte(body)))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Fatalf("%s status = %d, want %d", name, resp.StatusCode, want)
		}
	}

	assertStatus("anonymous owner read", http.MethodGet, "/customers/auth-user-1/account", "", "", http.StatusUnauthorized)
	assertStatus("owner customer account", http.MethodGet, "/customers/auth-user-1/account", ownerToken, "", http.StatusOK)
	assertStatus("other customer account", http.MethodGet, "/customers/auth-user-2/account", ownerToken, "", http.StatusForbidden)
	assertStatus("owner account detail", http.MethodGet, "/accounts/"+ownerAccount, ownerToken, "", http.StatusOK)
	assertStatus("owner transactions", http.MethodGet, "/accounts/"+ownerAccount+"/transactions", ownerToken, "", http.StatusOK)
	assertStatus("owner statement", http.MethodGet, "/accounts/"+ownerAccount+"/statement", ownerToken, "", http.StatusOK)
	assertStatus("owner transfer detail", http.MethodGet, "/transfers/"+strconv.FormatInt(tr.ID, 10), ownerToken, "", http.StatusOK)
	assertStatus("owner transfer list", http.MethodGet, "/transfers?account_id="+ownerAccount, ownerToken, "", http.StatusOK)
	assertStatus("other transactions", http.MethodGet, "/accounts/"+otherAccount+"/transactions", ownerToken, "", http.StatusForbidden)
	assertStatus("other transfer list", http.MethodGet, "/transfers?account_id="+otherAccount, ownerToken, "", http.StatusForbidden)
	assertStatus("owner write still forbidden", http.MethodPost, "/accounts/"+ownerAccount+"/recharges", ownerToken, `{"amount":{"amount":1,"currency":"CNY"},"voucher_no":"owner-write-denied"}`, http.StatusForbidden)
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

func TestBuildMux_HealthRoute(t *testing.T) {
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

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/health status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
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

func createAccountWithAdmin(t *testing.T, baseURL, customerID string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"customer_id": customerID})
	req, err := http.NewRequest(http.MethodPost, baseURL+"/accounts", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Admin-Token", "admin-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create account status = %d, want 201", resp.StatusCode)
	}
	var got struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ID == "" {
		t.Fatal("created account missing id")
	}
	return got.ID
}

func testRSAJWK(pub *rsa.PublicKey) string {
	return `{"kty":"RSA","alg":"RS256","n":"` + base64.RawURLEncoding.EncodeToString(pub.N.Bytes()) + `","e":"` + base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()) + `"}`
}

func signRS256TestToken(t *testing.T, key *rsa.PrivateKey, subject string) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload, err := json.Marshal(map[string]any{
		"sub": subject,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	body := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	sum := sha256.Sum256([]byte(body))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return body + "." + base64.RawURLEncoding.EncodeToString(sig)
}

var _ = filepath.Join
