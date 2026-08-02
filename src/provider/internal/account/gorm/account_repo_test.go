package gorm

import (
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
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
	if err := db.AutoMigrate(&account.Account{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAccountRepo(t *testing.T) {
	db := setupDB(t)
	repo := NewAccountRepo()

	a := &account.Account{ID: "acc_1", CustomerID: "cust_1", Balance: 100}
	if err := repo.Create(db, a); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// 重复主键 → ErrDuplicatedKey
	err := repo.Create(db, &account.Account{ID: "acc_1", CustomerID: "cust_2"})
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Errorf("Create(dup) err = %v, want ErrDuplicatedKey", err)
	}

	got, err := repo.Get(db, "acc_1")
	if err != nil || got.Balance != 100 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := repo.Get(db, "acc_missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrRecordNotFound", err)
	}

	locked, err := repo.GetForUpdate(db, "acc_1")
	if err != nil || locked.ID != "acc_1" {
		t.Fatalf("GetForUpdate = %+v, %v", locked, err)
	}
	if _, err := repo.GetForUpdate(db, "acc_missing"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetForUpdate(missing) err = %v, want ErrRecordNotFound", err)
	}

	a.Balance = 200
	a.UpdatedAt = a.CreatedAt
	if err := repo.Update(db, a); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got, _ = repo.Get(db, "acc_1")
	if got.Balance != 200 {
		t.Errorf("balance = %d, want 200", got.Balance)
	}

	repo.Create(db, &account.Account{ID: "acc_2", CustomerID: "cust_2"})
	list, err := repo.List(db)
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
}

func TestAccountRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewAccountRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.Create(db, &account.Account{ID: "a"}); err == nil {
		t.Error("Create: expected error")
	}
	if _, err := repo.Get(db, "a"); err == nil {
		t.Error("Get: expected error")
	}
	if _, err := repo.GetForUpdate(db, "a"); err == nil {
		t.Error("GetForUpdate: expected error")
	}
	if err := repo.Update(db, &account.Account{ID: "a"}); err == nil {
		t.Error("Update: expected error")
	}
	if _, err := repo.List(db); err == nil {
		t.Error("List: expected error")
	}
}
