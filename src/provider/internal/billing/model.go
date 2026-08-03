package billing

import (
	billingtoolkit "github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/billing"
)

// 抵扣类型（契约来自工具库 pkg/billing）。
const (
	KindCoupon  = billingtoolkit.KindCoupon  // 优惠券抵扣
	KindVoucher = billingtoolkit.KindVoucher // 代金券抵现
	KindBalance = billingtoolkit.KindBalance // 余额支付
)

// CouponInput 参与结算计算的优惠券（中立输入，契约来自工具库）。
type CouponInput = billingtoolkit.CouponInput

// VoucherInput 参与结算计算的代金券。
type VoucherInput = billingtoolkit.VoucherInput

// Deduction 一项抵扣。
type Deduction = billingtoolkit.Deduction

// BillingRule 计费规则：抵扣顺序配置（v0.1.0 提供默认顺序，规则引擎后置）。
type BillingRule struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	Priority  int    `json:"priority"`                             // 执行顺序
	Kind      string `gorm:"size:16" json:"kind"`                  // coupon / voucher / balance
	Condition string `gorm:"type:text" json:"condition,omitempty"` // JSON 条件
}
