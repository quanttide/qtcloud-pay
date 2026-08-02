package coupon

import "time"

// 优惠券类型。
const (
	TypeDiscount      = "discount"       // 折扣券：按比例优惠
	TypeFullReduction = "full_reduction" // 满减券：满足门槛后减额
)

// 适用范围。
const (
	ScopeAll     = "all"     // 全场通用
	ScopeCloud   = "cloud"   // 云服务
	ScopeCourse  = "course"  // 课程
	ScopeData    = "data"    // 数据服务
	ScopeProduct = "product" // 指定商品
)

// 状态。
const (
	StatusIssued  = "issued"  // 已发放
	StatusUsed    = "used"    // 已使用
	StatusExpired = "expired" // 已过期
)

// Coupon 优惠券：按规则抵扣的优惠手段，本身不代表钱，是一条规则。
type Coupon struct {
	ID        int64      `gorm:"primaryKey" json:"id"`
	AccountID string     `gorm:"index;size:64" json:"account_id"`
	BatchNo   string     `gorm:"index:idx_coupon_batch;size:64" json:"-"`
	Type      string     `gorm:"size:32" json:"type"`
	Rate      int        `json:"rate,omitempty"`      // 折扣券：整数百分比（90 = 9 折）
	Threshold int64      `json:"threshold,omitempty"` // 满减券：门槛（分）
	Amount    int64      `json:"amount,omitempty"`    // 满减券：减额（分）
	Scope     string     `gorm:"size:32" json:"scope"`
	ProductID string     `gorm:"size:64" json:"product_id,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	Status    string     `gorm:"size:16" json:"status"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	OrderID   string     `gorm:"size:64" json:"order_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// MatchesScope 券是否适用于指定业务与商品。
func (c *Coupon) MatchesScope(scope, productID string) bool {
	switch c.Scope {
	case ScopeAll:
		return true
	case ScopeProduct:
		return c.ProductID == productID
	default:
		return c.Scope == scope
	}
}

// Expired 是否已过有效期。
func (c *Coupon) Expired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}
