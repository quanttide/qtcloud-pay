package billing

import (
	"context"

	"gorm.io/gorm"

	billingtoolkit "github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/billing"
)

// 错误（契约来自工具库 pkg/billing）。
var (
	// ErrInsufficientBalance 余额不足，无法完成结算。
	ErrInsufficientBalance = billingtoolkit.ErrInsufficientBalance
	// ErrInvalidAmount 订单金额必须为正整数（分）。
	ErrInvalidAmount = billingtoolkit.ErrInvalidAmount
)

// Service 计费规则服务：抵扣计算（委托工具库纯函数）与规则表查询。
type Service struct {
	repo Repository
}

// NewService 创建计费规则服务。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Calculate 结算计算：委托工具库 pkg/billing 的纯函数。
// 抵扣顺序与力度规则见工具库契约（满减 → 折扣 → 代金券 → 余额）。
func (s *Service) Calculate(amount int64, coupons []CouponInput, vouchers []VoucherInput, balance int64) ([]Deduction, error) {
	return billingtoolkit.Calculate(amount, coupons, vouchers, balance)
}

// Rules 查询计费规则（按优先级排序；v0.1.0 为规则表预留）。
func (s *Service) Rules(ctx context.Context, db *gorm.DB) ([]BillingRule, error) {
	return s.repo.List(db)
}
