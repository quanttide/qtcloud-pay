package gorm

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

// VoucherRepo 代金券 GORM 仓库。
type VoucherRepo struct{}

// NewVoucherRepo 创建代金券 GORM 仓库。
func NewVoucherRepo() *VoucherRepo {
	return &VoucherRepo{}
}

func (r *VoucherRepo) CreateBatch(db *gorm.DB, vouchers []*voucher.Voucher) error {
	return db.Create(vouchers).Error
}

func (r *VoucherRepo) Get(db *gorm.DB, id int64) (*voucher.Voucher, error) {
	var v voucher.Voucher
	if err := db.Where("id = ?", id).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VoucherRepo) GetForUpdate(db *gorm.DB, id int64) (*voucher.Voucher, error) {
	var v voucher.Voucher
	if err := db.Where("id = ?", id).First(&v).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (r *VoucherRepo) Update(db *gorm.DB, v *voucher.Voucher) error {
	return db.Model(v).Updates(map[string]any{
		"status":   v.Status,
		"used_at":  v.UsedAt,
		"order_id": v.OrderID,
	}).Error
}

func (r *VoucherRepo) ListByAccount(db *gorm.DB, accountID string) ([]voucher.Voucher, error) {
	var list []voucher.Voucher
	err := db.Where("account_id = ?", accountID).
		Order("id DESC").Find(&list).Error
	return list, err
}

func (r *VoucherRepo) CountByBatch(db *gorm.DB, batchNo string) (int64, error) {
	var n int64
	err := db.Model(&voucher.Voucher{}).
		Where("batch_no = ?", batchNo).Count(&n).Error
	return n, err
}

func (r *VoucherRepo) UpsertRuleSet(db *gorm.DB, ruleSet *voucher.PricingRuleSet) error {
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"source", "version", "payload", "updated_at",
		}),
	}).Create(ruleSet).Error
}

func (r *VoucherRepo) GetRuleSet(db *gorm.DB, id string) (*voucher.PricingRuleSet, error) {
	var ruleSet voucher.PricingRuleSet
	if err := db.Where("id = ?", id).First(&ruleSet).Error; err != nil {
		return nil, err
	}
	return &ruleSet, nil
}

func (r *VoucherRepo) ListRuleSets(db *gorm.DB) ([]voucher.PricingRuleSet, error) {
	var list []voucher.PricingRuleSet
	err := db.Order("id ASC").Find(&list).Error
	return list, err
}

var _ voucher.Repository = (*VoucherRepo)(nil)
var _ voucher.PricingRuleSetRepository = (*VoucherRepo)(nil)
