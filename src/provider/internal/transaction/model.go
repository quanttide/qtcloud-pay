package transaction

import (
	"time"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/ledger"
)

// 交易类型（契约见工具库 pkg/ledger；本地类型化别名保持内部 API 不变）。
const (
	TypeRecharge = ledger.TypeRecharge // 充值（对公打款入账）
	TypeRefund   = ledger.TypeRefund   // 退款（多退登记：对公退款出账）
	TypeConsume  = ledger.TypeConsume  // 消费（余额支付部分）
	TypeIssue    = ledger.TypeIssue    // 发券（信息性记录，不影响余额）
	TypeRedeem   = ledger.TypeRedeem   // 核销（券抵扣部分，不影响余额）
)

// Transaction 一笔客户交易的不可变记录，是账本。
// 只插入、不更新、不删除；幂等由 IdempotencyKey 唯一约束保证。
// JSON 形状以工具库 pkg/ledger 的 Transaction 契约为准（本结构附加 gorm 存储约束）。
type Transaction struct {
	ID             int64       `gorm:"primaryKey" json:"id"`
	AccountID      string      `gorm:"index;size:64" json:"account_id"`
	Type           ledger.Type `gorm:"size:16" json:"type"`
	Amount         int64       `gorm:"comment:金额（分）" json:"amount"`
	BalanceAfter   int64       `gorm:"comment:交易后余额快照（仅充值/消费有效）" json:"balance_after"`
	OrderID        string      `gorm:"size:64" json:"order_id,omitempty"`
	IdempotencyKey string      `gorm:"uniqueIndex;size:128" json:"-"`
	Note           string      `json:"note,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

// AffectsBalance 该类型是否影响余额（发券/核销不参与余额求和）。
func (t *Transaction) AffectsBalance() bool {
	return ledger.AffectsBalance(t.Type)
}

// SignedAmount 余额方向的带符号金额：充值 +，退款/消费 −，其余 0。
func (t *Transaction) SignedAmount() int64 {
	return ledger.SignedAmount(t.Type, t.Amount)
}

// Contract 返回工具库账本契约视图（跨应用流转的统一形状，见 pkg/ledger）。
func (t *Transaction) Contract() ledger.Transaction {
	return ledger.Transaction{
		ID:             t.ID,
		AccountID:      t.AccountID,
		Type:           t.Type,
		Amount:         t.Amount,
		BalanceAfter:   t.BalanceAfter,
		OrderID:        t.OrderID,
		IdempotencyKey: t.IdempotencyKey,
		Note:           t.Note,
		CreatedAt:      t.CreatedAt,
	}
}
