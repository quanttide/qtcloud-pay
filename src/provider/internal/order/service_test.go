package order_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	accountgorm "github.com/quanttide/qtcloud-pay/src/provider/internal/account/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	billinggorm "github.com/quanttide/qtcloud-pay/src/provider/internal/billing/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	coupongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/coupon/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
	ordergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/order/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	transactiongorm "github.com/quanttide/qtcloud-pay/src/provider/internal/transaction/gorm"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	vouchergorm "github.com/quanttide/qtcloud-pay/src/provider/internal/voucher/gorm"
)

type env struct {
	db         *gorm.DB
	accountSvc *account.Service
	couponSvc  *coupon.Service
	voucherSvc *voucher.Service
	billingSvc *billing.Service
	orderSvc   *order.Service
	txSvc      *transaction.Service
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
	if err := db.AutoMigrate(
		&account.Account{}, &transaction.Transaction{},
		&coupon.Coupon{}, &voucher.Voucher{},
		&order.Order{}, &billing.BillingRule{},
	); err != nil {
		t.Fatal(err)
	}
	txSvc := transaction.NewService(transactiongorm.NewTransactionRepo())
	accSvc := account.NewService(db, accountgorm.NewAccountRepo(), txSvc)
	couponSvc := coupon.NewService(db, coupongorm.NewCouponRepo(), txSvc)
	voucherSvc := voucher.NewService(db, vouchergorm.NewVoucherRepo(), txSvc)
	billingSvc := billing.NewService(billinggorm.NewBillingRuleRepo())
	orderSvc := order.NewService(db, ordergorm.NewOrderRepo(), accSvc, couponSvc, voucherSvc, billingSvc, txSvc)
	return &env{db: db, accountSvc: accSvc, couponSvc: couponSvc,
		voucherSvc: voucherSvc, billingSvc: billingSvc, orderSvc: orderSvc, txSvc: txSvc}
}

// prepareAccount 充值并发放一套优惠，返回账户 ID。
func (e *env) prepareAccount(t *testing.T, ctx context.Context) string {
	t.Helper()
	acc, err := e.accountSvc.Create(ctx, "cust_1")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.accountSvc.Recharge(ctx, acc.ID, 100000, "voucher-main", "对公打款"); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(24 * time.Hour)
	if err := e.couponSvc.Issue(ctx, &coupon.IssueRequest{
		AccountID: acc.ID, Type: coupon.TypeFullReduction,
		Threshold: 80000, Amount: 20000, Scope: coupon.ScopeAll,
		ExpiresAt: expires, Count: 1, BatchNo: "coupon-fr-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.couponSvc.Issue(ctx, &coupon.IssueRequest{
		AccountID: acc.ID, Type: coupon.TypeDiscount, Rate: 90,
		Scope: coupon.ScopeAll, ExpiresAt: expires, Count: 1, BatchNo: "coupon-d-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := e.voucherSvc.Issue(ctx, &voucher.IssueRequest{
		AccountID: acc.ID, Amount: 5000, Scope: voucher.ScopeAll,
		ExpiresAt: expires, Count: 1, BatchNo: "voucher-1",
	}); err != nil {
		t.Fatal(err)
	}
	return acc.ID
}

