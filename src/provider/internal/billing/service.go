package billing

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

var (
	// ErrInsufficientBalance 余额不足，无法完成结算。
	ErrInsufficientBalance = errors.New("billing: insufficient balance")
	// ErrInvalidAmount 订单金额必须为正整数（分）。
	ErrInvalidAmount = errors.New("billing: invalid amount")
)

// Service 计费规则服务：抵扣顺序配置与抵扣计算（纯计算）。
type Service struct {
	repo Repository
}

// NewService 创建计费规则服务。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Calculate 结算计算（纯函数，无 I/O）：给定订单金额与可用券/余额，输出逐项抵扣明细。
//
// v0.1.0 默认顺序（规则引擎后置）：
//  1. 满减券：满足门槛（≤ 剩余应付）中力度最大的一张
//  2. 折扣券：按 rate 优惠（9 折 = rate 90 = 省 10%），向下取整
//  3. 代金券：逐张抵扣 min(面值, 剩余应付)
//  4. 余额：补足剩余
//
// 余额不足返回 ErrInsufficientBalance。
func (s *Service) Calculate(amount int64, coupons []CouponInput, vouchers []VoucherInput, balance int64) ([]Deduction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	remain := amount
	var plan []Deduction

	// 1. 满减券（力度最大的一张）
	if c := bestFullReduction(coupons, remain); c != nil {
		plan = append(plan, Deduction{Kind: KindCoupon, RefID: c.ID, Amount: c.Amount})
		remain -= c.Amount
	}

	// 2. 折扣券（折扣力度最大的一张）：省 (100−rate)%
	if c := bestDiscount(coupons); c != nil {
		discount := remain * int64(100-c.Rate) / 100
		if discount > 0 {
			plan = append(plan, Deduction{Kind: KindCoupon, RefID: c.ID, Amount: discount})
			remain -= discount
		}
	}

	// 3. 代金券逐张抵现
	for _, v := range vouchers {
		if remain == 0 {
			break
		}
		d := min(v.Amount, remain)
		plan = append(plan, Deduction{Kind: KindVoucher, RefID: v.ID, Amount: d})
		remain -= d
	}

	// 4. 余额补足
	if remain > balance {
		return nil, ErrInsufficientBalance
	}
	if remain > 0 {
		plan = append(plan, Deduction{Kind: KindBalance, Amount: remain})
	}
	return plan, nil
}

// Rules 查询计费规则（按优先级排序；v0.1.0 为规则表预留）。
func (s *Service) Rules(ctx context.Context, db *gorm.DB) ([]BillingRule, error) {
	return s.repo.List(db)
}

// bestFullReduction 满足门槛（≤ remain）中减额最大的一张满减券；无则返回 nil。
func bestFullReduction(coupons []CouponInput, remain int64) *CouponInput {
	var best *CouponInput
	for i := range coupons {
		c := &coupons[i]
		if c.Type == "full_reduction" && c.Threshold <= remain && c.Amount > 0 {
			if best == nil || c.Amount > best.Amount {
				best = c
			}
		}
	}
	return best
}

// bestDiscount 折扣力度最大（rate 最低）的一张折扣券；无则返回 nil。
func bestDiscount(coupons []CouponInput) *CouponInput {
	var best *CouponInput
	for i := range coupons {
		c := &coupons[i]
		if c.Type == "discount" && c.Rate > 0 && c.Rate <= 100 {
			if best == nil || c.Rate < best.Rate {
				best = c
			}
		}
	}
	return best
}
