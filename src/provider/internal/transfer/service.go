package transfer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/idempotency"
)

var (
	ErrInvalidRequest = errors.New("transfer: invalid request")
	ErrNotFound       = errors.New("transfer: not found")
	ErrInvalidStatus  = errors.New("transfer: invalid status")
)

const (
	transferIdempotencyBiz = "transfer"
	defaultTaxRateBP       = 0
	defaultVoucherYears    = 1
)

type accountSvc interface {
	Lock(ctx context.Context, db *gorm.DB, id string) (*account.Account, error)
	Save(ctx context.Context, db *gorm.DB, a *account.Account) error
}

type voucherSvc interface {
	IssueWithDB(ctx context.Context, db *gorm.DB, req *voucher.IssueRequest) ([]voucher.Voucher, error)
}

type transactionSvc interface {
	Append(ctx context.Context, db *gorm.DB, t *transaction.Transaction) error
	GetByKey(ctx context.Context, db *gorm.DB, key string) (*transaction.Transaction, error)
}

// CreateRequest 创建转赠请求。
type CreateRequest struct {
	FromAccountID  string
	ToAccountID    string
	Amount         int64
	Note           string
	IdempotencyKey string
}

// ReviewRequest 审核转赠请求。
type ReviewRequest struct {
	Decision   string
	ReviewedBy string
	Note       string
}

// RefundRequest 办理拒绝转赠后的人工退款留痕请求。
type RefundRequest struct {
	RefundedBy string
	Note       string
}

// Service 代金券转赠服务。
type Service struct {
	db         *gorm.DB
	repo       Repository
	accountSvc accountSvc
	voucherSvc voucherSvc
	txSvc      transactionSvc
}

// NewService 创建转赠服务。
func NewService(db *gorm.DB, repo Repository, accountSvc accountSvc, voucherSvc voucherSvc, txSvc transactionSvc) *Service {
	return &Service{db: db, repo: repo, accountSvc: accountSvc, voucherSvc: voucherSvc, txSvc: txSvc}
}

// Create 创建转赠：购买人余额消费 + pending 转赠记录，同事务提交。
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*VoucherTransfer, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}
	txKey, err := transferTransactionKey(req.IdempotencyKey)
	if err != nil {
		return nil, ErrInvalidRequest
	}
	var created *VoucherTransfer
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := s.repo.GetByIdempotencyKey(tx, req.IdempotencyKey)
		if err == nil {
			created = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		acc, err := s.accountSvc.Lock(ctx, tx, req.FromAccountID)
		if err != nil {
			return err
		}
		if acc.Balance < req.Amount {
			return account.ErrInsufficientBalance
		}
		if err := ensureAccountExists(tx, req.ToAccountID); err != nil {
			return err
		}
		acc.Balance -= req.Amount
		if err := s.accountSvc.Save(ctx, tx, acc); err != nil {
			return err
		}
		sourceTx := &transaction.Transaction{
			AccountID:      req.FromAccountID,
			Type:           transaction.TypeConsume,
			Amount:         req.Amount,
			BalanceAfter:   acc.Balance,
			IdempotencyKey: txKey,
			Note:           transferPurchaseNote(req.Note),
		}
		if err := s.txSvc.Append(ctx, tx, sourceTx); err != nil {
			return err
		}
		created = &VoucherTransfer{
			FromAccountID:       req.FromAccountID,
			ToAccountID:         req.ToAccountID,
			SourceTransactionID: sourceTx.ID,
			Amount:              req.Amount,
			TaxRateBP:           defaultTaxRateBP,
			Status:              StatusPending,
			Note:                strings.TrimSpace(req.Note),
			IdempotencyKey:      strings.TrimSpace(req.IdempotencyKey),
		}
		return s.repo.Create(tx, created)
	})
	if errors.Is(err, transaction.ErrDuplicateKey) || errors.Is(err, gorm.ErrDuplicatedKey) {
		return s.repo.GetByIdempotencyKey(s.db.WithContext(ctx), req.IdempotencyKey)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, account.ErrNotFound
	}
	return created, err
}

// Review 审核转赠；approved 会向受赠账户再发行一张代金券。
func (s *Service) Review(ctx context.Context, id int64, req *ReviewRequest) (*VoucherTransfer, error) {
	if id <= 0 || req == nil || strings.TrimSpace(req.ReviewedBy) == "" {
		return nil, ErrInvalidRequest
	}
	decision := strings.TrimSpace(req.Decision)
	if decision != DecisionApproved && decision != DecisionRejected {
		return nil, ErrInvalidRequest
	}
	var reviewed *VoucherTransfer
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := s.repo.GetForUpdate(tx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Terminal() {
			reviewed = t
			return nil
		}
		if t.Status != StatusPending {
			return ErrInvalidStatus
		}
		now := time.Now()
		t.ReviewedBy = strings.TrimSpace(req.ReviewedBy)
		t.ReviewedAt = &now
		t.Note = mergeReviewNote(t.Note, req.Note)
		switch decision {
		case DecisionRejected:
			t.Status = StatusRejected
		case DecisionApproved:
			issuedAmount := t.IssuedAmountCents()
			if issuedAmount <= 0 {
				return ErrInvalidRequest
			}
			// 转赠再发行暂按 1 年有效期；后续若运营规则明确，可在配置层策略化。
			issued, err := s.voucherSvc.IssueWithDB(ctx, tx, &voucher.IssueRequest{
				AccountID: t.ToAccountID,
				Amount:    issuedAmount,
				Scope:     voucher.ScopeAll,
				ExpiresAt: now.AddDate(defaultVoucherYears, 0, 0),
				Count:     1,
				BatchNo:   transferBatchNo(t.ID),
				Note:      fmt.Sprintf("转赠再发行：transfer_id=%d", t.ID),
			})
			if err != nil {
				return err
			}
			if len(issued) != 1 {
				return ErrInvalidStatus
			}
			t.Status = StatusApproved
			t.IssuedVoucherID = &issued[0].ID
		}
		t.UpdatedAt = now
		if err := s.repo.Update(tx, t); err != nil {
			return err
		}
		reviewed = t
		return nil
	})
	return reviewed, err
}

