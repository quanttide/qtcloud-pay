package transfer_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	accountgorm "github.com/quanttide/qtcloud-pay/src/provider/internal/account/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transfer"
	transfergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transfer/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
)

type env struct {
	db          *gorm.DB
	accountSvc  *account.Service
	voucherSvc  *voucher.Service
	transferSvc *transfer.Service
	txSvc       *transaction.Service
}

func setupEnv(t *testing.T) *env {
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
	if err := db.AutoMigrate(&account.Account{}, &transaction.Transaction{}, &voucher.Voucher{}, &transfer.VoucherTransfer{}); err != nil {
		t.Fatal(err)
	}
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	accountSvc := account.NewService(db, accountgorm.NewAccountRepo(), txSvc)
	voucherSvc := voucher.NewService(db, vouchergorm.NewVoucherRepo(), txSvc)
	transferSvc := transfer.NewService(db, transfergorm.NewTransferRepo(), accountSvc, voucherSvc, txSvc)
	return &env{db: db, accountSvc: accountSvc, voucherSvc: voucherSvc, transferSvc: transferSvc, txSvc: txSvc}
}

func TestCreateAndApproveTransfer(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")
	if err := e.accountSvc.Recharge(ctx, from.ID, 10000, "recharge-transfer", ""); err != nil {
		t.Fatal(err)
	}

	tr, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: 3000,
		Note: "实训基地转赠", IdempotencyKey: "transfer-001",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.Status != transfer.StatusPending || tr.SourceTransactionID == 0 {
		t.Fatalf("transfer = %+v", tr)
	}
	fromAfter, _ := e.accountSvc.Get(ctx, from.ID)
	if fromAfter.Balance != 7000 {
		t.Fatalf("balance = %d, want 7000", fromAfter.Balance)
	}
	dup, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: 3000,
		IdempotencyKey: "transfer-001",
	})
	if err != nil || dup.ID != tr.ID {
		t.Fatalf("dup = %+v, %v", dup, err)
	}

	reviewed, err := e.transferSvc.Review(ctx, tr.ID, &transfer.ReviewRequest{
		Decision: transfer.DecisionApproved, ReviewedBy: "赵子奕", Note: "通过",
	})
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if reviewed.Status != transfer.StatusApproved || reviewed.IssuedVoucherID == nil {
		t.Fatalf("reviewed = %+v", reviewed)
	}
	vouchers, err := e.voucherSvc.List(ctx, to.ID)
	if err != nil || len(vouchers) != 1 {
		t.Fatalf("vouchers = %+v, %v", vouchers, err)
	}
	if vouchers[0].Amount != 3000 || vouchers[0].AccountID != to.ID {
		t.Fatalf("voucher = %+v", vouchers[0])
	}
	again, err := e.transferSvc.Review(ctx, tr.ID, &transfer.ReviewRequest{
		Decision: transfer.DecisionApproved, ReviewedBy: "刘婧怡",
	})
	if err != nil || again.IssuedVoucherID == nil || *again.IssuedVoucherID != *reviewed.IssuedVoucherID {
		t.Fatalf("review again = %+v, %v", again, err)
	}
	vouchers, _ = e.voucherSvc.List(ctx, to.ID)
	if len(vouchers) != 1 {
		t.Fatalf("voucher count = %d, want 1", len(vouchers))
	}
}

func TestCreateTransferInsufficientBalanceRollsBack(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")

	_, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: 1,
		IdempotencyKey: "insufficient-001",
	})
	if !errors.Is(err, account.ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	var count int64
	if err := e.db.Model(&transfer.VoucherTransfer{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("transfer rows = %d, want 0", count)
	}
	txs, _ := e.txSvc.List(ctx, e.db, from.ID, 10, 0)
	if len(txs) != 0 {
		t.Fatalf("txs = %+v, want none", txs)
	}
}

func TestRejectTransferDoesNotIssueVoucher(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")
	e.accountSvc.Recharge(ctx, from.ID, 5000, "reject-recharge", "")
	tr, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: 1000,
		IdempotencyKey: "reject-001",
	})
	if err != nil {
		t.Fatal(err)
	}
	reviewed, err := e.transferSvc.Review(ctx, tr.ID, &transfer.ReviewRequest{
		Decision: transfer.DecisionRejected, ReviewedBy: "刘婧怡", Note: "不通过",
	})
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Status != transfer.StatusRejected || reviewed.IssuedVoucherID != nil {
		t.Fatalf("reviewed = %+v", reviewed)
	}
	vouchers, _ := e.voucherSvc.List(ctx, to.ID)
	if len(vouchers) != 0 {
		t.Fatalf("vouchers = %+v, want none", vouchers)
	}
}