func TestSettle_FullFlow(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	accID := e.prepareAccount(t, ctx)

	ord, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-1", CustomerID: "cust_1", AccountID: accID,
		ProductID: "course-1", Scope: "course", Amount: 100000,
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if ord.Status != order.StatusSettled || ord.SettledAt == nil {
		t.Errorf("order = %+v", ord)
	}
	// 结算明细：满减 20000 → 折扣省 8000（9 折）→ 代金券 5000 → 余额 67000
	var plan []billing.Deduction
	if err := json.Unmarshal(ord.SettleDetail, &plan); err != nil {
		t.Fatal(err)
	}
	if len(plan) != 4 {
		t.Fatalf("plan = %+v", plan)
	}
	want := []billing.Deduction{
		{Kind: billing.KindCoupon, RefID: plan[0].RefID, Amount: 20000},
		{Kind: billing.KindCoupon, RefID: plan[1].RefID, Amount: 8000},
		{Kind: billing.KindVoucher, RefID: plan[2].RefID, Amount: 5000},
		{Kind: billing.KindBalance, Amount: 67000},
	}
	for i, w := range want {
		if plan[i].Kind != w.Kind || plan[i].Amount != w.Amount {
			t.Errorf("plan[%d] = %+v, want %+v", i, plan[i], w)
		}
	}

	// 余额：100000 − 67000
	acc, _ := e.accountSvc.Get(ctx, accID)
	if acc.Balance != 33000 {
		t.Errorf("balance = %d, want 33000", acc.Balance)
	}

	// 账本：1 充值 + 3 发券 + 1 消费 + 3 核销
	txs, _ := e.txSvc.ListAll(ctx, e.db, accID)
	if len(txs) != 8 {
		t.Fatalf("txs = %d, want 8", len(txs))
	}
	var consume *transaction.Transaction
	var redeems int
	for i := range txs {
		if txs[i].Type == transaction.TypeConsume {
			consume = &txs[i]
		}
		if txs[i].Type == transaction.TypeRedeem {
			redeems++
		}
	}
	if consume == nil || consume.Amount != 67000 || consume.OrderID != "ORD-1" {
		t.Errorf("consume = %+v", consume)
	}
	if redeems != 3 {
		t.Errorf("redeems = %d, want 3", redeems)
	}

	// 券状态全部核销
	coupons, _ := e.couponSvc.List(ctx, accID)
	for _, c := range coupons {
		if c.Status != coupon.StatusUsed || c.OrderID != "ORD-1" {
			t.Errorf("coupon = %+v", c)
		}
	}
	vouchers, _ := e.voucherSvc.List(ctx, accID)
	if len(vouchers) != 1 || vouchers[0].Status != voucher.StatusUsed {
		t.Errorf("vouchers = %+v", vouchers)
	}
}

func TestSettle_BalanceOnly(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 10000, "v-main", "")

	ord, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-B", AccountID: acc.ID, Scope: "course", Amount: 10000,
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	var plan []billing.Deduction
	json.Unmarshal(ord.SettleDetail, &plan)
	if len(plan) != 1 || plan[0].Kind != billing.KindBalance || plan[0].Amount != 10000 {
		t.Errorf("plan = %+v", plan)
	}
	got, _ := e.accountSvc.Get(ctx, acc.ID)
	if got.Balance != 0 {
		t.Errorf("balance = %d, want 0", got.Balance)
	}
	txs, _ := e.txSvc.ListAll(ctx, e.db, acc.ID)
	if len(txs) != 2 { // 充值 + 消费
		t.Errorf("txs = %d, want 2", len(txs))
	}
}

func TestSettle_Idempotent(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	accID := e.prepareAccount(t, ctx)

	req := &order.SettleRequest{
		OrderID: "ORD-2", CustomerID: "cust_1", AccountID: accID,
		Scope: "course", Amount: 100000,
	}
	first, err := e.orderSvc.Settle(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := e.orderSvc.Settle(ctx, req)
	if err != nil {
		t.Fatalf("Settle(dup): %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("order id = %s, want %s", second.ID, first.ID)
	}
	// 不重复扣款、不重复核销
	acc, _ := e.accountSvc.Get(ctx, accID)
	if acc.Balance != 33000 {
		t.Errorf("balance = %d, want 33000", acc.Balance)
	}
	txs, _ := e.txSvc.ListAll(ctx, e.db, accID)
	if len(txs) != 8 { // 1 充值 + 3 发券 + 1 消费 + 3 核销
		t.Errorf("txs = %d, want 8", len(txs))
	}
}

func TestSettle_InsufficientBalance(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 1000, "v-main", "")

	_, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-3", AccountID: acc.ID, Scope: "course", Amount: 2000,
	})
	if !errors.Is(err, billing.ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	// 全部回滚：余额不变、无订单、无交易
	got, _ := e.accountSvc.Get(ctx, acc.ID)
	if got.Balance != 1000 {
		t.Errorf("balance = %d, want 1000", got.Balance)
	}
	if _, err := e.orderSvc.Get(ctx, "ORD-3"); !errors.Is(err, account.ErrNotFound) {
		t.Errorf("order should not exist: %v", err)
	}
	txs, _ := e.txSvc.ListAll(ctx, e.db, acc.ID)
	if len(txs) != 1 {
		t.Errorf("txs = %d, want 1（仅充值）", len(txs))
	}
}