// Refund 对 rejected 转赠办理人工退款留痕：购买人余额加回 + 写入正向入账流水。
func (s *Service) Refund(ctx context.Context, id int64, req *RefundRequest) (*VoucherTransfer, error) {
	if id <= 0 || req == nil || strings.TrimSpace(req.RefundedBy) == "" {
		return nil, ErrInvalidRequest
	}
	var refunded *VoucherTransfer
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		t, err := s.repo.GetForUpdate(tx, id)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if t.Status != StatusRejected {
			return ErrInvalidStatus
		}
		if t.RefundTransactionID != nil {
			refunded = t
			return nil
		}
		acc, err := s.accountSvc.Lock(ctx, tx, t.FromAccountID)
		if err != nil {
			return err
		}
		acc.Balance += t.Amount
		if err := s.accountSvc.Save(ctx, tx, acc); err != nil {
			return err
		}
		key, err := transferRefundTransactionKey(t.ID)
		if err != nil {
			return err
		}
		refundTx := &transaction.Transaction{
			AccountID:      t.FromAccountID,
			Type:           transaction.TypeRecharge,
			Amount:         t.Amount,
			BalanceAfter:   acc.Balance,
			IdempotencyKey: key,
			Note:           transferRefundNote(t.ID, req.Note),
		}
		if err := s.txSvc.Append(ctx, tx, refundTx); err != nil {
			return err
		}
		now := time.Now()
		t.RefundTransactionID = &refundTx.ID
		t.RefundedBy = strings.TrimSpace(req.RefundedBy)
		t.RefundedAt = &now
		t.Note = mergeRefundNote(t.Note, req.Note)
		t.UpdatedAt = now
		if err := s.repo.Update(tx, t); err != nil {
			return err
		}
		refunded = t
		return nil
	})
	if errors.Is(err, transaction.ErrDuplicateKey) || errors.Is(err, gorm.ErrDuplicatedKey) {
		return s.repo.Get(s.db.WithContext(ctx), id)
	}
	return refunded, err
}

// Get 查询转赠详情。
func (s *Service) Get(ctx context.Context, id int64) (*VoucherTransfer, error) {
	t, err := s.repo.Get(s.db.WithContext(ctx), id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return t, err
}

// List 查询转赠列表。
func (s *Service) List(ctx context.Context, filter ListFilter) ([]VoucherTransfer, error) {
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, ErrInvalidRequest
	}
	if filter.Offset < 0 {
		return nil, ErrInvalidRequest
	}
	return s.repo.List(s.db.WithContext(ctx), filter)
}

func validateCreate(req *CreateRequest) error {
	if req == nil {
		return ErrInvalidRequest
	}
	if strings.TrimSpace(req.FromAccountID) == "" || strings.TrimSpace(req.ToAccountID) == "" {
		return ErrInvalidRequest
	}
	if req.Amount <= 0 || strings.TrimSpace(req.IdempotencyKey) == "" {
		return ErrInvalidRequest
	}
	req.FromAccountID = strings.TrimSpace(req.FromAccountID)
	req.ToAccountID = strings.TrimSpace(req.ToAccountID)
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	req.Note = strings.TrimSpace(req.Note)
	return nil
}

func ensureAccountExists(db *gorm.DB, accountID string) error {
	var count int64
	if err := db.Model(&account.Account{}).Where("id = ?", accountID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return account.ErrNotFound
	}
	return nil
}

func transferTransactionKey(raw string) (string, error) {
	return idempotency.Key(transferIdempotencyBiz, raw)
}

func transferRefundTransactionKey(id int64) (string, error) {
	return idempotency.Key("transfer-refund", fmt.Sprintf("%d", id))
}

func transferBatchNo(id int64) string {
	return fmt.Sprintf("transfer-%d", id)
}

func transferPurchaseNote(note string) string {
	note = strings.TrimSpace(note)
	if note == "" {
		return "代金券转赠购买"
	}
	return "代金券转赠购买：" + note
}

func mergeReviewNote(existing, review string) string {
	existing = strings.TrimSpace(existing)
	review = strings.TrimSpace(review)
	if review == "" {
		return existing
	}
	if existing == "" {
		return review
	}
	return existing + "\n审核备注：" + review
}

func transferRefundNote(id int64, note string) string {
	base := fmt.Sprintf("转赠作废退款：transfer_id=%d", id)
	note = strings.TrimSpace(note)
	if note == "" {
		return base
	}
	return base + "；" + note
}

func mergeRefundNote(existing, refund string) string {
	existing = strings.TrimSpace(existing)
	refund = strings.TrimSpace(refund)
	if refund == "" {
		return existing
	}
	if existing == "" {
		return refund
	}
	return existing + "\n退款备注：" + refund
}

func validStatus(status string) bool {
	switch status {
	case StatusPending, StatusApproved, StatusRejected:
		return true
	default:
		return false
	}
}
