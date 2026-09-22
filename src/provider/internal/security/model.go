package security

import "time"

const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"

	PermissionAll = "*:*"

	PermAccountRead    = "account:read"
	PermAccountWrite   = "account:write"
	PermCouponWrite    = "coupon:write"
	PermVoucherWrite   = "voucher:write"
	PermTransferRead   = "transfer:read"
	PermTransferWrite  = "transfer:write"
	PermTransferReview = "transfer:review"
	PermTransferRefund = "transfer:refund"
	PermOrderRead      = "order:read"
	PermOrderWrite     = "order:write"
	PermReconcileRead  = "reconcile:read"
	PermReconcileWrite = "reconcile:write"
	PermRuleRead       = "rule:read"
	PermRuleWrite      = "rule:write"
	PermAdminDelete    = "admin:delete"
	PermSecurityAdmin  = "security:admin"
	PermChannelPay     = "channel:pay"
	PermChannelQuery   = "channel:query"
	PermChannelRefund  = "channel:refund"
)

// Role 权限角色。
type Role struct {
	Name        string `gorm:"primaryKey;size:64"`
	Description string `gorm:"size:255"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RolePermission 角色权限分配表。
type RolePermission struct {
	ID         uint   `gorm:"primaryKey"`
	RoleName   string `gorm:"index;size:64;not null"`
	Permission string `gorm:"index;size:128;not null"`
	CreatedAt  time.Time
}

// UserRole 用户角色分配表。UserID 对齐 qtcloud-auth 的用户标识。
type UserRole struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    string `gorm:"index;size:128;not null"`
	RoleName  string `gorm:"index;size:64;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// InitialRoles 返回支付系统默认角色。
func InitialRoles() []Role {
	return []Role{
		{Name: RoleAdmin, Description: "全权限"},
		{Name: RoleOperator, Description: "日常运营：充值、退款、发券、转赠审核、结算和查账，无删除与权限管理"},
		{Name: RoleViewer, Description: "只读查账"},
	}
}

// InitialRolePermissions 返回默认权限矩阵。
func InitialRolePermissions() map[string][]string {
	return map[string][]string{
		RoleAdmin:    {PermissionAll},
		RoleOperator: {PermAccountRead, PermAccountWrite, PermCouponWrite, PermVoucherWrite, PermTransferRead, PermTransferWrite, PermTransferReview, PermTransferRefund, PermOrderRead, PermOrderWrite, PermReconcileRead, PermReconcileWrite, PermRuleRead, PermRuleWrite, PermChannelPay, PermChannelQuery, PermChannelRefund},
		RoleViewer:   {PermAccountRead, PermTransferRead, PermOrderRead, PermReconcileRead, PermRuleRead, PermChannelQuery},
	}
}