func TestSettle_InvalidRequest(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	cases := []struct {
		name string
		req  *order.SettleRequest
	}{
		{"nil", nil},
		{"empty order id", &order.SettleRequest{AccountID: "acc_1", Amount: 100}},
		{"empty account", &order.SettleRequest{OrderID: "ORD-1", Amount: 100}},
		{"amount zero", &order.SettleRequest{OrderID: "ORD-1", AccountID: "acc_1", Amount: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := e.orderSvc.Settle(ctx, c.req); !errors.Is(err, order.ErrInvalidRequest) {
				t.Errorf("err = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestSettle_AccountNotFound(t *testing.T) {
	e := setupEnv(t)
	_, err := e.orderSvc.Settle(context.Background(), &order.SettleRequest{
		OrderID: "ORD-1", AccountID: "acc_missing", Scope: "course", Amount: 100,
	})
	if !errors.Is(err, account.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSettle_ScopeMismatch(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 10000, "v-main", "")
	// 仅指定商品的满减券（商品不匹配）
	e.couponSvc.Issue(ctx, &coupon.IssueRequest{
		AccountID: acc.ID, Type: coupon.TypeFullReduction,
		Threshold: 100, Amount: 5000, Scope: coupon.ScopeProduct, ProductID: "course-1",
		ExpiresAt: time.Now().Add(time.Hour), Count: 1, BatchNo: "cp-1",
	})

	ord, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-S", AccountID: acc.ID, ProductID: "course-2",
		Scope: "course", Amount: 10000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var plan []billing.Deduction
	json.Unmarshal(ord.SettleDetail, &plan)
	if len(plan) != 1 || plan[0].Kind != billing.KindBalance {
		t.Errorf("plan = %+v, want balance only", plan)
	}
}

func TestSettle_VoucherPartial(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 5000, "v-main", "")
	e.voucherSvc.Issue(ctx, &voucher.IssueRequest{
		AccountID: acc.ID, Amount: 10000, Scope: voucher.ScopeAll,
		ExpiresAt: time.Now().Add(time.Hour), Count: 1, BatchNo: "vp-1",
	})

	ord, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-V", AccountID: acc.ID, Scope: "course", Amount: 8000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var plan []billing.Deduction
	json.Unmarshal(ord.SettleDetail, &plan)
	if len(plan) != 1 || plan[0].Kind != billing.KindVoucher || plan[0].Amount != 8000 {
		t.Errorf("plan = %+v", plan)
	}
	// 余额分文未动
	got, _ := e.accountSvc.Get(ctx, acc.ID)
	if got.Balance != 5000 {
		t.Errorf("balance = %d, want 5000", got.Balance)
	}
}

func TestGet(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()
	accID := e.prepareAccount(t, ctx)
	e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-G", AccountID: accID, Scope: "course", Amount: 100000,
	})

	ord, err := e.orderSvc.Get(ctx, "ORD-G")
	if err != nil || ord.ID != "ORD-G" {
		t.Fatalf("Get = %+v, %v", ord, err)
	}

	if _, err := e.orderSvc.Get(ctx, "ORD-MISSING"); !errors.Is(err, account.ErrNotFound) {
		t.Errorf("Get(missing) err = %v, want ErrNotFound", err)
	}
}

func TestSettle_DBError(t *testing.T) {
	e := setupEnv(t)
	ctx := context.Background()

	sqlDB, _ := e.db.DB()
	sqlDB.Close()

	_, err := e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-E", AccountID: "acc_1", Scope: "course", Amount: 100,
	})
	if err == nil {
		t.Fatal("expected error on closed db")
	}
}

func TestSettle_GetError(t *testing.T) {
	e := setupEnv(t)
	sqlDB, _ := e.db.DB()
	sqlDB.Close()
	if _, err := e.orderSvc.Get(context.Background(), "ORD-1"); err == nil {
		t.Fatal("expected error on closed db")
	}
}

// ---- 错误分支注入（stub 依赖） ----

// stubAccount 注入账户服务错误。
type stubAccount struct {
	lockErr error
	saveErr error
}

func (s *stubAccount) Lock(ctx context.Context, db *gorm.DB, id string) (*account.Account, error) {
	if s.lockErr != nil {
		return nil, s.lockErr
	}
	return &account.Account{ID: id, Balance: 100000}, nil
}

func (s *stubAccount) Save(ctx context.Context, db *gorm.DB, a *account.Account) error {
	return s.saveErr
}

// stubCoupons 注入优惠券服务错误。
type stubCoupons struct {
	availErr error
	useErr   error
}

func (s *stubCoupons) Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]coupon.Coupon, error) {
	if s.availErr != nil {
		return nil, s.availErr
	}
	return nil, nil
}

