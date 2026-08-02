package order

import "gorm.io/gorm"

// Repository 订单存储接口。
type Repository interface {
	Create(db *gorm.DB, o *Order) error
	// Get 查询订单；不存在返回 gorm.ErrRecordNotFound。
	Get(db *gorm.DB, id string) (*Order, error)
}
