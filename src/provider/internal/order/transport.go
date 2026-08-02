package order

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	"github.com/quanttide/qtcloud-pay/src/provider/pkg/money"
)

// Handler 订单 API。
type Handler struct {
	svc *Service
}

// NewHandler 创建订单 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册订单路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /orders", h.handleSettle)
	mux.HandleFunc("GET /orders/{id}", h.handleGet)
}

func (h *Handler) handleSettle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID    string      `json:"order_id"`
		CustomerID string      `json:"customer_id"`
		AccountID  string      `json:"account_id"`
		ProductID  string      `json:"product_id"`
		Scope      string      `json:"scope"`
		Amount     money.Cents `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	order, err := h.svc.Settle(r.Context(), &SettleRequest{
		OrderID: req.OrderID, CustomerID: req.CustomerID, AccountID: req.AccountID,
		ProductID: req.ProductID, Scope: req.Scope, Amount: int64(req.Amount),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toOrderDTO(order))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("id"))
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "missing order id")
		return
	}
	order, err := h.svc.Get(r.Context(), orderID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toOrderDTO(order))
}

// orderDTO 订单响应（金额以元传输）。
type orderDTO struct {
	ID           string          `json:"id"`
	CustomerID   string          `json:"customer_id"`
	AccountID    string          `json:"account_id"`
	ProductID    string          `json:"product_id,omitempty"`
	Scope        string          `json:"scope,omitempty"`
	Amount       money.Cents     `json:"amount"`
	Status       string          `json:"status"`
	SettleDetail json.RawMessage `json:"settle_detail,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	SettledAt    *time.Time      `json:"settled_at,omitempty"`
}

func toOrderDTO(o *Order) orderDTO {
	return orderDTO{
		ID: o.ID, CustomerID: o.CustomerID, AccountID: o.AccountID,
		ProductID: o.ProductID, Scope: o.Scope, Amount: money.Cents(o.Amount),
		Status: o.Status, SettleDetail: formatSettleDetail(o.SettleDetail),
		CreatedAt: o.CreatedAt, SettledAt: o.SettledAt,
	}
}

// formatSettleDetail 将结算明细快照中的金额（存储为分）转为元。
func formatSettleDetail(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(raw) == "null" {
		return raw
	}
	var ds []struct {
		Kind   string `json:"kind"`
		RefID  int64  `json:"ref_id"`
		Amount int64  `json:"amount"` // 存储为分
	}
	if err := json.Unmarshal(raw, &ds); err != nil {
		return raw
	}
	type detailOut struct {
		Kind   string      `json:"kind"`
		RefID  int64       `json:"ref_id"`
		Amount money.Cents `json:"amount"`
	}
	out := make([]detailOut, 0, len(ds))
	for _, d := range ds {
		out = append(out, detailOut{Kind: d.Kind, RefID: d.RefID, Amount: money.Cents(d.Amount)})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}

// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("order: write json response: %v", err)
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
	case errors.Is(err, account.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, billing.ErrInsufficientBalance):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, billing.ErrInvalidAmount):
		status = http.StatusBadRequest
	case errors.Is(err, coupon.ErrUnavailable), errors.Is(err, voucher.ErrUnavailable):
		status = http.StatusConflict
	}
	log.Printf("order: %v", err)
	writeError(w, status, http.StatusText(status))
}
