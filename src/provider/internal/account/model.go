package account

import "time"

// Account 客户在我们这里的虚拟钱包。
// Balance 是交易的投影，与交易在同一事务内维护。
type Account struct {
	ID         string    `gorm:"primaryKey;size:64" json:"id"`
	CustomerID string    `gorm:"index;size:64" json:"customer_id"`
	Balance    int64     `gorm:"comment:余额（分）" json:"balance"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
