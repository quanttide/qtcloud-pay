package gorm

import (
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func TestPricingRuleSetRepo(t *testing.T) {
	db := setupDB(t)
	if err := db.AutoMigrate(&voucher.PricingRuleSet{}); err != nil {
		t.Fatal(err)
	}
	repo := NewVoucherRepo()

	ruleSet := &voucher.PricingRuleSet{ID: "qtclass", Source: "source-a", Version: "v1", Payload: `{"ok":true}`}
	if err := repo.UpsertRuleSet(db, ruleSet); err != nil {
		t.Fatalf("UpsertRuleSet: %v", err)
	}
	ruleSet.Source = "source-b"
	ruleSet.Version = "v2"
	if err := repo.UpsertRuleSet(db, ruleSet); err != nil {
		t.Fatalf("UpsertRuleSet(update): %v", err)
	}
	got, err := repo.GetRuleSet(db, "qtclass")
	if err != nil || got.Source != "source-b" || got.Version != "v2" {
		t.Fatalf("GetRuleSet = %+v, %v", got, err)
	}
	list, err := repo.ListRuleSets(db)
	if err != nil || len(list) != 1 {
		t.Fatalf("ListRuleSets = %d, %v", len(list), err)
	}
}

func TestPricingRuleSetRepo_DBError(t *testing.T) {
	db := setupDB(t)
	repo := NewVoucherRepo()
	sqlDB, _ := db.DB()
	sqlDB.Close()

	if err := repo.UpsertRuleSet(db, &voucher.PricingRuleSet{ID: "qtclass"}); err == nil {
		t.Error("UpsertRuleSet: expected error")
	}
	if _, err := repo.GetRuleSet(db, "qtclass"); err == nil {
		t.Error("GetRuleSet: expected error")
	}
	if _, err := repo.ListRuleSets(db); err == nil {
		t.Error("ListRuleSets: expected error")
	}
}
