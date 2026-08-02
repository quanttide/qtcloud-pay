package voucher

import "time"

// 适用范围。
const (
	ScopeAll     = "all"
	ScopeCloud   = "cloud"
	ScopeCourse  = "course"
	ScopeData    = "data"
	ScopeProduct = "product"
)

// 状态。
const (
	StatusIssued  = "issued"
	StatusUsed    = "used"
	StatusExpired = "expired"
)

// Voucher 代金券：直接抵现的优惠手段，本身**就是钱**，结算时直接抵减应付款项。
type Voucher struct {
	ID        int64      `gorm:"primaryKey" json:"id"`
	AccountID string     `gorm:"index;size:64" json:"account_id"`
	BatchNo   string     `gorm:"index:idx_voucher_batch;size:64" json:"-"`
	Amount    int64      `json:"amount"` // 面值（分）
	Scope     string     `gorm:"size:32" json:"scope"`
	ProductID string     `gorm:"size:64" json:"product_id,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	Status    string     `gorm:"size:16" json:"status"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	OrderID   string     `gorm:"size:64" json:"order_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// MatchesScope 券是否适用于指定业务与商品。
func (v *Voucher) MatchesScope(scope, productID string) bool {
	switch v.Scope {
	case ScopeAll:
		return true
	case ScopeProduct:
		return v.ProductID == productID
	default:
		return v.Scope == scope
	}
}

// Expired 是否已过有效期。
func (v *Voucher) Expired(now time.Time) bool {
	return now.After(v.ExpiresAt)
}
