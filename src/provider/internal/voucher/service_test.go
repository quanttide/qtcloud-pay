package voucher_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
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
	if err := db.AutoMigrate(&voucher.Voucher{}, &voucher.PricingRuleSet{}, &transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newService(t *testing.T) (*voucher.Service, *gorm.DB) {
	t.Helper()
	db := setupDB(t)
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	svc := voucher.NewService(db, vouchergorm.NewVoucherRepo(), txSvc)
	return svc, db
}

func validIssue() *voucher.IssueRequest {
	return &voucher.IssueRequest{
		AccountID: "acc_1", Amount: 3000, Scope: voucher.ScopeAll,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Count:     2, BatchNo: "batch-v-001",
	}
}

func TestIssue(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	if err := svc.Issue(ctx, validIssue()); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	list, err := svc.List(ctx, "acc_1")
	if err != nil || len(list) != 2 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
	for _, v := range list {
		if v.Status != voucher.StatusIssued || v.Amount != 3000 || v.BatchNo != "batch-v-001" {
			t.Errorf("voucher = %+v", v)
		}
	}
	// 发券交易：金额 = 批次总面值
	txs, _ := transaction.NewService(transactiongorm.NewTransactionRepo()).List(ctx, db, "acc_1", 10, 0)
	if len(txs) != 1 || txs[0].Type != transaction.TypeIssue || txs[0].Amount != 6000 {
		t.Errorf("txs = %+v", txs)
	}
}

func TestIssue_Idempotent(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	svc.Issue(ctx, validIssue())
	if err := svc.Issue(ctx, validIssue()); err != nil {
		t.Fatalf("Issue(dup): %v", err)
	}
	list, _ := svc.List(ctx, "acc_1")
	if len(list) != 2 {
		t.Errorf("vouchers = %d, want 2", len(list))
	}
	txs, _ := transaction.NewService(transactiongorm.NewTransactionRepo()).List(ctx, db, "acc_1", 10, 0)
	if len(txs) != 1 {
		t.Errorf("txs = %d, want 1", len(txs))
	}
}

func TestIssue_Validation(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	base := validIssue()
	cases := []struct {
		name string
		mut  func(*voucher.IssueRequest)
	}{
		{"nil", func(r *voucher.IssueRequest) {}},
		{"empty account", func(r *voucher.IssueRequest) { r.AccountID = "" }},
		{"empty batch", func(r *voucher.IssueRequest) { r.BatchNo = "" }},
		{"amount zero", func(r *voucher.IssueRequest) { r.Amount = 0 }},
		{"amount negative", func(r *voucher.IssueRequest) { r.Amount = -1 }},
		{"count zero", func(r *voucher.IssueRequest) { r.Count = 0 }},
		{"count too large", func(r *voucher.IssueRequest) { r.Count = 1001 }},
		{"expired", func(r *voucher.IssueRequest) { r.ExpiresAt = time.Now().Add(-time.Hour) }},
		{"bad scope", func(r *voucher.IssueRequest) { r.Scope = "unknown" }},
		{"product scope no product", func(r *voucher.IssueRequest) { r.Scope = voucher.ScopeProduct; r.ProductID = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var err error
			if c.name == "nil" {
				err = svc.Issue(ctx, nil)
			} else {
				req := *base
				c.mut(&req)
				err = svc.Issue(ctx, &req)
			}
			if !errors.Is(err, voucher.ErrInvalidIssue) {
				t.Errorf("err = %v, want ErrInvalidIssue", err)
			}
		})
	}
}

func TestIssue_ProductScope(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	req := validIssue()
	req.Scope = voucher.ScopeProduct
	req.ProductID = "data-1"
	if err := svc.Issue(ctx, req); err != nil {
		t.Fatalf("Issue(product): %v", err)
	}
}

// stubVoucherRepo 注入代金券仓库错误。
type stubVoucherRepo struct {
	voucher.Repository
	createBatchErr error
	updateErr      error
	getErr         error
}

func (s *stubVoucherRepo) CreateBatch(db *gorm.DB, vouchers []*voucher.Voucher) error {
	return s.createBatchErr
}

func (s *stubVoucherRepo) CountByBatch(db *gorm.DB, batchNo string) (int64, error) {
	return 0, nil
}

func (s *stubVoucherRepo) Update(db *gorm.DB, v *voucher.Voucher) error {
	return s.updateErr
}

func (s *stubVoucherRepo) GetForUpdate(db *gorm.DB, id int64) (*voucher.Voucher, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &voucher.Voucher{
		ID: id, Status: voucher.StatusIssued,
		ExpiresAt: time.Now().Add(time.Hour),
	}, nil
}

// stubTxRepo 注入交易账本错误。
type stubTxRepo struct {
	transaction.Repository
	getErr    error
	createErr error
}

func (s *stubTxRepo) GetByKey(db *gorm.DB, key string) (*transaction.Transaction, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *stubTxRepo) Create(db *gorm.DB, t *transaction.Transaction) error {
	return s.createErr
}

func TestIssue_RepoErrors(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	req := validIssue()

	svc1 := voucher.NewService(db, &stubVoucherRepo{}, transaction.NewService(&stubTxRepo{getErr: errors.New("down")}))
	if err := svc1.Issue(ctx, req); err == nil {
		t.Error("Exists error should propagate")
	}

	svc2 := voucher.NewService(db, &stubVoucherRepo{createBatchErr: errors.New("write failed")},
		transaction.NewService(&stubTxRepo{}))
	if err := svc2.Issue(ctx, req); err == nil {
		t.Error("CreateBatch error should propagate")
	}

	svc3 := voucher.NewService(db, &stubVoucherRepo{createBatchErr: gorm.ErrDuplicatedKey},
		transaction.NewService(&stubTxRepo{}))
	if err := svc3.Issue(ctx, req); err != nil {
		t.Errorf("duplicate CreateBatch should be nil, got %v", err)
	}

	svc4 := voucher.NewService(db, &stubVoucherRepo{},
		transaction.NewService(&stubTxRepo{createErr: errors.New("append failed")}))
	if err := svc4.Issue(ctx, req); err == nil {
		t.Error("Append error should propagate")
	}
}

func TestList_LazyExpiry(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	repo := vouchergorm.NewVoucherRepo()
	repo.CreateBatch(db, []*voucher.Voucher{{
		AccountID: "acc_1", BatchNo: "old", Amount: 100, Scope: voucher.ScopeAll,
		ExpiresAt: time.Now().Add(-time.Hour), Status: voucher.StatusIssued,
	}})

	list, err := svc.List(ctx, "acc_1")
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
	if list[0].Status != voucher.StatusExpired {
		t.Errorf("status = %s, want expired", list[0].Status)
	}
	got, _ := repo.Get(db, list[0].ID)
	if got.Status != voucher.StatusExpired {
		t.Errorf("persisted status = %s, want expired", got.Status)
	}
}

func TestAvailable(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	now := time.Now()
	repo := vouchergorm.NewVoucherRepo()
	seed := func(status string, expiresIn time.Duration, scope, productID string) {
		repo.CreateBatch(db, []*voucher.Voucher{{
			AccountID: "acc_1", BatchNo: scope + productID + status, Amount: 100,
			Scope: scope, ProductID: productID, Status: status,
			ExpiresAt: now.Add(expiresIn),
		}})
	}
	seed(voucher.StatusIssued, time.Hour, voucher.ScopeAll, "")
	seed(voucher.StatusUsed, time.Hour, voucher.ScopeAll, "")
	seed(voucher.StatusIssued, -time.Hour, voucher.ScopeAll, "")
	seed(voucher.StatusIssued, time.Hour, voucher.ScopeData, "")

	available, err := svc.Available(ctx, db, "acc_1", voucher.ScopeData, "data-1")
	if err != nil || len(available) != 2 {
		t.Fatalf("Available = %d items, %v", len(available), err)
	}
}

func TestUse(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	req := validIssue()
	req.Count = 1
	svc.Issue(ctx, req)
	list, _ := svc.List(ctx, "acc_1")

	if err := svc.Use(ctx, db, list[0].ID, "ORD-1"); err != nil {
		t.Fatalf("Use: %v", err)
	}
	got, _ := svc.List(ctx, "acc_1")
	if got[0].Status != voucher.StatusUsed || got[0].OrderID != "ORD-1" || got[0].UsedAt == nil {
		t.Errorf("voucher = %+v", got[0])
	}

	if err := svc.Use(ctx, db, list[0].ID, "ORD-2"); !errors.Is(err, voucher.ErrUnavailable) {
		t.Errorf("use again err = %v, want ErrUnavailable", err)
	}
	if err := svc.Use(ctx, db, 99999, "ORD-3"); !errors.Is(err, voucher.ErrUnavailable) {
		t.Errorf("missing err = %v, want ErrUnavailable", err)
	}
}

func TestUse_Expired(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	repo := vouchergorm.NewVoucherRepo()
	repo.CreateBatch(db, []*voucher.Voucher{{
		AccountID: "acc_1", BatchNo: "old", Amount: 100, Scope: voucher.ScopeAll,
		ExpiresAt: time.Now().Add(-time.Hour), Status: voucher.StatusIssued,
	}})
	list, _ := svc.List(ctx, "acc_1")

	if err := svc.Use(ctx, db, list[0].ID, "ORD-1"); !errors.Is(err, voucher.ErrUnavailable) {
		t.Errorf("expired err = %v, want ErrUnavailable", err)
	}
}

func TestUse_RepoErrors(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	svc1 := voucher.NewService(db, &stubVoucherRepo{getErr: errors.New("down")},
		transaction.NewService(transactiongorm.NewTransactionRepo()))
	if err := svc1.Use(ctx, db, 1, "ORD-1"); err == nil {
		t.Error("GetForUpdate error should propagate")
	}

	svc2 := voucher.NewService(db, &stubVoucherRepo{updateErr: errors.New("write failed")},
		transaction.NewService(transactiongorm.NewTransactionRepo()))
	if err := svc2.Use(ctx, db, 1, "ORD-1"); err == nil {
		t.Error("Update error should propagate")
	}
}

func TestMatchesScopeAndExpired(t *testing.T) {
	now := time.Now()
	v := &voucher.Voucher{Scope: voucher.ScopeAll}
	if !v.MatchesScope(voucher.ScopeData, "") {
		t.Error("all scope should match any")
	}
	v = &voucher.Voucher{Scope: voucher.ScopeProduct, ProductID: "p1"}
	if !v.MatchesScope(voucher.ScopeData, "p1") || v.MatchesScope(voucher.ScopeData, "p2") {
		t.Error("product scope matching wrong")
	}

	if !(&voucher.Voucher{ExpiresAt: now.Add(-time.Minute)}).Expired(now) {
		t.Error("should be expired")
	}
	if (&voucher.Voucher{ExpiresAt: now.Add(time.Minute)}).Expired(now) {
		t.Error("should not be expired")
	}
}
