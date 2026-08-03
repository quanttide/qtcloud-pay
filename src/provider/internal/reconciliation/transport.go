package reconciliation

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/httpapi"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
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
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	dtos := make([]discrepancyDTO, 0, len(discrepancies))
	for _, d := range discrepancies {
		dtos = append(dtos, discrepancyDTO{
			AccountID: d.AccountID,
			Balance:   money.New(d.Balance, money.CNY),
			Expected:  money.New(d.Expected, money.CNY),
		})
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"discrepancies": dtos})
}

// discrepancyDTO 一致性差异响应（金额以元传输）。
type discrepancyDTO struct {
	AccountID string       `json:"account_id"`
	Balance   *money.Money `json:"balance"`
	Expected  *money.Money `json:"expected"`
}

func (h *Handler) handleBankFile(w http.ResponseWriter, r *http.Request) {
	report, err := h.svc.ReconcileBankFile(r.Context(), r.Body)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, report)
}

func (h *Handler) handleStatement(w http.ResponseWriter, r *http.Request) {
	accountID := strings.TrimSpace(r.PathValue("id"))
	if accountID == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "missing account id")
		return
	}
	stmt, err := h.svc.Statement(r.Context(), accountID)
	if err != nil {
		httpapi.WriteServiceError(w, err, errMapper)
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, toStatementDTO(stmt))
}

// statementDTO 账单响应（金额以元传输）。
type statementDTO struct {
	AccountID   string              `json:"account_id"`
	Opening     *money.Money        `json:"opening_balance"`
	Closing     *money.Money        `json:"closing_balance"`
	Entries     []statementEntryDTO `json:"entries"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// statementEntryDTO 账单条目（金额以元传输）。
type statementEntryDTO struct {
	ID             int64        `json:"id"`
	Type           string       `json:"type"`
	Amount         *money.Money `json:"amount"`
	Note           string       `json:"note,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	RunningBalance *money.Money `json:"running_balance"`
}

func toStatementDTO(s *Statement) statementDTO {
	entries := make([]statementEntryDTO, 0, len(s.Entries))
	for _, e := range s.Entries {
		entries = append(entries, statementEntryDTO{
			ID: e.ID, Type: e.Type, Amount: money.New(e.Amount, money.CNY), Note: e.Note,
			CreatedAt: e.CreatedAt, RunningBalance: money.New(e.RunningBalance, money.CNY),
		})
	}
	return statementDTO{
		AccountID: s.AccountID, Opening: money.New(s.Opening, money.CNY),
		Closing: money.New(s.Closing, money.CNY), Entries: entries, GeneratedAt: s.GeneratedAt,
	}
}

// errMapper 服务错误 → HTTP 状态码映射（未识别错误由 httpapi 记日志并返回 500）。
var errMapper = httpapi.Mapper(func(err error) int {
	switch {
	case errors.Is(err, account.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidCSV):
		return http.StatusBadRequest
	}
	return 0
})
