package gorm

import (
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
)

// CouponRepo 优惠券 GORM 仓库。
type CouponRepo struct{}

// NewCouponRepo 创建优惠券 GORM 仓库。
func NewCouponRepo() *CouponRepo {
	return &CouponRepo{}
}

func (r *CouponRepo) CreateBatch(db *gorm.DB, coupons []*coupon.Coupon) error {
	return db.Create(coupons).Error
}

func (r *CouponRepo) Get(db *gorm.DB, id int64) (*coupon.Coupon, error) {
	var c coupon.Coupon
	if err := db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CouponRepo) GetForUpdate(db *gorm.DB, id int64) (*coupon.Coupon, error) {
	var c coupon.Coupon
	if err := db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *CouponRepo) Update(db *gorm.DB, c *coupon.Coupon) error {
	return db.Model(c).Updates(map[string]any{
		"status":   c.Status,
		"used_at":  c.UsedAt,
		"order_id": c.OrderID,
	}).Error
}

func (r *CouponRepo) ListByAccount(db *gorm.DB, accountID string) ([]coupon.Coupon, error) {
	var list []coupon.Coupon
	err := db.Where("account_id = ?", accountID).
		Order("id DESC").Find(&list).Error
	return list, err
}

func (r *CouponRepo) CountByBatch(db *gorm.DB, batchNo string) (int64, error) {
	var n int64
	err := db.Model(&coupon.Coupon{}).
		Where("batch_no = ?", batchNo).Count(&n).Error
	return n, err
}

var _ coupon.Repository = (*CouponRepo)(nil)
