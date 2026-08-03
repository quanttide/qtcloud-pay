package voucher

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/idempotency"
)

var (
	// ErrUnavailable 代金券不存在、已使用或已过期，不可抵现。
	ErrUnavailable = errors.New("voucher: unavailable")
	// ErrInvalidIssue 发放请求不合法。
	ErrInvalidIssue = errors.New("voucher: invalid issue request")
)

// maxBatchCount 单次发放数量上限。
const maxBatchCount = 1000

// IssueRequest 发放请求。
type IssueRequest struct {
	AccountID string
	Amount    int64 // 面值（分）
	Scope     string
	ProductID string
	ExpiresAt time.Time
	Count     int
	BatchNo   string
	Note      string
}

// Service 代金券服务。
type Service struct {
	db    *gorm.DB
	repo  Repository
	txSvc *transaction.Service
}

// NewService 创建代金券服务。
func NewService(db *gorm.DB, repo Repository, txSvc *transaction.Service) *Service {
	return &Service{db: db, repo: repo, txSvc: txSvc}
}

// Issue 批量发放代金券（幂等：同一批次号只发一次）。
// 发放本身生成一条发券交易，保证账本完整（不丢）。
func (s *Service) Issue(ctx context.Context, req *IssueRequest) error {
	if err := validateIssue(req); err != nil {
		return err
	}
	key, err := idempotency.Key(idempotency.IssueVoucher, req.BatchNo)
	if err != nil {
		return err
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		exists, err := s.txSvc.Exists(ctx, tx, key)
		if err != nil {
			return err
		}
		if exists {
			return nil // 幂等：该批次已发放
		}
		n, err := s.repo.CountByBatch(tx, req.BatchNo)
		if err != nil {
			return err
		}
		if n > 0 {
			return nil // 幂等：券已存在（防御分支）
		}
		vouchers := buildVouchers(req)
		if err := s.repo.CreateBatch(tx, vouchers); err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil // 并发下另一请求已发放
			}
			return err
		}
		return s.txSvc.Append(ctx, tx, &transaction.Transaction{
			AccountID:      req.AccountID,
			Type:           transaction.TypeIssue,
			Amount:         int64(len(vouchers)) * req.Amount,
			IdempotencyKey: key,
			Note:           req.Note,
		})
	})
	if errors.Is(err, transaction.ErrDuplicateKey) {
		return nil // 并发下另一请求已发放（发券交易幂等键冲突），本请求整体回滚，视为成功
	}
	return err
}

// List 查询账户代金券（id 倒序），并惰性流转过期状态。
func (s *Service) List(ctx context.Context, accountID string) ([]Voucher, error) {
	list, err := s.repo.ListByAccount(s.db, accountID)
	if err != nil {
		return nil, err
	}
	for i := range list {
		s.expireIfNeeded(&list[i])
	}
	return list, nil
}

// Available 返回账户当前可用的代金券（已发放、未过期、适用范围匹配），供结算计算使用。
func (s *Service) Available(ctx context.Context, db *gorm.DB, accountID, scope, productID string) ([]Voucher, error) {
	list, err := s.repo.ListByAccount(db, accountID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	available := make([]Voucher, 0, len(list))
	for _, v := range list {
		if v.Status != StatusIssued || v.Expired(now) || !v.MatchesScope(scope, productID) {
			continue
		}
		available = append(available, v)
	}
	return available, nil
}

// Use 抵现一张代金券（供结算编排调用）：校验已发放且未过期，置为已使用并关联订单。
func (s *Service) Use(ctx context.Context, db *gorm.DB, id int64, orderID string) error {
	v, err := s.repo.GetForUpdate(db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUnavailable
		}
		return err
	}
	if v.Status != StatusIssued || v.Expired(time.Now()) {
		return ErrUnavailable
	}
	now := time.Now()
	v.Status, v.UsedAt, v.OrderID = StatusUsed, &now, orderID
	return s.repo.Update(db, v)
}

// expireIfNeeded 惰性流转：发现已过期则更新状态（不做定时任务）。
func (s *Service) expireIfNeeded(v *Voucher) {
	if v.Status == StatusIssued && v.Expired(time.Now()) {
		v.Status = StatusExpired
		_ = s.repo.Update(s.db, v)
	}
}

// validateIssue 校验发放请求。
func validateIssue(req *IssueRequest) error {
	switch {
	case req == nil || req.AccountID == "" || req.BatchNo == "":
		return ErrInvalidIssue
	case req.Amount <= 0:
		return ErrInvalidIssue
	case req.Count <= 0 || req.Count > maxBatchCount:
		return ErrInvalidIssue
	case !req.ExpiresAt.After(time.Now()):
		return ErrInvalidIssue
	}
	switch req.Scope {
	case ScopeAll, ScopeCloud, ScopeCourse, ScopeData:
	case ScopeProduct:
		if req.ProductID == "" {
			return ErrInvalidIssue
		}
	default:
		return ErrInvalidIssue
	}
	return nil
}

// buildVouchers 按发放请求生成 count 张代金券。
func buildVouchers(req *IssueRequest) []*Voucher {
	vouchers := make([]*Voucher, 0, req.Count)
	for i := 0; i < req.Count; i++ {
		vouchers = append(vouchers, &Voucher{
			AccountID: req.AccountID,
			BatchNo:   req.BatchNo,
			Amount:    req.Amount,
			Scope:     req.Scope,
			ProductID: req.ProductID,
			ExpiresAt: req.ExpiresAt,
			Status:    StatusIssued,
		})
	}
	return vouchers
}
