package gorm

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
)

func TestBillingRuleRepo(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&billing.BillingRule{}); err != nil {
		t.Fatal(err)
	}
	repo := NewBillingRuleRepo()

	if err := db.Create(&billing.BillingRule{Priority: 2, Kind: "voucher"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&billing.BillingRule{Priority: 1, Kind: "coupon"}).Error; err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(db)
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
	if list[0].Kind != "coupon" || list[1].Kind != "voucher" {
		t.Errorf("order = %+v", list)
	}
}

func TestBillingRuleRepo_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewBillingRuleRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if _, err := repo.List(db); err == nil {
		t.Error("List: expected error")
	}
}
