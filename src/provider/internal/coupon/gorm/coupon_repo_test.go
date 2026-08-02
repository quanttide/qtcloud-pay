package gorm

import (
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
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
	if err := db.AutoMigrate(&coupon.Coupon{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestCouponRepo(t *testing.T) {
	db := setupDB(t)
	repo := NewCouponRepo()
	now := time.Now()

	coupons := []*coupon.Coupon{
		{ID: 1, AccountID: "acc_1", BatchNo: "b1", Type: coupon.TypeDiscount, Rate: 90,
			Scope: coupon.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: coupon.StatusIssued},
		{ID: 2, AccountID: "acc_1", BatchNo: "b1", Type: coupon.TypeDiscount, Rate: 90,
			Scope: coupon.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: coupon.StatusIssued},
	}
	if err := repo.CreateBatch(db, coupons); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}
	// 同一批次多张券共用 batch_no 是合法的（幂等由发券交易幂等键保证）
	// 重复主键 → ErrDuplicatedKey
	err := repo.CreateBatch(db, []*coupon.Coupon{{
		ID: 1, AccountID: "acc_1", BatchNo: "b2", Type: coupon.TypeDiscount, Rate: 90,
		Scope: coupon.ScopeAll, ExpiresAt: now.Add(time.Hour), Status: coupon.StatusIssued,
	}})
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("CreateBatch(dup) err = %v, want ErrDuplicatedKey", err)
	}

	got, err := repo.Get(db, coupons[0].ID)
	if err != nil || got.Rate != 90 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := repo.Get(db, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrRecordNotFound", err)
	}

	locked, err := repo.GetForUpdate(db, coupons[0].ID)
	if err != nil || locked.ID != coupons[0].ID {
		t.Fatalf("GetForUpdate = %+v, %v", locked, err)
	}
	if _, err := repo.GetForUpdate(db, 99999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetForUpdate(missing) err = %v, want ErrRecordNotFound", err)
	}

	usedAt := now
	coupons[0].Status, coupons[0].UsedAt, coupons[0].OrderID = coupon.StatusUsed, &usedAt, "ORD-1"
	if err := repo.Update(db, coupons[0]); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(db, coupons[0].ID)
	if got.Status != coupon.StatusUsed || got.OrderID != "ORD-1" {
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
	n, _ = repo.CountByBatch(db, "b-missing")
	if n != 0 {
		t.Errorf("CountByBatch(missing) = %d, want 0", n)
	}
}

func TestCouponRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewCouponRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.CreateBatch(db, []*coupon.Coupon{{}}); err == nil {
		t.Error("CreateBatch: expected error")
	}
	if _, err := repo.Get(db, 1); err == nil {
		t.Error("Get: expected error")
	}
	if _, err := repo.GetForUpdate(db, 1); err == nil {
		t.Error("GetForUpdate: expected error")
	}
	if err := repo.Update(db, &coupon.Coupon{ID: 1}); err == nil {
		t.Error("Update: expected error")
	}
	if _, err := repo.ListByAccount(db, "a"); err == nil {
		t.Error("ListByAccount: expected error")
	}
	if _, err := repo.CountByBatch(db, "b"); err == nil {
		t.Error("CountByBatch: expected error")
	}
}
