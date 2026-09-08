package security

import (
	"net/http"
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

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return strings.TrimSpace(r.Header.Get("X-Service-Token"))
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
