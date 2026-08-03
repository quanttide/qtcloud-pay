package order

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/idempotency"
)

var (
	// ErrInvalidRequest 结算请求不合法。
	ErrInvalidRequest = errors.New("order: invalid settle request")
)

// SettleRequest 结算请求。
type SettleRequest struct {
	OrderID    string // 商户订单号（幂等键）
	CustomerID string
	AccountID  string
	ProductID  string
	Scope      string // 业务类型（云服务/课程/数据服务）
	Amount     int64  // 订单金额（分）
}

// 依赖接口（结算编排所需的最小表面，便于测试注入）。
// 具体实现由各模块 Service 提供。
type (
	accountSvc interface {
		Lock(ctx context.Context, db *gorm.DB, id string) (*account.Account, error)
		Save(ctx context.Context, db *gorm.DB, a *account.Account) error
	}
	couponSvc interface {
		Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]coupon.Coupon, error)
		Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error
	}
	voucherSvc interface {
		Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]voucher.Voucher, error)
		Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error
	}
	billingSvc interface {
		Calculate(amount int64, coupons []billing.CouponInput, vouchers []billing.VoucherInput, balance int64) ([]billing.Deduction, error)
	}
	transactionSvc interface {
		Append(ctx context.Context, db *gorm.DB, t *transaction.Transaction) error
	}
)

// Service 订单与结算服务：结算编排者，单事务协调 billing/coupon/voucher/account/transaction。
type Service struct {
	db         *gorm.DB
	repo       Repository
	accountSvc accountSvc
	couponSvc  couponSvc
	voucherSvc voucherSvc
	billingSvc billingSvc
	txSvc      transactionSvc
}

// NewService 创建订单服务。
func NewService(db *gorm.DB, repo Repository, accountSvc accountSvc,
	couponSvc couponSvc, voucherSvc voucherSvc,
	billingSvc billingSvc, txSvc transactionSvc) *Service {
	return &Service{
		db: db, repo: repo,
		accountSvc: accountSvc, couponSvc: couponSvc, voucherSvc: voucherSvc,
		billingSvc: billingSvc, txSvc: txSvc,
	}
}

// Settle 下单并结算：应用计费规则 → 生成消费/核销交易 → 更新余额与券状态，全程单事务。
// 幂等：同一订单号重复提交返回已有订单，不重复结算。
func (s *Service) Settle(ctx context.Context, req *SettleRequest) (*Order, error) {
	if req == nil || req.OrderID == "" || req.AccountID == "" || req.Amount <= 0 {
		return nil, ErrInvalidRequest
	}
	var order *Order
	err := s.db.Transaction(func(tx *gorm.DB) error {
		existing, err := s.repo.Get(tx, req.OrderID)
		if err == nil && existing != nil {
			order = existing // 幂等：已结算过
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 锁账户行：同账户结算串行化，券核销无需再单独加锁
		acc, err := s.accountSvc.Lock(ctx, tx, req.AccountID)
		if err != nil {
			return err
		}
		coupons, err := s.couponSvc.Available(ctx, tx, req.AccountID, req.Scope, req.ProductID)
		if err != nil {
			return err
		}
		vouchers, err := s.voucherSvc.Available(ctx, tx, req.AccountID, req.Scope, req.ProductID)
		if err != nil {
			return err
		}
		plan, err := s.billingSvc.Calculate(req.Amount, toCouponInputs(coupons), toVoucherInputs(vouchers), acc.Balance)
		if err != nil {
			return err // 余额不足等，整体回滚
		}

		// 执行：核销券、扣余额
		var balancePaid int64
		for _, d := range plan {
			switch d.Kind {
			case billing.KindCoupon:
				if err := s.couponSvc.Use(ctx, tx, d.RefID, req.OrderID); err != nil {
					return err
				}
			case billing.KindVoucher:
				if err := s.voucherSvc.Use(ctx, tx, d.RefID, req.OrderID); err != nil {
					return err
				}
			case billing.KindBalance:
				balancePaid = d.Amount
			}
		}
		acc.Balance -= balancePaid
		if err := s.accountSvc.Save(ctx, tx, acc); err != nil {
			return err
		}

		// 账本：余额部分一条消费交易，每张券一条核销交易
		if balancePaid > 0 {
			key, err := idempotency.Key(idempotency.Settle, req.OrderID)
			if err != nil {
				return err
			}
			if err := s.txSvc.Append(ctx, tx, &transaction.Transaction{
				AccountID:      req.AccountID,
				Type:           transaction.TypeConsume,
				Amount:         balancePaid,
				BalanceAfter:   acc.Balance,
				OrderID:        req.OrderID,
				IdempotencyKey: key,
			}); err != nil {
				return err
			}
		}
		for _, d := range plan {
			if d.Kind != billing.KindCoupon && d.Kind != billing.KindVoucher {
				continue
			}
			key, err := idempotency.SettleRedeemKey(req.OrderID, d.Kind, d.RefID)
			if err != nil {
				return err
			}
			if err := s.txSvc.Append(ctx, tx, &transaction.Transaction{
				AccountID:      req.AccountID,
				Type:           transaction.TypeRedeem,
				Amount:         d.Amount,
				OrderID:        req.OrderID,
				IdempotencyKey: key,
			}); err != nil {
				return err
			}
		}

		detail, err := json.Marshal(plan)
		if err != nil {
			return err
		}
		now := time.Now()
		order = &Order{
			ID: req.OrderID, CustomerID: req.CustomerID, AccountID: req.AccountID,
			ProductID: req.ProductID, Scope: req.Scope, Amount: req.Amount,
			Status: StatusSettled, SettleDetail: detail, SettledAt: &now,
		}
		return s.repo.Create(tx, order)
	})
	if errors.Is(err, transaction.ErrDuplicateKey) || errors.Is(err, gorm.ErrDuplicatedKey) {
		// 并发下另一请求已结算成功，返回已有订单
		return s.Get(ctx, req.OrderID)
	}
	return order, err
}

// Get 查询订单与结算明细。
func (s *Service) Get(ctx context.Context, id string) (*Order, error) {
	o, err := s.repo.Get(s.db, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, account.ErrNotFound
	}
	return o, err
}

// toCouponInputs 转换为计费计算的中立输入。
func toCouponInputs(coupons []coupon.Coupon) []billing.CouponInput {
	inputs := make([]billing.CouponInput, 0, len(coupons))
	for _, c := range coupons {
		inputs = append(inputs, billing.CouponInput{
			ID: c.ID, Type: c.Type, Rate: c.Rate, Threshold: c.Threshold, Amount: c.Amount,
		})
	}
	return inputs
}

// toVoucherInputs 转换为计费计算的中立输入。
func toVoucherInputs(vouchers []voucher.Voucher) []billing.VoucherInput {
	inputs := make([]billing.VoucherInput, 0, len(vouchers))
	for _, v := range vouchers {
		inputs = append(inputs, billing.VoucherInput{ID: v.ID, Amount: v.Amount})
	}
	return inputs
}
