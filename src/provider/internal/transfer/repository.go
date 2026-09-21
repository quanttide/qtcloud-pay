package transfer

import "gorm.io/gorm"

// Repository 转赠存储接口。
type Repository interface {
	Create(db *gorm.DB, t *VoucherTransfer) error
	Get(db *gorm.DB, id int64) (*VoucherTransfer, error)
	GetForUpdate(db *gorm.DB, id int64) (*VoucherTransfer, error)
	GetByIdempotencyKey(db *gorm.DB, key string) (*VoucherTransfer, error)
	Update(db *gorm.DB, t *VoucherTransfer) error
	List(db *gorm.DB, filter ListFilter) ([]VoucherTransfer, error)
}