func (s *stubCoupons) Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error {
	return s.useErr
}

// stubVouchers 注入代金券服务错误。
type stubVouchers struct {
	availErr error
	useErr   error
}

func (s *stubVouchers) Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]voucher.Voucher, error) {
	if s.availErr != nil {
		return nil, s.availErr
	}
	return nil, nil
}

func (s *stubVouchers) Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error {
	return s.useErr
}

// stubBilling 注入计费计算（固定计划或错误）。
type stubBilling struct {
	plan []billing.Deduction
	err  error
}

func (s *stubBilling) Calculate(amount int64, coupons []billing.CouponInput, vouchers []billing.VoucherInput, balance int64) ([]billing.Deduction, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.plan, nil
}

// stubTxSvc 注入交易账本错误。
type stubTxSvc struct {
	appendErr  error
	failRedeem bool
}

func (s *stubTxSvc) Append(ctx context.Context, db *gorm.DB, t *transaction.Transaction) error {
	if s.failRedeem && strings.Contains(t.IdempotencyKey, ":redeem:") {
		return errors.New("redeem write failed")
	}
	return s.appendErr
}

// newOrderSvcWith 用自定义依赖构建订单服务。
func newOrderSvcWith(t *testing.T, db *gorm.DB, repo order.Repository,
	acc accountSvcT, coupons couponSvcT, vouchers voucherSvcT, bill billingSvcT, tx txSvcT) *order.Service {
	t.Helper()
	return order.NewService(db, repo, acc, coupons, vouchers, bill, tx)
}

// 测试用依赖接口别名（与实现包内的依赖接口方法集一致）。
type (
	accountSvcT interface {
		Lock(ctx context.Context, db *gorm.DB, id string) (*account.Account, error)
		Save(ctx context.Context, db *gorm.DB, a *account.Account) error
	}
	couponSvcT interface {
		Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]coupon.Coupon, error)
		Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error
	}
	voucherSvcT interface {
		Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]voucher.Voucher, error)
		Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error
	}
	billingSvcT interface {
		Calculate(amount int64, coupons []billing.CouponInput, vouchers []billing.VoucherInput, balance int64) ([]billing.Deduction, error)
	}
	txSvcT interface {
		Append(ctx context.Context, db *gorm.DB, t *transaction.Transaction) error
	}
)

