package main

import (
	"bytes"
	"context"
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

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")),
		&gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&account.Account{}, &transaction.Transaction{},
		&coupon.Coupon{}, &voucher.Voucher{},
		&order.Order{}, &billing.BillingRule{},
	); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOpenDB(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "qtcloud.db"))
	db, err := openDB()
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	for _, m := range []any{
		&account.Account{}, &transaction.Transaction{},
		&coupon.Coupon{}, &voucher.Voucher{},
		&order.Order{}, &billing.BillingRule{},
	} {
		if !db.Migrator().HasTable(m) {
			t.Errorf("table missing: %T", m)
		}
	}
}

func TestOpenDB_PostgresInvalidDSN(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "://bad")
	if _, err := openDB(); err == nil {
		t.Fatal("expected error for invalid postgres dsn")
	}
}

func TestOpenDB_PostgresMigrateError(t *testing.T) {
	// gorm.Open 不发起连接，合法 DSN 可走到 AutoMigrate（无服务器时迁移失败）
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable")
	if _, err := openDB(); err == nil {
		t.Fatal("expected migrate error without postgres server")
	}
}

func TestBuildMux_LedgerRoutes(t *testing.T) {
	db := openTestDB(t)
	mux, err := buildMux(db, "")
	if err != nil {
		t.Fatalf("buildMux: %v", err)
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

func TestBuildMux_AlipayChannel(t *testing.T) {
	db := openTestDB(t)
	t.Setenv("ALIPAY_APP_ID", "2021000000000001")
	t.Setenv("ALIPAY_PRIVATE_KEY", generateTestKeyPEM(t))

	mux, err := buildMux(db, "alipay")
	if err != nil {
		t.Fatalf("buildMux: %v", err)
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
	db := openTestDB(t)
	if _, err := buildMux(db, "unionpay"); err == nil {
		t.Fatal("expected error for unsupported channel")
	}
}

func TestNewProvider(t *testing.T) {
	// alipay 成功
	t.Setenv("ALIPAY_APP_ID", "2021000000000001")
	t.Setenv("ALIPAY_PRIVATE_KEY", generateTestKeyPEM(t))
	p, err := newProvider("alipay")
	if err != nil || p.Name() != "alipay" {
		t.Fatalf("newProvider(alipay) = %v, %v", p, err)
	}

	// wechat 成功（需要商户证书与私钥）
	privPEM, certPEM := generateTestCertPEM(t)
	t.Setenv("WECHAT_APP_ID", "wx123")
	t.Setenv("WECHAT_MCH_ID", "mch123")
	t.Setenv("WECHAT_API_V3_KEY", "test-api-v3-key-1234567890abcd")
	t.Setenv("WECHAT_MCH_KEY", privPEM)
	t.Setenv("WECHAT_MCH_CERT", certPEM)
	t.Setenv("WECHAT_NOTIFY_URL", "https://example.com/notify")
	p, err = newProvider("wechat")
	if err != nil || p.Name() != "wechat" {
		t.Fatalf("newProvider(wechat) = %v, %v", p, err)
	}

	if _, err := newProvider("bad"); err == nil {
		t.Error("unsupported channel should error")
	}
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

func TestOpenDB_DefaultDSN(t *testing.T) {
	// 不设置 DB_SQLITE_DSN → 使用默认 qtcloud.db（在临时目录中）
	t.Chdir(t.TempDir())
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", "")
	db, err := openDB()
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	if !db.Migrator().HasTable(&account.Account{}) {
		t.Error("accounts table missing")
	}
}

func TestRun_OpenDBError(t *testing.T) {
	t.Setenv("DB_DRIVER", "postgres")
	t.Setenv("DATABASE_URL", "://bad")
	if err := run(context.Background(), "127.0.0.1:0", ""); err == nil {
		t.Fatal("expected openDB error")
	}
}

func TestRun_BuildMuxError(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))
	if err := run(context.Background(), "127.0.0.1:0", "unionpay"); err == nil {
		t.Fatal("expected buildMux error")
	}
}

func TestRun_InvalidAddr(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))
	err := run(context.Background(), "bad addr", "")
	if err == nil {
		t.Fatal("expected error for invalid addr")
	}
}

func TestRun_ServeAndShutdown(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_SQLITE_DSN", filepath.Join(t.TempDir(), "run.db"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, "127.0.0.1:0", "")
	}()

	// 等待服务启动（run 内部监听 127.0.0.1:0，端口未知，仅验证启动后随取消正常退出）
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not exit after cancel")
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
