package gorm

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/ledger"
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
	if err := db.AutoMigrate(&transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTransactionRepo(t *testing.T) {
	db := setupDB(t)
	repo := NewTransactionRepo()

	// Create
	tx := &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 100,
		BalanceAfter: 100, IdempotencyKey: "r1",
	}
	if err := repo.Create(db, tx); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 重复幂等键 → ErrDuplicatedKey
	err := repo.Create(db, &transaction.Transaction{
		AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 1,
		IdempotencyKey: "r1",
	})
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("Create(dup) err = %v, want ErrDuplicatedKey", err)
	}

	// GetByKey
	got, err := repo.GetByKey(db, "r1")
	if err != nil || got.Amount != 100 {
		t.Fatalf("GetByKey = %+v, %v", got, err)
	}
	if _, err := repo.GetByKey(db, "missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetByKey(missing) err = %v, want ErrRecordNotFound", err)
	}

	// ListByAccount / ListAllByAccount / Sum
	repo.Create(db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeConsume, Amount: 30, IdempotencyKey: "c1"})
	repo.Create(db, &transaction.Transaction{AccountID: "acc_1", Type: transaction.TypeRefund, Amount: 20, IdempotencyKey: "f1"})
	list, err := repo.ListByAccount(db, "acc_1", 1, 0)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListByAccount = %d items, %v", len(list), err)
	}
	all, err := repo.ListAllByAccount(db, "acc_1")
	if err != nil || len(all) != 3 {
		t.Fatalf("ListAllByAccount = %d items, %v", len(all), err)
	}
	sum, err := repo.SumByAccount(db, "acc_1")
	if err != nil || sum != 50 {
		t.Fatalf("SumByAccount = %d, %v; want 50 (100 − 30 − 20)", sum, err)
	}
}

// TestSumByAccount_MatchesLedgerBalance 等价性：SQL 聚合（SumByAccount）必须与
// ledger.Balance（余额 = Σ带符号交易的唯一推导规则）结果一致，防两处实现漂移。
func TestSumByAccount_MatchesLedgerBalance(t *testing.T) {
	db := setupDB(t)
	repo := NewTransactionRepo()

	txs := []*transaction.Transaction{
		{AccountID: "acc_1", Type: transaction.TypeRecharge, Amount: 10000, IdempotencyKey: "r1"},
		{AccountID: "acc_1", Type: transaction.TypeConsume, Amount: 3000, IdempotencyKey: "c1"},
		{AccountID: "acc_1", Type: transaction.TypeRefund, Amount: 2000, IdempotencyKey: "f1"},
		{AccountID: "acc_1", Type: transaction.TypeIssue, Amount: 6000, IdempotencyKey: "i1"},
		{AccountID: "acc_1", Type: transaction.TypeRedeem, Amount: 500, IdempotencyKey: "rd1"},
	}
	for _, tx := range txs {
		if err := repo.Create(db, tx); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := repo.SumByAccount(db, "acc_1")
	if err != nil {
		t.Fatal(err)
	}
	all, err := repo.ListAllByAccount(db, "acc_1")
	if err != nil {
		t.Fatal(err)
	}
	contracts := make([]ledger.Transaction, 0, len(all))
	for i := range all {
		contracts = append(contracts, all[i].Contract())
	}
	if got := ledger.Balance(contracts); got != sum {
		t.Errorf("ledger.Balance = %d, SumByAccount = %d；两处余额推导必须一致", got, sum)
	}
}

func TestTransactionRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewTransactionRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.Create(db, &transaction.Transaction{IdempotencyKey: "k"}); err == nil {
		t.Error("Create: expected error")
	}
	if _, err := repo.GetByKey(db, "k"); err == nil {
		t.Error("GetByKey: expected error")
	}
	if _, err := repo.ListByAccount(db, "a", 10, 0); err == nil {
		t.Error("ListByAccount: expected error")
	}
	if _, err := repo.ListAllByAccount(db, "a"); err == nil {
		t.Error("ListAllByAccount: expected error")
	}
	if _, err := repo.SumByAccount(db, "a"); err == nil {
		t.Error("SumByAccount: expected error")
	}
}