func TestSettle_ErrorBranches(t *testing.T) {
	cases := []struct {
		name  string
		build func(e *env) *order.Service
	}{
		{"coupon available error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				&stubCoupons{availErr: errors.New("down")}, e.voucherSvc, e.billingSvc, e.txSvc)
		}},
		{"voucher available error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				e.couponSvc, &stubVouchers{availErr: errors.New("down")}, e.billingSvc, e.txSvc)
		}},
		{"billing error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				e.couponSvc, e.voucherSvc, &stubBilling{err: errors.New("calc failed")}, e.txSvc)
		}},
		{"coupon use error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				&stubCoupons{useErr: errors.New("use failed")}, e.voucherSvc,
				&stubBilling{plan: []billing.Deduction{{Kind: billing.KindCoupon, RefID: 1, Amount: 100}}}, e.txSvc)
		}},
		{"voucher use error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				e.couponSvc, &stubVouchers{useErr: errors.New("use failed")},
				&stubBilling{plan: []billing.Deduction{{Kind: billing.KindVoucher, RefID: 1, Amount: 100}}}, e.txSvc)
		}},
		{"account save error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), &stubAccount{saveErr: errors.New("save failed")},
				e.couponSvc, e.voucherSvc,
				&stubBilling{plan: []billing.Deduction{{Kind: billing.KindBalance, Amount: 100}}}, e.txSvc)
		}},
		{"consume append error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				e.couponSvc, e.voucherSvc,
				&stubBilling{plan: []billing.Deduction{{Kind: billing.KindBalance, Amount: 100}}},
				&stubTxSvc{appendErr: errors.New("append failed")})
		}},
		{"redeem append error", func(e *env) *order.Service {
			return newOrderSvcWith(t, e.db, ordergorm.NewOrderRepo(), e.accountSvc,
				&stubCoupons{}, e.voucherSvc,
				&stubBilling{plan: []billing.Deduction{{Kind: billing.KindCoupon, RefID: 1, Amount: 100}}},
				&stubTxSvc{failRedeem: true})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := setupEnv(t)
			ctx := context.Background()
			acc, _ := e.accountSvc.Create(ctx, "cust_1")
			e.accountSvc.Recharge(ctx, acc.ID, 100000, "v-main", "")

			svc := c.build(e)
			_, err := svc.Settle(ctx, &order.SettleRequest{
				OrderID: "ORD-E", AccountID: acc.ID, Scope: "course", Amount: 10000,
			})
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestSettle_DuplicateCreate(t *testing.T) {
	// 并发重复结算：订单创建撞唯一键 → 返回已有订单
	e := setupEnv(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 100000, "v-main", "")

	repo := &duplicateOrderRepo{repo: ordergorm.NewOrderRepo()}
	svc := newOrderSvcWith(t, e.db, repo, e.accountSvc, e.couponSvc, e.voucherSvc, e.billingSvc, e.txSvc)

	ord, err := svc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-DUP", AccountID: acc.ID, Scope: "course", Amount: 10000,
	})
	if err != nil {
		t.Fatalf("Settle: %v", err)
	}
	if ord.ID != "ORD-DUP" {
		t.Errorf("order = %+v", ord)
	}
	// 余额未被重复扣减
	got, _ := e.accountSvc.Get(ctx, acc.ID)
	if got.Balance != 100000 {
		t.Errorf("balance = %d, want 100000（重复结算已回滚）", got.Balance)
	}
}

// duplicateOrderRepo 第一次 Get 返回不存在、Create 返回唯一冲突、之后 Get 返回已存在订单。
type duplicateOrderRepo struct {
	repo  order.Repository
	calls int
}

func (r *duplicateOrderRepo) Create(db *gorm.DB, o *order.Order) error {
	return gorm.ErrDuplicatedKey
}

func (r *duplicateOrderRepo) Get(db *gorm.DB, id string) (*order.Order, error) {
	r.calls++
	if r.calls == 1 {
		return nil, gorm.ErrRecordNotFound
	}
	return &order.Order{ID: id, Status: order.StatusSettled, Amount: 10000}, nil
}
