package account_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	accountgorm "github.com/quanttide/qtcloud-pay/src/provider/internal/account/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
)

func setupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&account.Account{}, &transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newService(t *testing.T) (*account.Service, *gorm.DB) {
	t.Helper()
	db := setupDB(t)
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	svc := account.NewService(db, accountgorm.NewAccountRepo(), txSvc)
	return svc, db
}

func TestCreate(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	acc, err := svc.Create(ctx, "cust_1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !strings.HasPrefix(acc.ID, "acc_") || acc.ID == "acc_" {
		t.Errorf("ID = %q, want acc_ prefix with suffix", acc.ID)
	}
	if acc.CustomerID != "cust_1" || acc.Balance != 0 {
		t.Errorf("account = %+v", acc)
	}

	if _, err := svc.Create(ctx, ""); err == nil {
		t.Error("Create(empty customer) should error")
	}
}

// stubRepo 仅覆盖 Create 的错误注入，其余方法走内嵌接口（未使用）。
type stubRepo struct {
	account.Repository
	createErr error
}

func (s *stubRepo) Create(db *gorm.DB, a *account.Account) error { return s.createErr }

func TestCreate_Exists(t *testing.T) {
	db := setupDB(t)
	svc := account.NewService(db, &stubRepo{createErr: gorm.ErrDuplicatedKey},
		transaction.NewService(transactiongorm.NewTransactionRepo()))
	_, err := svc.Create(context.Background(), "cust_1")
	if !errors.Is(err, account.ErrExists) {
		t.Errorf("err = %v, want ErrExists", err)
	}
}

func TestCreate_DBError(t *testing.T) {
	svc, db := newService(t)
	closeDB(t, db)
	if _, err := svc.Create(context.Background(), "cust_1"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestGet(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	acc, _ := svc.Create(ctx, "cust_1")
	got, err := svc.Get(ctx, acc.ID)
	if err != nil || got.CustomerID != "cust_1" {
		t.Fatalf("Get = %+v, %v", got, err)
	}

	if _, err := svc.Get(ctx, "acc_missing"); !errors.Is(err, account.ErrNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrNotFound", err)
	}
}

func TestList(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	svc.Create(ctx, "cust_1")
	svc.Create(ctx, "cust_2")
	list, err := svc.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
}

func TestLockAndSave(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	acc, _ := svc.Create(ctx, "cust_1")
	locked, err := svc.Lock(ctx, db, acc.ID)
	if err != nil || locked.Balance != 0 {
		t.Fatalf("Lock = %+v, %v", locked, err)
	}

	if _, err := svc.Lock(ctx, db, "acc_missing"); !errors.Is(err, account.ErrNotFound) {
		t.Errorf("Lock(missing) err = %v, want ErrNotFound", err)
	}

	locked.Balance = 500
	if err := svc.Save(ctx, db, locked); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, _ := svc.Get(ctx, acc.ID)
	if got.Balance != 500 {
		t.Errorf("balance = %d, want 500", got.Balance)
	}
}

func TestRecharge(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	acc, _ := svc.Create(ctx, "cust_1")
	if err := svc.Recharge(ctx, acc.ID, 10000, "voucher-001", "对公打款"); err != nil {
		t.Fatalf("Recharge: %v", err)
	}
	got, _ := svc.Get(ctx, acc.ID)
	if got.Balance != 10000 {
		t.Errorf("balance = %d, want 10000", got.Balance)
	}
	txs, err := svc.ListTransactions(ctx, acc.ID, 10, 0)
	if err != nil || len(txs) != 1 {
		t.Fatalf("transactions = %d, %v", len(txs), err)
	}
	if txs[0].Type != transaction.TypeRecharge || txs[0].Amount != 10000 || txs[0].BalanceAfter != 10000 {
		t.Errorf("tx = %+v", txs[0])
	}

	// 幂等：同凭证号重复提交不重复入账
	if err := svc.Recharge(ctx, acc.ID, 10000, "voucher-001", ""); err != nil {
		t.Fatalf("Recharge(dup): %v", err)
	}
	got, _ = svc.Get(ctx, acc.ID)
	if got.Balance != 10000 {
		t.Errorf("balance after dup = %d, want 10000", got.Balance)
	}
	txs, _ = svc.ListTransactions(ctx, acc.ID, 10, 0)
	if len(txs) != 1 {
		t.Errorf("transactions after dup = %d, want 1", len(txs))
	}
}

func TestRecharge_Validation(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	acc, _ := svc.Create(ctx, "cust_1")

	if err := svc.Recharge(ctx, acc.ID, 0, "v1", ""); !errors.Is(err, account.ErrInvalidAmount) {
		t.Errorf("amount=0 err = %v, want ErrInvalidAmount", err)
	}
	if err := svc.Recharge(ctx, acc.ID, -5, "v1", ""); !errors.Is(err, account.ErrInvalidAmount) {
		t.Errorf("amount<0 err = %v, want ErrInvalidAmount", err)
	}
	if err := svc.Recharge(ctx, acc.ID, 100, "", ""); !errors.Is(err, account.ErrInvalidRecharge) {
		t.Errorf("empty voucher err = %v, want ErrInvalidRecharge", err)
	}
	if err := svc.Recharge(ctx, "", 100, "v1", ""); err == nil {
		t.Error("empty account should error")
	}
}

func TestRecharge_AccountNotFound(t *testing.T) {
	svc, _ := newService(t)
	err := svc.Recharge(context.Background(), "acc_missing", 100, "v1", "")
	if !errors.Is(err, account.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// stubTxRepo 注入交易账本的错误。
type stubTxRepo struct {
	transaction.Repository
	getErr    error
	createErr error
}

func (s *stubTxRepo) GetByKey(db *gorm.DB, key string) (*transaction.Transaction, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubTxRepo) Create(db *gorm.DB, t *transaction.Transaction) error {
	return s.createErr
}

func TestRecharge_TxServiceError(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	// Exists 查询失败 → 错误传播
	txSvcErr := transaction.NewService(&stubTxRepo{getErr: errors.New("db down")})
	svcErr := account.NewService(db, accountgorm.NewAccountRepo(), txSvcErr)
	if err := svcErr.Recharge(ctx, "acc_1", 100, "v1", ""); err == nil {
		t.Error("Exists error should propagate")
	}

	// 写交易失败 → 错误传播（余额一并回滚）
	txSvcAppend := transaction.NewService(&stubTxRepo{createErr: errors.New("write failed")})
	svcAppend := account.NewService(db, accountgorm.NewAccountRepo(), txSvcAppend)
	acc, err := svcAppend.Create(ctx, "cust_1")
	if err != nil {
		t.Fatal(err)
	}
	if err := svcAppend.Recharge(ctx, acc.ID, 100, "v2", ""); err == nil {
		t.Fatal("Append error should propagate")
	}
	got, _ := svcAppend.Get(ctx, acc.ID)
	if got.Balance != 0 {
		t.Errorf("balance = %d, want 0 (rolled back)", got.Balance)
	}
}

func TestListTransactions(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	acc, _ := svc.Create(ctx, "cust_1")
	svc.Recharge(ctx, acc.ID, 100, "v1", "")
	svc.Recharge(ctx, acc.ID, 200, "v2", "")

	txs, err := svc.ListTransactions(ctx, acc.ID, 1, 0)
	if err != nil || len(txs) != 1 {
		t.Fatalf("ListTransactions = %d items, %v", len(txs), err)
	}
	if txs[0].Amount != 200 { // 倒序
		t.Errorf("amount = %d, want 200", txs[0].Amount)
	}
}

func closeDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}
