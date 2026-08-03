package reconciliation

import (
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
)

// Discrepancy 余额与交易不一致的账户。
type Discrepancy struct {
	AccountID string `json:"account_id"`
	Balance   int64  `json:"balance"`  // 账户当前余额
	Expected  int64  `json:"expected"` // 由交易推导的余额
}

// BankRow 银行流水 CSV 行。
type BankRow struct {
	VoucherNo   string `json:"voucher_no"` // 凭证号
	AmountCents int64  `json:"amount"`     // 金额（分）
	Date        string `json:"date"`       // 日期 YYYY-MM-DD
}

// BankMatch 与充值交易匹配成功的流水行。
type BankMatch struct {
	Row           BankRow `json:"row"`
	TransactionID int64   `json:"transaction_id"`
}

// BankUnmatch 未能匹配的流水行及原因。
type BankUnmatch struct {
	Row    BankRow `json:"row"`
	Reason string  `json:"reason"`
}

// BankReport 对公打款核对报告。
type BankReport struct {
	Total     int           `json:"total"`
	Matched   []BankMatch   `json:"matched"`
	Unmatched []BankUnmatch `json:"unmatched"`
}

// StatementEntry 账单流水条目（带运行余额）。
type StatementEntry struct {
	ID             int64     `json:"id"`
	Type           string    `json:"type"`
	Amount         int64     `json:"amount"`
	Note           string    `json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	RunningBalance int64     `json:"running_balance"`
}

// Statement 账户账单：期初余额 + 流水 + 期末余额。
type Statement struct {
	AccountID   string           `json:"account_id"`
	Opening     int64            `json:"opening_balance"`
	Closing     int64            `json:"closing_balance"`
	Entries     []StatementEntry `json:"entries"`
	GeneratedAt time.Time        `json:"generated_at"`
}

// toStatementEntries 将交易流水转换为账单条目并计算运行余额。
func toStatementEntries(txs []transaction.Transaction, opening int64) []StatementEntry {
	running := opening
	entries := make([]StatementEntry, 0, len(txs))
	for _, t := range txs {
		if t.AffectsBalance() {
			running += t.SignedAmount()
		}
		entries = append(entries, StatementEntry{
			ID: t.ID, Type: string(t.Type), Amount: t.Amount, Note: t.Note,
			CreatedAt: t.CreatedAt, RunningBalance: running,
		})
	}
	return entries
}
