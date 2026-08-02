package transaction

import "time"

// 交易类型。
const (
	TypeRecharge = "recharge" // 充值（对公打款入账）
	TypeRefund   = "refund"   // 退款（多退登记：对公退款出账）
	TypeConsume  = "consume"  // 消费（余额支付部分）
	TypeIssue    = "issue"    // 发券（信息性记录，不影响余额）
	TypeRedeem   = "redeem"   // 核销（券抵扣部分，不影响余额）
)

// Transaction 一笔客户交易的不可变记录，是账本。
// 只插入、不更新、不删除；幂等由 IdempotencyKey 唯一约束保证。
type Transaction struct {
	ID             int64     `gorm:"primaryKey" json:"id"`
	AccountID      string    `gorm:"index;size:64" json:"account_id"`
	Type           string    `gorm:"size:16" json:"type"`
	Amount         int64     `gorm:"comment:金额（分）" json:"amount"`
	BalanceAfter   int64     `gorm:"comment:交易后余额快照（仅充值/消费有效）" json:"balance_after"`
	OrderID        string    `gorm:"size:64" json:"order_id,omitempty"`
	IdempotencyKey string    `gorm:"uniqueIndex;size:128" json:"-"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// AffectsBalance 该类型是否影响余额（发券/核销不参与余额求和）。
func (t *Transaction) AffectsBalance() bool {
	return t.Type == TypeRecharge || t.Type == TypeRefund || t.Type == TypeConsume
}

// SignedAmount 余额方向的带符号金额：充值 +，退款/消费 −，其余 0。
func (t *Transaction) SignedAmount() int64 {
	switch t.Type {
	case TypeRecharge:
		return t.Amount
	case TypeRefund, TypeConsume:
		return -t.Amount
	default:
		return 0
	}
}
