package gorm

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
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
	if err := db.AutoMigrate(&order.Order{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOrderRepo(t *testing.T) {
	db := setupDB(t)
	repo := NewOrderRepo()
	now := time.Now()

	detail, _ := json.Marshal([]map[string]any{{"kind": "balance", "amount": 100}})
	o := &order.Order{
		ID: "ORD-1", CustomerID: "cust_1", AccountID: "acc_1",
		Scope: "course", Amount: 100, Status: order.StatusSettled,
		SettleDetail: detail, SettledAt: &now,
	}
	if err := repo.Create(db, o); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 重复主键 → ErrDuplicatedKey
	err := repo.Create(db, &order.Order{ID: "ORD-1", Amount: 100, Status: order.StatusSettled})
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("Create(dup) err = %v, want ErrDuplicatedKey", err)
	}

	got, err := repo.Get(db, "ORD-1")
	if err != nil || got.ID != "ORD-1" || got.Status != order.StatusSettled {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if string(got.SettleDetail) != string(detail) {
		t.Errorf("detail = %s", got.SettleDetail)
	}
	if _, err := repo.Get(db, "ORD-MISSING"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrRecordNotFound", err)
	}
}

func TestOrderRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewOrderRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.Create(db, &order.Order{ID: "a"}); err == nil {
		t.Error("Create: expected error")
	}
	if _, err := repo.Get(db, "a"); err == nil {
		t.Error("Get: expected error")
	}
}
