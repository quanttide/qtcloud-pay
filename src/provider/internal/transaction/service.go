package transaction

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

// ErrDuplicateKey 幂等键已存在（重复入账）。
var ErrDuplicateKey = errors.New("transaction: duplicate idempotency key")

// ErrNotFound 交易不存在。
var ErrNotFound = errors.New("transaction: not found")

// Service 交易账本服务：账本写入的唯一入口。
type Service struct {
	repo Repository
}

// NewService 创建交易账本服务。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Append 写入一条交易（只插入，不更新不删除）。
// 幂等键已存在时返回 ErrDuplicateKey，由调用方决定是否视为成功。
func (s *Service) Append(ctx context.Context, db *gorm.DB, t *Transaction) error {
	if err := s.repo.Create(db, t); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrDuplicateKey
		}
		return err
	}
	return nil
}

// Exists 幂等键是否已入账。
func (s *Service) Exists(ctx context.Context, db *gorm.DB, key string) (bool, error) {
	_, err := s.repo.GetByKey(db, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetByKey 按幂等键查询交易；不存在返回 ErrNotFound。
func (s *Service) GetByKey(ctx context.Context, db *gorm.DB, key string) (*Transaction, error) {
	t, err := s.repo.GetByKey(db, key)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return t, err
}

// List 按账户分页查询流水（id 倒序）。
func (s *Service) List(ctx context.Context, db *gorm.DB, accountID string, limit, offset int) ([]Transaction, error) {
	return s.repo.ListByAccount(db, accountID, limit, offset)
}

// ListAll 按账户查询全部流水（id 升序），供对账与账单使用。
func (s *Service) ListAll(ctx context.Context, db *gorm.DB, accountID string) ([]Transaction, error) {
	return s.repo.ListAllByAccount(db, accountID)
}

// Sum 汇总账户余额变动：Σ(充值) − Σ(余额支付)，用于一致性校验。
func (s *Service) Sum(ctx context.Context, db *gorm.DB, accountID string) (int64, error) {
	return s.repo.SumByAccount(db, accountID)
}
