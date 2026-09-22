package transfer

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
)

// Handler 代金券转赠 API。
type Handler struct {
	svc *Service
}

// NewHandler 创建转赠 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册转赠路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /transfers", h.handleCreate)
	mux.HandleFunc("GET /transfers", h.handleList)
	mux.HandleFunc("GET /transfers/{id}", h.handleGet)
	mux.HandleFunc("POST /transfers/{id}/review", h.handleReview)
	mux.HandleFunc("POST /transfers/{id}/refund", h.handleRefund)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromAccountID  string       `json:"from_account_id"`
		ToAccountID    string       `json:"to_account_id"`
		Amount         *money.Money `json:"amount"`
		Note           string       `json:"note"`
		IdempotencyKey string       `json:"idempotency_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Create(r.Context(), &CreateRequest{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		Amount:         money.CentsOf(req.Amount),
		Note:           req.Note,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, toDTO(t))
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	limit, offset := httpapi.ParsePagination(r)
	list, err := h.svc.List(r.Context(), ListFilter{
		Status:    strings.TrimSpace(r.URL.Query().Get("status")),
		AccountID: strings.TrimSpace(r.URL.Query().Get("account_id")),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	dtos := make([]transferDTO, 0, len(list))
	for i := range list {
		dtos = append(dtos, toDTO(&list[i]))
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"transfers": dtos})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid transfer id")
		return
	}
	t, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toDTO(t))
}

func (h *Handler) handleReview(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid transfer id")
		return
	}
	var req struct {
		Decision   string `json:"decision"`
		ReviewedBy string `json:"reviewed_by"`
		Note       string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Review(r.Context(), id, &ReviewRequest{
		Decision: req.Decision, ReviewedBy: req.ReviewedBy, Note: req.Note,
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toDTO(t))
}

func (h *Handler) handleRefund(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid transfer id")
		return
	}
	var req struct {
		RefundedBy string `json:"refunded_by"`
		Note       string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	t, err := h.svc.Refund(r.Context(), id, &RefundRequest{
		RefundedBy: req.RefundedBy, Note: req.Note,
	})
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toDTO(t))
}

type transferDTO struct {
	ID                  int64        `json:"id"`
	FromAccountID       string       `json:"from_account_id"`
	ToAccountID         string       `json:"to_account_id"`
	SourceTransactionID int64        `json:"source_transaction_id"`
	Amount              *money.Money `json:"amount"`
	TaxRateBP           int          `json:"tax_rate_bp"`
	IssuedAmount        *money.Money `json:"issued_amount"`
	Status              string       `json:"status"`
	ReviewedBy          string       `json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time   `json:"reviewed_at,omitempty"`
	IssuedVoucherID     *int64       `json:"issued_voucher_id,omitempty"`
	RefundTransactionID *int64       `json:"refund_transaction_id,omitempty"`
	RefundedBy          string       `json:"refunded_by,omitempty"`
	RefundedAt          *time.Time   `json:"refunded_at,omitempty"`
	Note                string       `json:"note,omitempty"`
	IdempotencyKey      string       `json:"idempotency_key"`
	CreatedAt           time.Time    `json:"created_at"`
	UpdatedAt           time.Time    `json:"updated_at"`
}

func toDTO(t *VoucherTransfer) transferDTO {
	return transferDTO{
		ID:                  t.ID,
		FromAccountID:       t.FromAccountID,
		ToAccountID:         t.ToAccountID,
		SourceTransactionID: t.SourceTransactionID,
		Amount:              money.New(t.Amount, money.CNY),
		TaxRateBP:           t.TaxRateBP,
		IssuedAmount:        money.New(t.IssuedAmountCents(), money.CNY),
		Status:              t.Status,
		ReviewedBy:          t.ReviewedBy,
		ReviewedAt:          t.ReviewedAt,
		IssuedVoucherID:     t.IssuedVoucherID,
		RefundTransactionID: t.RefundTransactionID,
		RefundedBy:          t.RefundedBy,
		RefundedAt:          t.RefundedAt,
		Note:                t.Note,
		IdempotencyKey:      t.IdempotencyKey,
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}

func parseID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	return id, err == nil && id > 0
}

var errMapper = httpapi.Mapper(func(err error) int {
	switch {
	case errors.Is(err, ErrInvalidRequest), errors.Is(err, ErrInvalidStatus):
		return http.StatusBadRequest
	case errors.Is(err, ErrNotFound), errors.Is(err, account.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, account.ErrInsufficientBalance):
		return http.StatusUnprocessableEntity
	}
	return 0
})
