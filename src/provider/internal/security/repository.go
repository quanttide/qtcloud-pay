package security

import (
	"context"
	"errors"
	"strings"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInvalidUserID = errors.New("invalid user id")

// Store 管理权限数据。
type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&Role{}, &RolePermission{}, &UserRole{})
}

func SeedDefaults(ctx context.Context, db *gorm.DB) error {
	for _, role := range InitialRoles() {
		if err := db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&role).Error; err != nil {
			return err
		}
	}
	for role, perms := range InitialRolePermissions() {
		for _, perm := range perms {
			rp := RolePermission{RoleName: role, Permission: perm}
			if err := db.WithContext(ctx).Where("role_name = ? AND permission = ?", role, perm).FirstOrCreate(&rp).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) PermissionsForUser(ctx context.Context, userID string) (map[string]struct{}, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return map[string]struct{}{}, nil
	}
	var roles []UserRole
	if err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&roles).Error; err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return map[string]struct{}{}, nil
	}
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		names = append(names, role.RoleName)
	}
	var rows []RolePermission
	if err := s.db.WithContext(ctx).Where("role_name IN ?", names).Find(&rows).Error; err != nil {
		return nil, err
	}
	perms := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		perms[row.Permission] = struct{}{}
	}
	return perms, nil
}

func (s *Store) CustomerIDForAccount(ctx context.Context, accountID string) (string, bool, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", false, nil
	}
	var acc account.Account
	if err := s.db.WithContext(ctx).Select("customer_id").Where("id = ?", accountID).First(&acc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return acc.CustomerID, true, nil
}

func (s *Store) ListRoles(ctx context.Context) ([]Role, error) {
	var roles []Role
	err := s.db.WithContext(ctx).Order("name").Find(&roles).Error
	return roles, err
}

func (s *Store) ListRolePermissions(ctx context.Context) ([]RolePermission, error) {
	var perms []RolePermission
	err := s.db.WithContext(ctx).Order("role_name, permission").Find(&perms).Error
	return perms, err
}

func (s *Store) RolesForUser(ctx context.Context, userID string) ([]UserRole, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, ErrInvalidUserID
	}
	var roles []UserRole
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Order("role_name").Find(&roles).Error
	return roles, err
}

func (s *Store) SetUserRoles(ctx context.Context, userID string, roleNames []string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrInvalidUserID
	}
	clean := make([]string, 0, len(roleNames))
	seen := map[string]struct{}{}
	for _, role := range roleNames {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		clean = append(clean, role)
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, role := range clean {
			var count int64
			if err := tx.Model(&Role{}).Where("name = ?", role).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return gorm.ErrRecordNotFound
			}
		}
		if err := tx.Where("user_id = ?", userID).Delete(&UserRole{}).Error; err != nil {
			return err
		}
		for _, role := range clean {
			ur := UserRole{UserID: userID, RoleName: role}
			if err := tx.Create(&ur).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
