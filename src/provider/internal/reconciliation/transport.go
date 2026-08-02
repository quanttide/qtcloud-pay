package reconciliation

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
)

// Handler 对账 API。
type Handler struct {
	svc *Service
}

// NewHandler 创建对账 API。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 注册对账路由。
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /reconcile/consistency", h.handleConsistency)
	mux.HandleFunc("POST /reconcile/bank", h.handleBankFile)
	mux.HandleFunc("GET /accounts/{id}/statement", h.handleStatement)
}

func (h *Handler) handleConsistency(w http.ResponseWriter, r *http.Request) {
	discrepancies, err := h.svc.CheckConsistency(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if discrepancies == nil {
		discrepancies = []Discrepancy{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"discrepancies": discrepancies})
}

func (h *Handler) handleBankFile(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.ReconcileBankFile(r.Context(), r.Body)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handler) handleStatement(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		writeError(w, http.StatusBadRequest, "missing account id")
		return
	}
	stmt, err := h.svc.Statement(r.Context(), accountID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stmt)
}

// writeJSON 以 JSON 格式写入响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("reconciliation: write json response: %v", err)
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
	case errors.Is(err, ErrInvalidCSV):
		status = http.StatusBadRequest
	}
	log.Printf("reconciliation: %v", err)
	writeError(w, status, http.StatusText(status))
}
