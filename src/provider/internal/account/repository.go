package account

import "gorm.io/gorm"

// Repository 账户存储接口。
// 方法统一以 *gorm.DB 为首参（可传共享连接或 *gorm.Tx），事务由调用方编排。
type Repository interface {
	Create(db *gorm.DB, a *Account) error
	// Get 查询账户；不存在返回 gorm.ErrRecordNotFound。
	Get(db *gorm.DB, id string) (*Account, error)
	// GetForUpdate 锁定账户行并查询（同账户并发写串行化）。
	GetForUpdate(db *gorm.DB, id string) (*Account, error)
	Update(db *gorm.DB, a *Account) error
	// List 查询全部账户（供对账）。
	List(db *gorm.DB) ([]Account, error)
}
