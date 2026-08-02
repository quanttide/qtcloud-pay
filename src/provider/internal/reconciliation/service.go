package reconciliation

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
)

// ErrInvalidCSV 银行流水 CSV 格式不合法。
var ErrInvalidCSV = errors.New("reconciliation: invalid bank csv")

// Service 对账与可查服务。
type Service struct {
	db         *gorm.DB
	accountSvc *account.Service
	txSvc      *transaction.Service
}

// NewService 创建对账服务。
func NewService(db *gorm.DB, accountSvc *account.Service, txSvc *transaction.Service) *Service {
	return &Service{db: db, accountSvc: accountSvc, txSvc: txSvc}
}

// CheckConsistency 一致性校验：逐账户比对「余额字段」与「Σ(充值) − Σ(余额支付)」。
func (s *Service) CheckConsistency(ctx context.Context) ([]Discrepancy, error) {
	accounts, err := s.accountSvc.List(ctx)
	if err != nil {
		return nil, err
	}
	var discrepancies []Discrepancy
	for _, a := range accounts {
		sum, err := s.txSvc.Sum(ctx, s.db, a.ID)
		if err != nil {
			return nil, err
		}
		if a.Balance != sum {
			discrepancies = append(discrepancies, Discrepancy{
				AccountID: a.ID, Balance: a.Balance, Expected: sum,
			})
		}
	}
	return discrepancies, nil
}

// ReconcileBankFile 对公打款核对：解析银行流水 CSV，与充值交易按凭证号比对。
//
// CSV 格式：`voucher_no,amount_cents,date`（首行为表头时可省略；金额为整数分）。
func (s *Service) ReconcileBankFile(ctx context.Context, r io.Reader) (*BankReport, error) {
	rows, err := parseBankCSV(r)
	if err != nil {
		return nil, err
	}
	report := &BankReport{Total: len(rows)}
	for _, row := range rows {
		tx, err := s.txSvc.GetByKey(ctx, s.db, "recharge:"+row.VoucherNo)
		if errors.Is(err, transaction.ErrNotFound) {
			report.Unmatched = append(report.Unmatched, BankUnmatch{Row: row, Reason: "未找到对应充值交易"})
			continue
		}
		if err != nil {
			return nil, err
		}
		if tx.Amount != row.AmountCents {
			report.Unmatched = append(report.Unmatched, BankUnmatch{Row: row, Reason: "金额不一致"})
			continue
		}
		report.Matched = append(report.Matched, BankMatch{Row: row, TransactionID: tx.ID})
	}
	return report, nil
}

// Statement 账户账单：期初余额 + 流水（运行余额）+ 期末余额。
func (s *Service) Statement(ctx context.Context, accountID string) (*Statement, error) {
	acc, err := s.accountSvc.Get(ctx, accountID)
	if err != nil {
		return nil, err
	}
	txs, err := s.txSvc.ListAll(ctx, s.db, accountID)
	if err != nil {
		return nil, err
	}
	var net int64
	for _, t := range txs {
		net += t.SignedAmount()
	}
	opening := acc.Balance - net
	return &Statement{
		AccountID:   accountID,
		Opening:     opening,
		Closing:     acc.Balance,
		Entries:     toStatementEntries(txs, opening),
		GeneratedAt: time.Now(),
	}, nil
}

// parseBankCSV 解析银行流水 CSV（纯函数）。
func parseBankCSV(r io.Reader) ([]BankRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCSV, err)
	}
	var rows []BankRow
	for i, rec := range records {
		if i == 0 && rec[0] == "voucher_no" {
			continue // 表头
		}
		amount, err := strconv.ParseInt(rec[1], 10, 64)
		if err != nil || amount <= 0 {
			return nil, fmt.Errorf("%w: line %d: invalid amount", ErrInvalidCSV, i+1)
		}
		if rec[0] == "" || rec[2] == "" {
			return nil, fmt.Errorf("%w: line %d: missing field", ErrInvalidCSV, i+1)
		}
		rows = append(rows, BankRow{VoucherNo: rec[0], AmountCents: amount, Date: rec[2]})
	}
	return rows, nil
}
