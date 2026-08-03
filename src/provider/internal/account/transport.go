package account

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
)

// Handler 账户 API。
type Handler struct {
	svc *Service
}

// NewHandler 创建账户 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册账户路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /accounts", h.handleCreate)
	mux.HandleFunc("POST /accounts/{id}/recharges", h.handleRecharge)
	mux.HandleFunc("POST /accounts/{id}/refunds", h.handleRefund)
	mux.HandleFunc("GET /accounts/{id}", h.handleGet)
	mux.HandleFunc("GET /accounts/{id}/transactions", h.handleTransactions)
}

// accountDTO 账户响应（金额以元传输）。
type accountDTO struct {
	ID         string       `json:"id"`
	CustomerID string       `json:"customer_id"`
	Balance    *money.Money `json:"balance"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

func toAccountDTO(a *Account) accountDTO {
	return accountDTO{
		ID: a.ID, CustomerID: a.CustomerID, Balance: money.New(a.Balance, money.CNY),
		CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

// txDTO 交易流水响应（金额以元传输）。
type txDTO struct {
	ID           int64        `json:"id"`
	AccountID    string       `json:"account_id"`
	Type         string       `json:"type"`
	Amount       *money.Money `json:"amount"`
	BalanceAfter *money.Money `json:"balance_after"`
	OrderID      string       `json:"order_id,omitempty"`
	Note         string       `json:"note,omitempty"`
	CreatedAt    time.Time    `json:"created_at"`
}

func toTxDTO(t transaction.Transaction) txDTO {
	return txDTO{
		ID: t.ID, AccountID: t.AccountID, Type: string(t.Type),
		Amount: money.New(t.Amount, money.CNY), BalanceAfter: money.New(t.BalanceAfter, money.CNY),
		OrderID: t.OrderID, Note: t.Note, CreatedAt: t.CreatedAt,
	}
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	acc, err := h.svc.Create(r.Context(), req.CustomerID)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toAccountDTO(acc))
}

func (h *Handler) handleRecharge(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Amount    *money.Money `json:"amount"`
		VoucherNo string       `json:"voucher_no"`
		Note      string       `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Recharge(r.Context(), accountID, money.CentsOf(req.Amount), req.VoucherNo, req.Note); err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"account_id": accountID})
}

func (h *Handler) handleRefund(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Amount    *money.Money `json:"amount"`
		VoucherNo string       `json:"voucher_no"`
		Note      string       `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Refund(r.Context(), accountID, money.CentsOf(req.Amount), req.VoucherNo, req.Note); err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"account_id": accountID})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	acc, err := h.svc.Get(r.Context(), accountID)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toAccountDTO(acc))
}

func (h *Handler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	limit, offset := httpapi.ParsePagination(r)
	txs, err := h.svc.ListTransactions(r.Context(), accountID, limit, offset)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	dtos := make([]txDTO, 0, len(txs))
	for _, t := range txs {
		dtos = append(dtos, toTxDTO(t))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"account_id":   accountID,
		"transactions": dtos,
	})
}

// errMapper 服务错误 → HTTP 状态码映射（未识别错误由 httpapi 记日志并返回 500）。
var errMapper = httpapi.Mapper(func(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrExists):
		return http.StatusConflict
	case errors.Is(err, ErrInsufficientBalance):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrInvalidRecharge), errors.Is(err, ErrInvalidRefund):
		return http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		return http.StatusBadRequest
	}
	return 0
})
