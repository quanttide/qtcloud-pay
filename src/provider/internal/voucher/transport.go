package voucher

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
)

// Handler 代金券 API。
type Handler struct {
	svc        *Service
	adminToken string // 非空且请求头匹配时才允许规则管理 API
}

// NewHandler 创建代金券 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// NewHandlerWithAdmin 创建带管理端点鉴权的代金券 API。
func NewHandlerWithAdmin(svc *Service, adminToken string) *Handler {
	return &Handler{svc: svc, adminToken: adminToken}
}

// Register 注册代金券路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts/{id}/vouchers", h.handleIssue)
	mux.HandleFunc("GET /accounts/{id}/vouchers", h.handleList)
	mux.HandleFunc("GET /admin/voucher-pricing-rules", h.handleListPricingRuleSets)
	mux.HandleFunc("GET /admin/voucher-pricing-rules/{id}", h.handleGetPricingRuleSet)
	mux.HandleFunc("PUT /admin/voucher-pricing-rules/{id}", h.handleUpsertPricingRuleSet)
}

func (h *Handler) handleIssue(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Amount    *money.Money `json:"amount"`
		Scope     string       `json:"scope"`
		ProductID string       `json:"product_id"`
		ExpiresAt time.Time    `json:"expires_at"`
		Count     int          `json:"count"`
		BatchNo   string       `json:"batch_no"`
		Note      string       `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.svc.Issue(r.Context(), &IssueRequest{
		AccountID: accountID,
		Amount:    money.CentsOf(req.Amount),
		Scope:     req.Scope,
		ProductID: req.ProductID,
		ExpiresAt: req.ExpiresAt,
		Count:     req.Count,
		BatchNo:   req.BatchNo,
		Note:      req.Note,
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"account_id": accountID,
		"batch_no":   req.BatchNo,
		"count":      req.Count,
	})
}

// voucherDTO 代金券响应（金额以元传输）。
type voucherDTO struct {
	ID        int64        `json:"id"`
	AccountID string       `json:"account_id"`
	Amount    *money.Money `json:"amount"`
	Scope     string       `json:"scope"`
	ProductID string       `json:"product_id,omitempty"`
	ExpiresAt time.Time    `json:"expires_at"`
	Status    string       `json:"status"`
	UsedAt    *time.Time   `json:"used_at,omitempty"`
	OrderID   string       `json:"order_id,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

func toVoucherDTO(v Voucher) voucherDTO {
	return voucherDTO{
		ID: v.ID, AccountID: v.AccountID, Amount: money.New(v.Amount, money.CNY),
		Scope: v.Scope, ProductID: v.ProductID, ExpiresAt: v.ExpiresAt,
		Status: v.Status, UsedAt: v.UsedAt, OrderID: v.OrderID, CreatedAt: v.CreatedAt,
	}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	list, err := h.svc.List(r.Context(), accountID)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	dtos := make([]voucherDTO, 0, len(list))
	for _, v := range list {
		dtos = append(dtos, toVoucherDTO(v))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"account_id": accountID,
		"vouchers":   dtos,
	})
}

type pricingRuleSetDTO struct {
	ID        string          `json:"id"`
	Source    string          `json:"source,omitempty"`
	Version   string          `json:"version,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func toPricingRuleSetDTO(ruleSet *PricingRuleSet) pricingRuleSetDTO {
	return pricingRuleSetDTO{
		ID:        ruleSet.ID,
		Source:    ruleSet.Source,
		Version:   ruleSet.Version,
		Payload:   json.RawMessage(ruleSet.Payload),
		CreatedAt: ruleSet.CreatedAt,
		UpdatedAt: ruleSet.UpdatedAt,
	}
}

func (h *Handler) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	if h.adminToken == "" || r.Header.Get("X-Admin-Token") != h.adminToken {
		httpapi.WriteError(w, http.StatusForbidden, "forbidden")
		return false
	}
	return true
}

func (h *Handler) handleUpsertPricingRuleSet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing rule set id")
		return
	}
	var req struct {
		Source  string          `json:"source"`
		Version string          `json:"version"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	ruleSet, err := h.svc.UpsertPricingRuleSet(r.Context(), &PricingRuleSet{
		ID:      id,
		Source:  strings.TrimSpace(req.Source),
		Version: strings.TrimSpace(req.Version),
		Payload: string(req.Payload),
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toPricingRuleSetDTO(ruleSet))
}

func (h *Handler) handleGetPricingRuleSet(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing rule set id")
		return
	}
	ruleSet, err := h.svc.GetPricingRuleSet(r.Context(), id)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toPricingRuleSetDTO(ruleSet))
}

func (h *Handler) handleListPricingRuleSets(w http.ResponseWriter, r *http.Request) {
	if !h.requireAdmin(w, r) {
		return
	}
	list, err := h.svc.ListPricingRuleSets(r.Context())
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	dtos := make([]pricingRuleSetDTO, 0, len(list))
	for i := range list {
		dtos = append(dtos, toPricingRuleSetDTO(&list[i]))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"rule_sets": dtos})
}

// errMapper 服务错误 → HTTP 状态码映射（未识别错误由 httpapi 记日志并返回 500）。
var errMapper = httpapi.Mapper(func(err error) int {
	switch {
	case errors.Is(err, ErrInvalidIssue):
		return http.StatusBadRequest
	case errors.Is(err, ErrInvalidRuleSet):
		return http.StatusBadRequest
	case errors.Is(err, ErrRuleSetNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrRuleSetRepoUnavailable):
		return http.StatusInternalServerError
	}
	return 0
})
