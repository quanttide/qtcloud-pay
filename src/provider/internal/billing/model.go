package billing

// 抵扣类型。
const (
	KindCoupon   = "coupon"   // 优惠券抵扣
	KindVoucher  = "voucher"  // 代金券抵现
	KindBalance  = "balance"  // 余额支付
)

// CouponInput 参与结算计算的优惠券（billing 不依赖 coupon 模块，使用中立输入）。
type CouponInput struct {
	ID        int64
	Type      string // coupon.TypeDiscount / coupon.TypeFullReduction
	Rate      int    // 折扣券：整数百分比
	Threshold int64  // 满减券：门槛（分）
	Amount    int64  // 满减券：减额（分）
}

// VoucherInput 参与结算计算的代金券。
type VoucherInput struct {
	ID     int64
	Amount int64 // 面值（分）
}

// Deduction 一项抵扣。
type Deduction struct {
	Kind   string `json:"kind"`             // coupon / voucher / balance
	RefID  int64  `json:"ref_id,omitempty"` // 券 ID（balance 时为 0）
	Amount int64  `json:"amount"`           // 抵扣额（分）
}

// BillingRule 计费规则：抵扣顺序配置（v0.1.0 提供默认顺序，规则引擎后置）。
type BillingRule struct {
	ID        int64  `gorm:"primaryKey" json:"id"`
	Priority  int    `json:"priority"`             // 执行顺序
	Kind      string `gorm:"size:16" json:"kind"`  // coupon / voucher / balance
	Condition string `gorm:"type:text" json:"condition,omitempty"` // JSON 条件
}