func TestRefundRejectedTransfer(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")
	e.accountSvc.Recharge(ctx, from.ID, 5000, "refund-recharge", "")
	tr := createRejectedTransfer(t, e, from.ID, to.ID, 1200, "refund-001")

	refunded, err := e.transferSvc.Refund(ctx, tr.ID, &transfer.RefundRequest{
		RefundedBy: "赵子奕", Note: "人工退款完成",
	})
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if refunded.RefundTransactionID == nil || refunded.RefundedBy != "赵子奕" || refunded.RefundedAt == nil {
		t.Fatalf("refunded = %+v", refunded)
	}
	acc, _ := e.accountSvc.Get(ctx, from.ID)
	if acc.Balance != 5000 {
		t.Fatalf("balance = %d, want 5000", acc.Balance)
	}
	txs, _ := e.txSvc.List(ctx, e.db, from.ID, 10, 0)
	if len(txs) != 3 || txs[0].Type != transaction.TypeRecharge || txs[0].Amount != 1200 {
		t.Fatalf("txs = %+v", txs)
	}
}

func TestRefundRejectedTransferIdempotent(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")
	e.accountSvc.Recharge(ctx, from.ID, 5000, "refund-idem-recharge", "")
	tr := createRejectedTransfer(t, e, from.ID, to.ID, 1000, "refund-idem")

	first, err := e.transferSvc.Refund(ctx, tr.ID, &transfer.RefundRequest{RefundedBy: "赵子奕"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.transferSvc.Refund(ctx, tr.ID, &transfer.RefundRequest{RefundedBy: "刘婧怡"})
	if err != nil {
		t.Fatal(err)
	}
	if first.RefundTransactionID == nil || second.RefundTransactionID == nil || *first.RefundTransactionID != *second.RefundTransactionID {
		t.Fatalf("first=%+v second=%+v", first, second)
	}
	acc, _ := e.accountSvc.Get(ctx, from.ID)
	if acc.Balance != 5000 {
		t.Fatalf("balance = %d, want 5000", acc.Balance)
	}
	txs, _ := e.txSvc.List(ctx, e.db, from.ID, 10, 0)
	if len(txs) != 3 {
		t.Fatalf("tx count = %d, want 3", len(txs))
	}
}

func TestRefundTransferInvalidStatusAndRequest(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	from, _ := e.accountSvc.Create(ctx, "cust_from")
	to, _ := e.accountSvc.Create(ctx, "cust_to")
	e.accountSvc.Recharge(ctx, from.ID, 5000, "refund-invalid-recharge", "")
	pending, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: from.ID, ToAccountID: to.ID, Amount: 1000,
		IdempotencyKey: "refund-invalid-pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.transferSvc.Refund(ctx, pending.ID, &transfer.RefundRequest{RefundedBy: "赵子奕"}); !errors.Is(err, transfer.ErrInvalidStatus) {
		t.Fatalf("pending refund err = %v, want ErrInvalidStatus", err)
	}
	approved, err := e.transferSvc.Review(ctx, pending.ID, &transfer.ReviewRequest{
		Decision: transfer.DecisionApproved, ReviewedBy: "赵子奕",
	})
	if err != nil || approved.Status != transfer.StatusApproved {
		t.Fatalf("approved = %+v, %v", approved, err)
	}
	if _, err := e.transferSvc.Refund(ctx, approved.ID, &transfer.RefundRequest{RefundedBy: "赵子奕"}); !errors.Is(err, transfer.ErrInvalidStatus) {
		t.Fatalf("approved refund err = %v, want ErrInvalidStatus", err)
	}
	rejected := createRejectedTransfer(t, e, from.ID, to.ID, 500, "refund-invalid-empty")
	if _, err := e.transferSvc.Refund(ctx, rejected.ID, &transfer.RefundRequest{}); !errors.Is(err, transfer.ErrInvalidRequest) {
		t.Fatalf("empty refunded_by err = %v, want ErrInvalidRequest", err)
	}
}

func createRejectedTransfer(t *testing.T, e *env, fromID, toID string, amount int64, key string) *transfer.VoucherTransfer {
	t.Helper()
	ctx := context.Background()
	tr, err := e.transferSvc.Create(ctx, &transfer.CreateRequest{
		FromAccountID: fromID, ToAccountID: toID, Amount: amount,
		IdempotencyKey: key,
	})
	if err != nil {
		t.Fatal(err)
	}
	rejected, err := e.transferSvc.Review(ctx, tr.ID, &transfer.ReviewRequest{
		Decision: transfer.DecisionRejected, ReviewedBy: "刘婧怡",
	})
	if err != nil {
		t.Fatal(err)
	}
	return rejected
}
