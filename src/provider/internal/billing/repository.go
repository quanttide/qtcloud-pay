package billing

import "gorm.io/gorm"

// Repository 计费规则存储接口。
type Repository interface {
	List(db *gorm.DB) ([]BillingRule, error)
}
