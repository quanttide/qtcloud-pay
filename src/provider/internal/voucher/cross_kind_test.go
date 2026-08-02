package voucher_test

import (
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	coupongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/coupon/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
)

// TestIssue_CrossKindBatchNo 复现：优惠券与代金券同批次号时，发券交易幂等键冲突。
func TestIssue_CrossKindBatchNo(t *testing.T) {
	// 需要同时迁移优惠券与代金券表
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&coupon.Coupon{}, &voucher.Voucher{}, &transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())

	couponSvc := coupon.NewService(db, coupongorm.NewCouponRepo(), txSvc)
	voucherSvc := voucher.NewService(db, vouchergorm.NewVoucherRepo(), txSvc)

	expires := time.Now().Add(24 * time.Hour)
	// 先发优惠券批次 b1
	if err := couponSvc.Issue(ctx, &coupon.IssueRequest{
		AccountID: "acc_1", Type: coupon.TypeFullReduction,
		Threshold: 100, Amount: 10, Scope: coupon.ScopeAll,
		ExpiresAt: expires, Count: 1, BatchNo: "b1",
	}); err != nil {
		t.Fatal(err)
	}
	// 再发代金券批次 b1 —— 幂等键冲突会被静默跳过
	if err := voucherSvc.Issue(ctx, &voucher.IssueRequest{
		AccountID: "acc_1", Amount: 100, Scope: voucher.ScopeAll,
		ExpiresAt: expires, Count: 1, BatchNo: "b1",
	}); err != nil {
		t.Fatal(err)
	}

	// 两张都应存在
	coupons, _ := couponSvc.List(ctx, "acc_1")
	if len(coupons) != 1 {
		t.Errorf("coupons = %d, want 1", len(coupons))
	}
	vouchers, _ := voucherSvc.List(ctx, "acc_1")
	if len(vouchers) != 1 {
		t.Errorf("vouchers = %d, want 1（当前 bug：代金券被静默跳过）", len(vouchers))
	}
}
