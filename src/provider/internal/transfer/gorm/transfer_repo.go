package gorm

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transfer"
)

// TransferRepo 代金券转赠 GORM 仓库。
type TransferRepo struct{}

// NewTransferRepo 创建代金券转赠仓库。
func NewTransferRepo() *TransferRepo {
	return &TransferRepo{}
}

func (r *TransferRepo) Create(db *gorm.DB, t *transfer.VoucherTransfer) error {
	return db.Create(t).Error
}

func (r *TransferRepo) Get(db *gorm.DB, id int64) (*transfer.VoucherTransfer, error) {
	var t transfer.VoucherTransfer
	if err := db.Where("id = ?", id).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransferRepo) GetForUpdate(db *gorm.DB, id int64) (*transfer.VoucherTransfer, error) {
	var t transfer.VoucherTransfer
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransferRepo) GetByIdempotencyKey(db *gorm.DB, key string) (*transfer.VoucherTransfer, error) {
	var t transfer.VoucherTransfer
	if err := db.Where("idempotency_key = ?", key).First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *TransferRepo) Update(db *gorm.DB, t *transfer.VoucherTransfer) error {
	return db.Model(t).Updates(map[string]any{
		"status":                t.Status,
		"reviewed_by":           t.ReviewedBy,
		"reviewed_at":           t.ReviewedAt,
		"issued_voucher_id":     t.IssuedVoucherID,
		"refund_transaction_id": t.RefundTransactionID,
		"refunded_by":           t.RefundedBy,
		"refunded_at":           t.RefundedAt,
		"note":                  t.Note,
		"updated_at":            t.UpdatedAt,
	}).Error
}

func (r *TransferRepo) List(db *gorm.DB, filter transfer.ListFilter) ([]transfer.VoucherTransfer, error) {
	q := db.Model(&transfer.VoucherTransfer{})
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	if filter.AccountID != "" {
		q = q.Where("from_account_id = ? OR to_account_id = ?", filter.AccountID, filter.AccountID)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var list []transfer.VoucherTransfer
	err := q.Order("id DESC").Limit(limit).Offset(filter.Offset).Find(&list).Error
	return list, err
}

var _ transfer.Repository = (*TransferRepo)(nil)
