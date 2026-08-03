package coupon

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
)

// Handler 优惠券 API。
type Handler struct {
	svc *Service
}

// NewHandler 创建优惠券 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册优惠券路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts/{id}/coupons", h.handleIssue)
	mux.HandleFunc("GET /accounts/{id}/coupons", h.handleList)
}

func (h *Handler) handleIssue(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Type      string      `json:"type"`
		Rate      int         `json:"rate"`
		Threshold *money.Money `json:"threshold"`
		Amount    *money.Money `json:"amount"`
		Scope     string      `json:"scope"`
		ProductID string      `json:"product_id"`
		ExpiresAt time.Time   `json:"expires_at"`
		Count     int         `json:"count"`
		BatchNo   string      `json:"batch_no"`
		Note      string      `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err := h.svc.Issue(r.Context(), &IssueRequest{
		AccountID: accountID,
		Type:      req.Type,
		Rate:      req.Rate,
		Threshold: money.CentsOf(req.Threshold),
		Amount:    money.CentsOf(req.Amount),
		Scope:     req.Scope,
		ProductID: req.ProductID,
		ExpiresAt: req.ExpiresAt,
		Count:     req.Count,
		BatchNo:   req.BatchNo,
		Note:      req.Note,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": accountID,
		"batch_no":   req.BatchNo,
		"count":      req.Count,
	})
}

// couponDTO 优惠券响应（金额以元传输）。
type couponDTO struct {
	ID        int64       `json:"id"`
	AccountID string      `json:"account_id"`
	Type      string      `json:"type"`
	Rate      int         `json:"rate,omitempty"`
	Threshold *money.Money `json:"threshold,omitempty"`
	Amount    *money.Money `json:"amount,omitempty"`
	Scope     string      `json:"scope"`
	ProductID string      `json:"product_id,omitempty"`
	ExpiresAt time.Time   `json:"expires_at"`
	Status    string      `json:"status"`
	UsedAt    *time.Time  `json:"used_at,omitempty"`
	OrderID   string      `json:"order_id,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}

func toCouponDTO(c Coupon) couponDTO {
	return couponDTO{
		ID: c.ID, AccountID: c.AccountID, Type: c.Type, Rate: c.Rate,
		Threshold: money.New(c.Threshold, money.CNY), Amount: money.New(c.Amount, money.CNY),
		Scope: c.Scope, ProductID: c.ProductID, ExpiresAt: c.ExpiresAt,
		Status: c.Status, UsedAt: c.UsedAt, OrderID: c.OrderID, CreatedAt: c.CreatedAt,
	}
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	list, err := h.svc.List(r.Context(), accountID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	dtos := make([]couponDTO, 0, len(list))
	for _, c := range list {
		dtos = append(dtos, toCouponDTO(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id": accountID,
		"coupons":    dtos,
	})
}

// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("coupon: write json response: %v", err)
	}
}

// writeError 写入错误响应。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeServiceError 将服务错误映射为 HTTP 状态码。
func writeServiceError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrInvalidIssue):
		status = http.StatusBadRequest
	}
	log.Printf("coupon: %v", err)
	writeError(w, status, http.StatusText(status))
}
