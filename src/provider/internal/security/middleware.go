package security

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
)

type routeRule struct {
	method     string
	prefix     string
	exact      string
	contains   string
	permission string
}

type Middleware struct {
	store      *Store
	verifier   *TokenVerifier
	adminToken string
}

func NewMiddleware(store *Store, verifier *TokenVerifier, adminToken string) *Middleware {
	return &Middleware{store: store, verifier: verifier, adminToken: strings.TrimSpace(adminToken)}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perm, protected := requiredPermission(r.Method, r.URL.Path)
		if !protected {
			next.ServeHTTP(w, r)
			return
		}
		if m.adminToken != "" && r.Header.Get("X-Admin-Token") == m.adminToken {
			next.ServeHTTP(w, r)
			return
		}
		token := bearerToken(r)
		if token == "" {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		claims, err := m.verifier.Verify(token)
		if err != nil {
			httpapi.WriteError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		allowed, err := m.ownerReadAllowed(r, claims.Subject)
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "permission lookup failed")
			return
		}
		if allowed {
			next.ServeHTTP(w, r)
			return
		}
		perms, err := m.store.PermissionsForUser(r.Context(), claims.Subject)
		if err != nil {
			httpapi.WriteError(w, http.StatusInternalServerError, "permission lookup failed")
			return
		}
		for _, role := range claims.Roles {
			for _, p := range InitialRolePermissions()[role] {
				perms[p] = struct{}{}
			}
		}
		if _, ok := perms[PermissionAll]; ok {
			next.ServeHTTP(w, r)
			return
		}
		if _, ok := perms[perm]; !ok {
			httpapi.WriteError(w, http.StatusForbidden, "forbidden")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (m *Middleware) ownerReadAllowed(r *http.Request, subject string) (bool, error) {
	if r.Method != http.MethodGet {
		return false, nil
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return false, nil
	}
	if customerID, ok := customerAccountPath(r.URL.Path); ok {
		return subject == customerID, nil
	}
	if accountID, ok := ownedAccountReadPath(r.URL.Path); ok {
		customerID, found, err := m.store.CustomerIDForAccount(r.Context(), accountID)
		if err != nil || !found {
			return false, err
		}
		return subject == customerID, nil
	}
	if transferID, ok := transferDetailPath(r.URL.Path); ok {
		return m.store.TransferOwnedBySubject(r.Context(), transferID, subject)
	}
	if accountID, ok := transferListAccount(r); ok {
		customerID, found, err := m.store.CustomerIDForAccount(r.Context(), accountID)
		if err != nil || !found {
			return false, err
		}
		return subject == customerID, nil
	}
	return false, nil
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return strings.TrimSpace(r.Header.Get("X-Service-Token"))
}

func customerAccountPath(path string) (string, bool) {
	parts := splitPath(path)
	if len(parts) == 3 && parts[0] == "customers" && parts[2] == "account" {
		return parts[1], true
	}
	return "", false
}

func ownedAccountReadPath(path string) (string, bool) {
	parts := splitPath(path)
	if len(parts) < 2 || parts[0] != "accounts" {
		return "", false
	}
	if len(parts) == 2 {
		return parts[1], true
	}
	if len(parts) != 3 {
		return "", false
	}
	switch parts[2] {
	case "transactions", "statement", "coupons", "vouchers":
		return parts[1], true
	default:
		return "", false
	}
}

func transferDetailPath(path string) (int64, bool) {
	parts := splitPath(path)
	if len(parts) != 2 || parts[0] != "transfers" {
		return 0, false
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	return id, err == nil && id > 0
}

func transferListAccount(r *http.Request) (string, bool) {
	if strings.Trim(r.URL.Path, "/") != "transfers" {
		return "", false
	}
	accountID := strings.TrimSpace(r.URL.Query().Get("account_id"))
	return accountID, accountID != ""
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func requiredPermission(method, path string) (string, bool) {
	for _, rule := range routeRules {
		if rule.method != method {
			continue
		}
		if rule.exact != "" && path == rule.exact {
			return rule.permission, true
		}
		if rule.prefix != "" && strings.HasPrefix(path, rule.prefix) {
			if rule.contains != "" && !strings.Contains(path, rule.contains) {
				continue
			}
			return rule.permission, true
		}
	}
	return "", false
}

var routeRules = []routeRule{
	{method: http.MethodPost, prefix: "/transfers/", contains: "/review", permission: PermTransferReview},
	{method: http.MethodPost, prefix: "/transfers/", contains: "/refund", permission: PermTransferRefund},
	{method: http.MethodPost, exact: "/transfers", permission: PermTransferWrite},
	{method: http.MethodGet, exact: "/transfers", permission: PermTransferRead},
	{method: http.MethodGet, prefix: "/transfers/", permission: PermTransferRead},
	{method: http.MethodPost, prefix: "/accounts/", contains: "/coupons", permission: PermCouponWrite},
	{method: http.MethodPost, prefix: "/accounts/", contains: "/vouchers", permission: PermVoucherWrite},
	{method: http.MethodPost, prefix: "/accounts/", contains: "/recharges", permission: PermAccountWrite},
	{method: http.MethodPost, prefix: "/accounts/", contains: "/refunds", permission: PermAccountWrite},
	{method: http.MethodPost, exact: "/accounts", permission: PermAccountWrite},
	{method: http.MethodGet, prefix: "/accounts/", permission: PermAccountRead},
	{method: http.MethodGet, prefix: "/customers/", permission: PermAccountRead},
	{method: http.MethodDelete, prefix: "/admin/accounts/", permission: PermAdminDelete},
	{method: http.MethodPost, prefix: "/orders", permission: PermOrderWrite},
	{method: http.MethodGet, prefix: "/orders/", permission: PermOrderRead},
	{method: http.MethodPost, prefix: "/reconcile/", permission: PermReconcileWrite},
	{method: http.MethodGet, prefix: "/reconcile/", permission: PermReconcileRead},
	{method: http.MethodGet, prefix: "/admin/voucher-pricing-rules", permission: PermRuleRead},
	{method: http.MethodPut, prefix: "/admin/voucher-pricing-rules", permission: PermRuleWrite},
	{method: http.MethodGet, prefix: "/admin/security/", permission: PermSecurityAdmin},
	{method: http.MethodPut, prefix: "/admin/security/", permission: PermSecurityAdmin},
	{method: http.MethodPost, prefix: "/admin/security/", permission: PermSecurityAdmin},
	{method: http.MethodPost, exact: "/pay", permission: PermChannelPay},
	{method: http.MethodGet, prefix: "/query/", permission: PermChannelQuery},
	{method: http.MethodPost, exact: "/refund", permission: PermChannelRefund},
}
