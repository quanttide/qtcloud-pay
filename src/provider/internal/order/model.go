package order

import (
	"encoding/json"
	"time"
)

// 订单状态。
const (
	StatusCreated = "created" // 已创建
	StatusSettled = "settled" // 已结算
)

// Order 客户购买付费服务的交易请求。结算时应用计费规则生成交易，更新余额与券状态。
type Order struct {
	ID           string          `gorm:"primaryKey;size:64" json:"id"`
	CustomerID   string          `gorm:"index;size:64" json:"customer_id"`
	AccountID    string          `gorm:"index;size:64" json:"account_id"`
	ProductID    string          `gorm:"size:64" json:"product_id,omitempty"`
	Scope        string          `gorm:"size:32" json:"scope,omitempty"` // 业务类型（云服务/课程/数据服务）
	Amount       int64           `json:"amount"`                         // 订单金额（分）
	Status       string          `gorm:"size:16" json:"status"`
	SettleDetail json.RawMessage `gorm:"type:text" json:"settle_detail,omitempty"` // 结算计划快照（逐项抵扣）
	CreatedAt    time.Time       `json:"created_at"`
	SettledAt    *time.Time      `json:"settled_at,omitempty"`
}
