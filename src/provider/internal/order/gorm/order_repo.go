package gorm

import (
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
)

// OrderRepo 订单 GORM 仓库。
type OrderRepo struct{}

// NewOrderRepo 创建订单 GORM 仓库。
func NewOrderRepo() *OrderRepo {
	return &OrderRepo{}
}

func (r *OrderRepo) Create(db *gorm.DB, o *order.Order) error {
	return db.Create(o).Error
}

func (r *OrderRepo) Get(db *gorm.DB, id string) (*order.Order, error) {
	var o order.Order
	if err := db.Where("id = ?", id).First(&o).Error; err != nil {
		return nil, err
	}
	return &o, nil
}

var _ order.Repository = (*OrderRepo)(nil)
