package gorm

import (
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
)

// AccountRepo 账户 GORM 仓库。
type AccountRepo struct{}

// NewAccountRepo 创建账户 GORM 仓库。
func NewAccountRepo() *AccountRepo {
	return &AccountRepo{}
}

func (r *AccountRepo) Create(db *gorm.DB, a *account.Account) error {
	return db.Create(a).Error
}

func (r *AccountRepo) Get(db *gorm.DB, id string) (*account.Account, error) {
	var a account.Account
	if err := db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepo) GetByCustomer(db *gorm.DB, customerID string) (*account.Account, error) {
	var a account.Account
	if err := db.Where("customer_id = ?", customerID).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepo) GetForUpdate(db *gorm.DB, id string) (*account.Account, error) {
	var a account.Account
	err := db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AccountRepo) Update(db *gorm.DB, a *account.Account) error {
	return db.Model(a).Updates(map[string]any{
		"balance":    a.Balance,
		"updated_at": a.UpdatedAt,
	}).Error
}

func (r *AccountRepo) List(db *gorm.DB) ([]account.Account, error) {
	var list []account.Account
	if err := db.Order("id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

var _ account.Repository = (*AccountRepo)(nil)
