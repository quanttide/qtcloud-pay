package transaction

import "gorm.io/gorm"

// Repository 交易账本存储接口。
// 方法统一以 *gorm.DB 为首参（可传共享连接或 *gorm.Tx），事务由调用方编排。
type Repository interface {
	// Create 插入一条交易；幂等键唯一冲突时返回 gorm.ErrDuplicatedKey。
	Create(db *gorm.DB, t *Transaction) error
	// GetByKey 按幂等键查询；不存在返回 gorm.ErrRecordNotFound。
	GetByKey(db *gorm.DB, key string) (*Transaction, error)
	// ListByAccount 按账户分页查询流水（id 倒序）。
	ListByAccount(db *gorm.DB, accountID string, limit, offset int) ([]Transaction, error)
	// ListAllByAccount 按账户查询全部流水（id 升序）。
	ListAllByAccount(db *gorm.DB, accountID string) ([]Transaction, error)
	// SumByAccount 汇总账户余额变动：Σ(充值) − Σ(余额支付)。
	SumByAccount(db *gorm.DB, accountID string) (int64, error)
}
