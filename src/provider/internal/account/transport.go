package account

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
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

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	acc, err := h.svc.Create(r.Context(), req.CustomerID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, acc)
}

func (h *Handler) handleRecharge(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Amount    int64  `json:"amount"`
		VoucherNo string `json:"voucher_no"`
		Note      string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Recharge(r.Context(), accountID, req.Amount, req.VoucherNo, req.Note); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account_id": accountID})
}

func (h *Handler) handleRefund(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	var req struct {
		Amount    int64  `json:"amount"`
		VoucherNo string `json:"voucher_no"`
		Note      string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := h.svc.Refund(r.Context(), accountID, req.Amount, req.VoucherNo, req.Note); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"account_id": accountID})
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	acc, err := h.svc.Get(r.Context(), accountID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, acc)
}

func (h *Handler) handleTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	limit, offset := parsePagination(r)
	txs, err := h.svc.ListTransactions(r.Context(), accountID, limit, offset)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if txs == nil {
		txs = []transaction.Transaction{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"account_id":   accountID,
		"transactions": txs,
	})
}

// parsePagination 解析 limit/offset 查询参数。
func parsePagination(r *http.Request) (limit, offset int) {
	limit, offset = 20, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 100 {
		limit = 100
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("account: write json response: %v", err)
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
	case errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, ErrExists):
		status = http.StatusConflict
	case errors.Is(err, ErrInsufficientBalance):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, ErrInvalidAmount), errors.Is(err, ErrInvalidRecharge), errors.Is(err, ErrInvalidRefund):
		status = http.StatusBadRequest
	case errors.Is(err, context.Canceled):
		status = http.StatusBadRequest
	}
	log.Printf("account: %v", err)
	writeError(w, status, http.StatusText(status))
}
