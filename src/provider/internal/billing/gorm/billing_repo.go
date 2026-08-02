package gorm

import (
	"gorm.io/gorm"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
)

// BillingRuleRepo 计费规则 GORM 仓库。
type BillingRuleRepo struct{}

// NewBillingRuleRepo 创建计费规则 GORM 仓库。
func NewBillingRuleRepo() *BillingRuleRepo {
	return &BillingRuleRepo{}
}

func (r *BillingRuleRepo) List(db *gorm.DB) ([]billing.BillingRule, error) {
	var list []billing.BillingRule
	err := db.Order("priority ASC").Find(&list).Error
	return list, err
}

var _ billing.Repository = (*BillingRuleRepo)(nil)
