// Package gorm 交易账本存储的 GORM 实现（SQLite/PostgreSQL 方言通用）。
package gorm

import (
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
)

// TransactionRepo 交易账本 GORM 仓库。
type TransactionRepo struct{}

// NewTransactionRepo 创建交易账本 GORM 仓库。
func NewTransactionRepo() *TransactionRepo {
	return &TransactionRepo{}
}

func (r *TransactionRepo) Create(db *gorm.DB, t *transaction.Transaction) error {
	return db.Create(t).Error
}

func (r *TransactionRepo) GetByKey(db *gorm.DB, key string) (*transaction.Transaction, error) {
	var t transaction.Transaction
	if err := db.Where("idempotency_key = ?", key).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransactionRepo) ListByAccount(db *gorm.DB, accountID string, limit, offset int) ([]transaction.Transaction, error) {
	var list []transaction.Transaction
	err := db.Where("account_id = ?", accountID).
		Order("id DESC").Limit(limit).Offset(offset).Find(&list).Error
	return list, err
}

func (r *TransactionRepo) ListAllByAccount(db *gorm.DB, accountID string) ([]transaction.Transaction, error) {
	var list []transaction.Transaction
	err := db.Where("account_id = ?", accountID).
		Order("id ASC").Find(&list).Error
	return list, err
}

func (r *TransactionRepo) SumByAccount(db *gorm.DB, accountID string) (int64, error) {
	var sum int64
	err := db.Model(&transaction.Transaction{}).
		Select("COALESCE(SUM(CASE WHEN type = ? THEN amount WHEN type IN (?, ?) THEN -amount ELSE 0 END), 0)",
			transaction.TypeRecharge, transaction.TypeRefund, transaction.TypeConsume).
		Where("account_id = ?", accountID).
		Scan(&sum).Error
	return sum, err
}
