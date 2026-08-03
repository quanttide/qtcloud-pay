package billing_test

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	billinggorm "github.com/quanttide/qtcloud-pay/src/provider/internal/billing/gorm"
)

// Calculate 的纯计算测试已随实现提炼至工具库 pkg/billing；
// 本包测试聚焦服务端规则表（gorm）部分。

func TestRules(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&billing.BillingRule{}); err != nil {
		t.Fatal(err)
	}
	svc := billing.NewService(billinggorm.NewBillingRuleRepo())

	// 空表
	rules, err := svc.Rules(context.Background(), db)
	if err != nil || len(rules) != 0 {
		t.Fatalf("Rules(empty) = %d, %v", len(rules), err)
	}

	// 按优先级排序
	db.Create(&billing.BillingRule{Priority: 2, Kind: "voucher"})
	db.Create(&billing.BillingRule{Priority: 1, Kind: "coupon"})
	rules, err = svc.Rules(context.Background(), db)
	if err != nil || len(rules) != 2 {
		t.Fatalf("Rules = %d, %v", len(rules), err)
	}
	if rules[0].Kind != "coupon" || rules[1].Kind != "voucher" {
		t.Errorf("rules order = %+v", rules)
	}
}

func TestRules_DBError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	svc := billing.NewService(billinggorm.NewBillingRuleRepo())
	sqlDB, _ := db.DB()
	sqlDB.Close()
	if _, err := svc.Rules(context.Background(), db); err == nil {
		t.Fatal("expected error on closed db")
	}
}
