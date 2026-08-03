package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
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
		OrderID    string       `json:"order_id"`
		CustomerID string       `json:"customer_id"`
		AccountID  string       `json:"account_id"`
		ProductID  string       `json:"product_id"`
		Scope      string       `json:"scope"`
		Amount     *money.Money `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	order, err := h.svc.Settle(r.Context(), &SettleRequest{
		OrderID: req.OrderID, CustomerID: req.CustomerID, AccountID: req.AccountID,
		ProductID: req.ProductID, Scope: req.Scope, Amount: money.CentsOf(req.Amount),
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toOrderDTO(order))
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.PathValue("id"))
	if orderID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing order id")
		return
	}
	order, err := h.svc.Get(r.Context(), orderID)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toOrderDTO(order))
}

// orderDTO 订单响应（金额以元传输）。
type orderDTO struct {
	ID           string          `json:"id"`
	CustomerID   string          `json:"customer_id"`
	AccountID    string          `json:"account_id"`
	ProductID    string          `json:"product_id,omitempty"`
	Scope        string          `json:"scope,omitempty"`
	Amount       *money.Money    `json:"amount"`
	Status       string          `json:"status"`
	SettleDetail json.RawMessage `json:"settle_detail,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	SettledAt    *time.Time      `json:"settled_at,omitempty"`
}

func toOrderDTO(o *Order) orderDTO {
	return orderDTO{
		ID: o.ID, CustomerID: o.CustomerID, AccountID: o.AccountID,
		ProductID: o.ProductID, Scope: o.Scope, Amount: money.New(o.Amount, money.CNY),
		Status: o.Status, SettleDetail: formatSettleDetail(o.SettleDetail),
		CreatedAt: o.CreatedAt, SettledAt: o.SettledAt,
	}
}

// formatSettleDetail 将结算明细快照中的金额（存储为分）转为 Money 对象（整数分 + CNY）。
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
		Kind   string       `json:"kind"`
		RefID  int64        `json:"ref_id"`
		Amount *money.Money `json:"amount"`
	}
	out := make([]detailOut, 0, len(ds))
	for _, d := range ds {
		out = append(out, detailOut{Kind: d.Kind, RefID: d.RefID, Amount: money.New(d.Amount, money.CNY)})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return raw
	}
	return b
}

// errMapper 服务错误 → HTTP 状态码映射（未识别错误由 httpapi 记日志并返回 500）。
var errMapper = httpapi.Mapper(func(err error) int {
	switch {
	case errors.Is(err, account.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, billing.ErrInsufficientBalance):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, billing.ErrInvalidAmount):
		return http.StatusBadRequest
	case errors.Is(err, coupon.ErrUnavailable), errors.Is(err, voucher.ErrUnavailable):
		return http.StatusConflict
	}
	return 0
})
