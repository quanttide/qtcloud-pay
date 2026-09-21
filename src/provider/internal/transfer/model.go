package transfer

import "time"

const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

const (
	DecisionApproved = "approved"
	DecisionRejected = "rejected"
)

// VoucherTransfer 代金券转赠记录。
//
// 转赠不是直接改写券归属，而是「购买人余额消费 -> 审核 -> 向受赠人再发行」的审计链。
type VoucherTransfer struct {
	ID                  int64      `gorm:"primaryKey" json:"id"`
	FromAccountID       string     `gorm:"index;size:64;not null" json:"from_account_id"`
	ToAccountID         string     `gorm:"index;size:64;not null" json:"to_account_id"`
	SourceTransactionID int64      `gorm:"index;not null" json:"source_transaction_id"`
	Amount              int64      `gorm:"not null" json:"amount"`      // 购买金额（分）
	TaxRateBP           int        `gorm:"not null" json:"tax_rate_bp"` // 税率，万分比；当前为 0
	Status              string     `gorm:"index;size:16;not null" json:"status"`
	ReviewedBy          string     `gorm:"size:128" json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time `json:"reviewed_at,omitempty"`
	IssuedVoucherID     *int64     `gorm:"index" json:"issued_voucher_id,omitempty"`
	Note                string     `gorm:"type:text" json:"note,omitempty"`
	IdempotencyKey      string     `gorm:"uniqueIndex;size:128;not null" json:"idempotency_key"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// Terminal 是否已经进入终态。
func (t *VoucherTransfer) Terminal() bool {
	return t.Status == StatusApproved || t.Status == StatusRejected
}

// IssuedAmountCents 按当前税率计算再发行金额。
// 金额以整数分运算；除税后出现小数分时向下取整，避免浮点误差。
func (t *VoucherTransfer) IssuedAmountCents() int64 {
	return t.Amount * int64(10000-t.TaxRateBP) / 10000
}

// ListFilter 转赠列表查询条件。
type ListFilter struct {
	Status    string
	AccountID string
	Limit     int
	Offset    int
}
