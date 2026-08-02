package coupon

import "gorm.io/gorm"

// Repository 优惠券存储接口。
// 方法统一以 *gorm.DB 为首参（可传共享连接或 *gorm.Tx），事务由调用方编排。
type Repository interface {
	// CreateBatch 批量插入（同一批次）；批次号唯一冲突时返回 gorm.ErrDuplicatedKey。
	CreateBatch(db *gorm.DB, coupons []*Coupon) error
	Get(db *gorm.DB, id int64) (*Coupon, error)
	GetForUpdate(db *gorm.DB, id int64) (*Coupon, error)
	Update(db *gorm.DB, c *Coupon) error
	ListByAccount(db *gorm.DB, accountID string) ([]Coupon, error)
	CountByBatch(db *gorm.DB, batchNo string) (int64, error)
}
