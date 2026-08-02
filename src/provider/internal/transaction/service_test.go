package transaction_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
)

// setupDB 创建内存 SQLite 测试库（单连接保证 :memory: 一致）。
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
	if err := db.AutoMigrate(&transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newTestService(db *gorm.DB) *transaction.Service {
	return transaction.NewService(transactiongorm.NewTransactionRepo())
}

// closeDB 关闭底层连接，使后续操作报错。
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

func TestAppend(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	tx := &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 100,
		BalanceAfter: 100, IdempotencyKey: "recharge:v1",
	}
	if err := svc.Append(ctx, db, tx); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if tx.ID == 0 {
		t.Error("ID should be assigned")
	}

	// 幂等键已存在 → ErrDuplicateKey
	err := svc.Append(ctx, db, &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 100,
		IdempotencyKey: "recharge:v1",
	})
	if !errors.Is(err, transaction.ErrDuplicateKey) {
		t.Errorf("err = %v, want ErrDuplicateKey", err)
	}
}

func TestAppend_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	err := svc.Append(ctx, db, &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 1,
		IdempotencyKey: "k",
	})
	if err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestExists(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	got, err := svc.Exists(ctx, db, "recharge:v1")
	if err != nil || got {
		t.Errorf("Exists(missing) = %v, %v; want false, nil", got, err)
	}

	svc.Append(ctx, db, &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 1,
		IdempotencyKey: "recharge:v1",
	})
	got, err = svc.Exists(ctx, db, "recharge:v1")
	if err != nil || !got {
		t.Errorf("Exists(present) = %v, %v; want true, nil", got, err)
	}
}

func TestExists_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	if _, err := svc.Exists(ctx, db, "k"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestGetByKey(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	if _, err := svc.GetByKey(ctx, db, "missing"); !errors.Is(err, transaction.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}

	svc.Append(ctx, db, &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 5,
		IdempotencyKey: "k1",
	})
	got, err := svc.GetByKey(ctx, db, "k1")
	if err != nil || got.Amount != 5 {
		t.Errorf("GetByKey = %+v, %v", got, err)
	}
}

func TestGetByKey_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	if _, err := svc.GetByKey(ctx, db, "k"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestList(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		svc.Append(ctx, db, &transaction.Transaction{
			AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: int64(i + 1),
			IdempotencyKey: "k1_" + strconvItoa(i),
		})
	}
	svc.Append(ctx, db, &transaction.Transaction{
		AccountID: "acc_2", Type: transaction.TypeRecharge, Amount: 9,
		IdempotencyKey: "k2",
	})

	all, err := svc.List(ctx, db, "acc_1", 10, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("List(all) = %d items, %v", len(all), err)
	}
	if all[0].Amount != 3 { // id 倒序
		t.Errorf("first item amount = %d, want 3", all[0].Amount)
	}

	page, err := svc.List(ctx, db, "acc_1", 1, 1)
	if err != nil || len(page) != 1 {
		t.Fatalf("List(page) = %d items, %v", len(page), err)
	}
	if page[0].Amount != 2 {
		t.Errorf("page item amount = %d, want 2", page[0].Amount)
	}

	empty, err := svc.List(ctx, db, "acc_none", 10, 0)
	if err != nil || len(empty) != 0 {
		t.Errorf("List(empty) = %d items, %v", len(empty), err)
	}
}

func TestList_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	if _, err := svc.List(ctx, db, "acc_1", 10, 0); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestListAll(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		svc.Append(ctx, db, &transaction.Transaction{
			AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: int64(i + 1),
			IdempotencyKey: "k_" + strconvItoa(i),
		})
	}
	all, err := svc.ListAll(ctx, db, "acc_1")
	if err != nil || len(all) != 3 {
		t.Fatalf("ListAll = %d items, %v", len(all), err)
	}
	if all[0].Amount != 1 || all[2].Amount != 3 { // id 升序
		t.Errorf("order wrong: %v", all)
	}
}

func TestListAll_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	if _, err := svc.ListAll(ctx, db, "acc_1"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestSum(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	svc.Append(ctx, db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 100, IdempotencyKey: "r1"})
	svc.Append(ctx, db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 50, IdempotencyKey: "r2"})
	svc.Append(ctx, db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeConsume, Amount: 30, IdempotencyKey: "c1"})
	// 发券/核销不参与余额求和
	svc.Append(ctx, db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeIssue, Amount: 999, IdempotencyKey: "i1"})
	svc.Append(ctx, db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeRedeem, Amount: 999, IdempotencyKey: "rd1"})

	sum, err := svc.Sum(ctx, db, "acc_1")
	if err != nil || sum != 120 {
		t.Errorf("Sum = %d, %v; want 120", sum, err)
	}

	other, err := svc.Sum(ctx, db, "acc_other")
	if err != nil || other != 0 {
		t.Errorf("Sum(empty) = %d, %v; want 0", other, err)
	}
}

func TestSum_DBError(t *testing.T) {
	db := setupDB(t)
	svc := newTestService(db)
	ctx := context.Background()

	closeDB(t, db)
	if _, err := svc.Sum(ctx, db, "acc_1"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestSignedAmountAndAffectsBalance(t *testing.T) {
	cases := []struct {
		typ     string
		amount  int64
		signed  int64
		affects bool
	}{
		{transaction.TypeRecharge, 100, 100, true},
		{transaction.TypeConsume, 30, -30, true},
		{transaction.TypeIssue, 50, 0, false},
		{transaction.TypeRedeem, 20, 0, false},
	}
	for _, c := range cases {
		tx := &transaction.Transaction{Type: c.typ, Amount: c.amount}
		if got := tx.SignedAmount(); got != c.signed {
			t.Errorf("%s SignedAmount = %d, want %d", c.typ, got, c.signed)
		}
		if got := tx.AffectsBalance(); got != c.affects {
			t.Errorf("%s AffectsBalance = %v, want %v", c.typ, got, c.affects)
		}
	}
}

// strconvItoa 小工具（避免在测试中额外导入 strconv 的别名需求）。
func strconvItoa(i int) string {
	return string(rune('0' + i))
}
