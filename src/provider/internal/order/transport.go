package order

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
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
		OrderID    string `json:"order_id"`
		CustomerID string `json:"customer_id"`
		AccountID  string `json:"account_id"`
		ProductID  string `json:"product_id"`
		Scope      string `json:"scope"`
		Amount     int64  `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	order, err := h.svc.Settle(r.Context(), &SettleRequest{
		OrderID: req.OrderID, CustomerID: req.CustomerID, AccountID: req.AccountID,
		ProductID: req.ProductID, Scope: req.Scope, Amount: req.Amount,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
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
	writeJSON(w, http.StatusOK, order)
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
