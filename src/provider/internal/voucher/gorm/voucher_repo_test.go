package gorm

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
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
	if err := db.AutoMigrate(&voucher.Voucher{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestVoucherRepo(t *testing.T) {
	db := setupDB(t)
	repo := NewVoucherRepo()
	now := time.Now()

	vouchers := []*voucher.Voucher{
		{ID: 1, AccountID: "acc_1", BatchNo: "b1", Amount: 100,
			Scope: voucher.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: voucher.StatusIssued},
		{ID: 2, AccountID: "acc_1", BatchNo: "b1", Amount: 100,
			Scope: voucher.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: voucher.StatusIssued},
	}
	if err := repo.CreateBatch(db, vouchers); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	// 重复主键 → ErrDuplicatedKey
	err := repo.CreateBatch(db, []*voucher.Voucher{{
		ID: 1, AccountID: "acc_1", BatchNo: "b2", Amount: 100,
		Scope: voucher.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: voucher.StatusIssued,
	}})
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("CreateBatch(dup) err = %v, want ErrDuplicatedKey", err)
	}

	got, err := repo.Get(db, 1)
	if err != nil || got.Amount != 100 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := repo.Get(db, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrRecordNotFound", err)
	}

	locked, err := repo.GetForUpdate(db, 1)
	if err != nil || locked.ID != 1 {
		t.Fatalf("GetForUpdate = %+v, %v", locked, err)
	}

	usedAt := now
	vouchers[0].Status, vouchers[0].UsedAt, vouchers[0].OrderID = voucher.StatusUsed, &usedAt, "ORD-1"
	if err := repo.Update(db, vouchers[0]); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(db, 1)
	if got.Status != voucher.StatusUsed || got.OrderID != "ORD-1" {
		t.Errorf("updated = %+v", got)
	}

	list, err := repo.ListByAccount(db, "acc_1")
	if err != nil || len(list) != 2 {
		t.Fatalf("ListByAccount = %d items, %v", len(list), err)
	}

	n, err := repo.CountByBatch(db, "b1")
	if err != nil || n != 2 {
		t.Fatalf("CountByBatch = %d, %v; want 2", n, err)
	}
}

func TestVoucherRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewVoucherRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.CreateBatch(db, []*voucher.Voucher{{}}); err == nil {
		t.Error("CreateBatch: expected error")
	}
	if _, err := repo.Get(db, 1); err == nil {
		t.Error("Get: expected error")
	}
	if _, err := repo.GetForUpdate(db, 1); err == nil {
		t.Error("GetForUpdate: expected error")
	}
	if err := repo.Update(db, &voucher.Voucher{ID: 1}); err == nil {
		t.Error("Update: expected error")
	}
	if _, err := repo.ListByAccount(db, "a"); err == nil {
		t.Error("ListByAccount: expected error")
	}
	if _, err := repo.CountByBatch(db, "b"); err == nil {
		t.Error("CountByBatch: expected error")
	}
}
