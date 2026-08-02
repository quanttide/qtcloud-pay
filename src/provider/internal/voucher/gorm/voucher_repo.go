package gorm

import (
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

// VoucherRepo 代金券 GORM 仓库。
type VoucherRepo struct{}

// NewVoucherRepo 创建代金券 GORM 仓库。
func NewVoucherRepo() *VoucherRepo {
	return &VoucherRepo{}
}

func (r *VoucherRepo) CreateBatch(db *gorm.DB, vouchers []*voucher.Voucher) error {
	return db.Create(vouchers).Error
}

func (r *VoucherRepo) Get(db *gorm.DB, id int64) (*voucher.Voucher, error) {
	var v voucher.Voucher
	if err := db.Where("id = ?", id).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VoucherRepo) GetForUpdate(db *gorm.DB, id int64) (*voucher.Voucher, error) {
	var v voucher.Voucher
	if err := db.Where("id = ?", id).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VoucherRepo) Update(db *gorm.DB, v *voucher.Voucher) error {
	return db.Model(v).Updates(map[string]any{
		"status":   v.Status,
		"used_at":  v.UsedAt,
		"order_id": v.OrderID,
	}).Error
}

func (r *VoucherRepo) ListByAccount(db *gorm.DB, accountID string) ([]voucher.Voucher, error) {
	var list []voucher.Voucher
	err := db.Where("account_id = ?", accountID).
		Order("id DESC").Find(&list).Error
	return list, err
}

func (r *VoucherRepo) CountByBatch(db *gorm.DB, batchNo string) (int64, error) {
	var n int64
	err := db.Model(&voucher.Voucher{}).
		Where("batch_no = ?", batchNo).Count(&n).Error
	return n, err
}

var _ voucher.Repository = (*VoucherRepo)(nil)
