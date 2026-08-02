package coupon_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	coupongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/coupon/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
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
	if err := db.AutoMigrate(&coupon.Coupon{}, &transaction.Transaction{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func newService(t *testing.T) (*coupon.Service, *gorm.DB) {
	t.Helper()
	db := setupDB(t)
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	svc := coupon.NewService(db, coupongorm.NewCouponRepo(), txSvc)
	return svc, db
}

func validIssue() *coupon.IssueRequest {
	return &coupon.IssueRequest{
		AccountID: "acc_1", Type: coupon.TypeFullReduction,
		Threshold: 10000, Amount: 2000, Scope: coupon.ScopeAll,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		Count:     3, BatchNo: "batch-001",
	}
}

func TestIssue(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	if err := svc.Issue(ctx, validIssue()); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	list, err := svc.List(ctx, "acc_1")
	if err != nil || len(list) != 3 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
	for _, c := range list {
		if c.Status != coupon.StatusIssued || c.BatchNo != "batch-001" ||
			c.Threshold != 10000 || c.Amount != 2000 || c.Type != coupon.TypeFullReduction {
			t.Errorf("coupon = %+v", c)
		}
	}
	// 发券交易已入账（金额 = 批次总面值）
	txs, err := transaction.NewService(transactiongorm.NewTransactionRepo()).List(ctx, db, "acc_1", 10, 0)
	if err != nil || len(txs) != 1 {
		t.Fatalf("txs = %d, %v", len(txs), err)
	}
	if txs[0].Type != transaction.TypeIssue || txs[0].Amount != 6000 {
		t.Errorf("tx = %+v", txs[0])
	}
}

func TestIssue_Discount(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	req := validIssue()
	req.Type = coupon.TypeDiscount
	req.Rate = 90
	req.Threshold, req.Amount = 0, 0
	if err := svc.Issue(ctx, req); err != nil {
		t.Fatalf("Issue(discount): %v", err)
	}
	list, _ := svc.List(ctx, "acc_1")
	if len(list) != 3 || list[0].Rate != 90 {
		t.Errorf("list = %+v", list)
	}
}

func TestIssue_Idempotent(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	if err := svc.Issue(ctx, validIssue()); err != nil {
		t.Fatal(err)
	}
	// 同批次重复发放 → 不重复
	if err := svc.Issue(ctx, validIssue()); err != nil {
		t.Fatalf("Issue(dup): %v", err)
	}
	list, _ := svc.List(ctx, "acc_1")
	if len(list) != 3 {
		t.Errorf("coupons = %d, want 3", len(list))
	}
	txs, _ := transaction.NewService(transactiongorm.NewTransactionRepo()).List(ctx, db, "acc_1", 10, 0)
	if len(txs) != 1 {
		t.Errorf("txs = %d, want 1", len(txs))
	}
}

func TestIssue_CountByBatchFallback(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	// 券已存在但无发券交易（历史数据场景）→ 防御分支
	repo := coupongorm.NewCouponRepo()
	req := validIssue()
	req.Count = 2
	repo.CreateBatch(db, []*coupon.Coupon{{
		AccountID: req.AccountID, BatchNo: req.BatchNo, Type: req.Type,
		Threshold: req.Threshold, Amount: req.Amount, Scope: req.Scope,
		ExpiresAt: req.ExpiresAt, Status: coupon.StatusIssued,
	}})
	if err := svc.Issue(ctx, req); err != nil {
		t.Fatalf("Issue: %v", err)
	}
	list, _ := svc.List(ctx, "acc_1")
	if len(list) != 1 {
		t.Errorf("coupons = %d, want 1（不重复发放）", len(list))
	}
}

func TestIssue_Validation(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	base := validIssue()
	cases := []struct {
		name string
		mut  func(*coupon.IssueRequest)
	}{
		{"nil", func(r *coupon.IssueRequest) {}},
		{"empty account", func(r *coupon.IssueRequest) { r.AccountID = "" }},
		{"empty batch", func(r *coupon.IssueRequest) { r.BatchNo = "" }},
		{"count zero", func(r *coupon.IssueRequest) { r.Count = 0 }},
		{"count too large", func(r *coupon.IssueRequest) { r.Count = 1001 }},
		{"expired", func(r *coupon.IssueRequest) { r.ExpiresAt = time.Now().Add(-time.Hour) }},
		{"bad type", func(r *coupon.IssueRequest) { r.Type = "unknown" }},
		{"discount rate zero", func(r *coupon.IssueRequest) { r.Type = coupon.TypeDiscount; r.Rate = 0 }},
		{"discount rate too large", func(r *coupon.IssueRequest) { r.Type = coupon.TypeDiscount; r.Rate = 101 }},
		{"threshold zero", func(r *coupon.IssueRequest) { r.Threshold = 0 }},
		{"amount zero", func(r *coupon.IssueRequest) { r.Amount = 0 }},
		{"amount over threshold", func(r *coupon.IssueRequest) { r.Amount = 20001 }},
		{"bad scope", func(r *coupon.IssueRequest) { r.Scope = "unknown" }},
		{"product scope no product", func(r *coupon.IssueRequest) { r.Scope = coupon.ScopeProduct; r.ProductID = "" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := *base
			c.mut(&req)
			var err error
			if c.name == "nil" {
				err = svc.Issue(ctx, nil)
			} else {
				err = svc.Issue(ctx, &req)
			}
			if !errors.Is(err, coupon.ErrInvalidIssue) {
				t.Errorf("err = %v, want ErrInvalidIssue", err)
			}
		})
	}
}

func TestIssue_ProductScope(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	req := validIssue()
	req.Scope = coupon.ScopeProduct
	req.ProductID = "course-1"
	if err := svc.Issue(ctx, req); err != nil {
		t.Fatalf("Issue(product): %v", err)
	}
}

// stubCouponRepo 注入优惠券仓库错误。
type stubCouponRepo struct {
	coupon.Repository
	createBatchErr error
	updateErr      error
	getErr         error
}

func (s *stubCouponRepo) CreateBatch(db *gorm.DB, coupons []*coupon.Coupon) error {
	return s.createBatchErr
}

func (s *stubCouponRepo) CountByBatch(db *gorm.DB, batchNo string) (int64, error) {
	return 0, nil
}

func (s *stubCouponRepo) Update(db *gorm.DB, c *coupon.Coupon) error {
	return s.updateErr
}

func (s *stubCouponRepo) GetForUpdate(db *gorm.DB, id int64) (*coupon.Coupon, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &coupon.Coupon{
		ID: id, Status: coupon.StatusIssued,
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

	// 幂等检查失败 → 传播
	svc1 := coupon.NewService(db, &stubCouponRepo{}, transaction.NewService(&stubTxRepo{getErr: errors.New("down")}))
	if err := svc1.Issue(ctx, req); err == nil {
		t.Error("Exists error should propagate")
	}

	// CreateBatch 失败 → 传播
	svc2 := coupon.NewService(db, &stubCouponRepo{createBatchErr: errors.New("write failed")},
		transaction.NewService(&stubTxRepo{}))
	if err := svc2.Issue(ctx, req); err == nil {
		t.Error("CreateBatch error should propagate")
	}

	// CreateBatch 唯一冲突（并发重复发放）→ 视为成功
	svc3 := coupon.NewService(db, &stubCouponRepo{createBatchErr: gorm.ErrDuplicatedKey},
		transaction.NewService(&stubTxRepo{}))
	if err := svc3.Issue(ctx, req); err != nil {
		t.Errorf("duplicate CreateBatch should be nil, got %v", err)
	}

	// 发券交易写入失败 → 传播（券一并回滚）
	svc4 := coupon.NewService(db, &stubCouponRepo{},
		transaction.NewService(&stubTxRepo{createErr: errors.New("append failed")}))
	if err := svc4.Issue(ctx, req); err == nil {
		t.Error("Append error should propagate")
	}
}

func TestList_LazyExpiry(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	// 直接入库一张已过期券（绕过发放校验）
	repo := coupongorm.NewCouponRepo()
	repo.CreateBatch(db, []*coupon.Coupon{{
		AccountID: "acc_1", BatchNo: "old", Type: coupon.TypeFullReduction,
		Threshold: 100, Amount: 10, Scope: coupon.ScopeAll,
		ExpiresAt: time.Now().Add(-time.Hour), Status: coupon.StatusIssued,
	}})

	list, err := svc.List(ctx, "acc_1")
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %d items, %v", len(list), err)
	}
	if list[0].Status != coupon.StatusExpired {
		t.Errorf("status = %s, want expired（惰性流转）", list[0].Status)
	}
	// 状态已持久化
	got, _ := repo.Get(db, list[0].ID)
	if got.Status != coupon.StatusExpired {
		t.Errorf("persisted status = %s, want expired", got.Status)
	}
}

func TestList_EmptyAndError(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	list, err := svc.List(ctx, "acc_none")
	if err != nil || len(list) != 0 {
		t.Errorf("List(empty) = %d, %v", len(list), err)
	}
}

func TestAvailable(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	now := time.Now()
	repo := coupongorm.NewCouponRepo()
	seed := func(status string, expiresIn time.Duration, scope, productID string) coupon.Coupon {
		c := coupon.Coupon{
			AccountID: "acc_1", BatchNo: scope + productID + status,
			Type: coupon.TypeFullReduction, Threshold: 100, Amount: 10,
			Scope: scope, ProductID: productID, Status: status,
			ExpiresAt: now.Add(expiresIn),
		}
		repo.CreateBatch(db, []*coupon.Coupon{&c})
		return c
	}
	seed(coupon.StatusIssued, time.Hour, coupon.ScopeAll, "")
	seed(coupon.StatusUsed, time.Hour, coupon.ScopeAll, "")
	seed(coupon.StatusIssued, -time.Hour, coupon.ScopeAll, "")   // 已过期
	seed(coupon.StatusIssued, time.Hour, coupon.ScopeCourse, "") // 业务不匹配
	seed(coupon.StatusIssued, time.Hour, coupon.ScopeProduct, "course-1")

	// 全场 + 课程 + 指定商品（course-1）在 course 业务下可用
	available, err := svc.Available(ctx, db, "acc_1", coupon.ScopeCourse, "course-1")
	if err != nil || len(available) != 3 {
		t.Fatalf("Available = %d items, %v", len(available), err)
	}

	// 指定商品（course-2）不匹配：只剩全场 + 课程
	available, err = svc.Available(ctx, db, "acc_1", coupon.ScopeCourse, "course-2")
	if err != nil || len(available) != 2 {
		t.Fatalf("Available(mismatch) = %d items, %v", len(available), err)
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
	if got[0].Status != coupon.StatusUsed || got[0].OrderID != "ORD-1" || got[0].UsedAt == nil {
		t.Errorf("coupon = %+v", got[0])
	}

	// 已使用 → ErrUnavailable
	if err := svc.Use(ctx, db, list[0].ID, "ORD-2"); !errors.Is(err, coupon.ErrUnavailable) {
		t.Errorf("use again err = %v, want ErrUnavailable", err)
	}
	// 不存在 → ErrUnavailable
	if err := svc.Use(ctx, db, 99999, "ORD-3"); !errors.Is(err, coupon.ErrUnavailable) {
		t.Errorf("missing err = %v, want ErrUnavailable", err)
	}
}

func TestUse_Expired(t *testing.T) {
	svc, db := newService(t)
	ctx := context.Background()

	repo := coupongorm.NewCouponRepo()
	repo.CreateBatch(db, []*coupon.Coupon{{
		AccountID: "acc_1", BatchNo: "old", Type: coupon.TypeFullReduction,
		Threshold: 100, Amount: 10, Scope: coupon.ScopeAll,
		ExpiresAt: time.Now().Add(-time.Hour), Status: coupon.StatusIssued,
	}})
	list, _ := svc.List(ctx, "acc_1")

	if err := svc.Use(ctx, db, list[0].ID, "ORD-1"); !errors.Is(err, coupon.ErrUnavailable) {
		t.Errorf("expired err = %v, want ErrUnavailable", err)
	}
}

func TestUse_RepoErrors(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()

	// 查询错误 → 传播
	svc1 := coupon.NewService(db, &stubCouponRepo{getErr: errors.New("down")},
		transaction.NewService(transactiongorm.NewTransactionRepo()))
	if err := svc1.Use(ctx, db, 1, "ORD-1"); err == nil {
		t.Error("GetForUpdate error should propagate")
	}

	// 更新错误 → 传播
	svc2 := coupon.NewService(db, &stubCouponRepo{updateErr: errors.New("write failed")},
		transaction.NewService(transactiongorm.NewTransactionRepo()))
	if err := svc2.Use(ctx, db, 1, "ORD-1"); err == nil {
		t.Error("Update error should propagate")
	}
}

func TestMatchesScopeAndExpired(t *testing.T) {
	now := time.Now()
	c := &coupon.Coupon{Scope: coupon.ScopeAll}
	if !c.MatchesScope(coupon.ScopeCourse, "") {
		t.Error("all scope should match any")
	}
	c = &coupon.Coupon{Scope: coupon.ScopeProduct, ProductID: "p1"}
	if !c.MatchesScope(coupon.ScopeData, "p1") {
		t.Error("product scope should match product id")
	}
	if c.MatchesScope(coupon.ScopeData, "p2") {
		t.Error("product scope should not match other product")
	}
	c = &coupon.Coupon{Scope: coupon.ScopeCourse}
	if !c.MatchesScope(coupon.ScopeCourse, "") {
		t.Error("course scope should match course")
	}
	if c.MatchesScope(coupon.ScopeData, "") {
		t.Error("course scope should not match data")
	}

	if !(&coupon.Coupon{ExpiresAt: now.Add(-time.Minute)}).Expired(now) {
		t.Error("should be expired")
	}
	if (&coupon.Coupon{ExpiresAt: now.Add(time.Minute)}).Expired(now) {
		t.Error("should not be expired")
	}
}
