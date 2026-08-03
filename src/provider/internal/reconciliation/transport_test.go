package reconciliation_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/reconciliation"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/transaction"
	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"
)

func newTestServer(t *testing.T) (*httptest.Server, *reconciliation.Service) {
	t.Helper()
	_, _, _, reconSvc := setupEnv(t)
	h := reconciliation.NewHandler(reconSvc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, reconSvc
}

func TestTransport_Consistency(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, err := http.Get(ts.URL + "/reconcile/consistency")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		Discrepancies []json.RawMessage `json:"discrepancies"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got.Discrepancies) != 0 {
		t.Errorf("discrepancies = %d, want 0", len(got.Discrepancies))
	}
}

func TestTransport_Consistency_ServiceError(t *testing.T) {
	db, accSvc, _, _ := setupEnv(t)
	ctx := context.Background()
	acc, _ := accSvc.Create(ctx, "cust_1")
	reconSvc := reconciliation.NewService(db, accSvc,
		transaction.NewService(&stubTxRepo{sumErr: errors.New("sum failed")}))
	h := reconciliation.NewHandler(reconSvc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/reconcile/consistency")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (account %s)", resp.StatusCode, acc.ID)
	}
}

func TestTransport_BankFile(t *testing.T) {
	ts, _ := newTestServer(t)

	// 合法 CSV
	resp, err := http.Post(ts.URL+"/reconcile/bank", "text/csv", bytes.NewReader([]byte("voucher-1,100,2026-08-01\n")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("valid status = %d, want 200", resp.StatusCode)
	}

	// 非法 CSV → 400
	resp2, err := http.Post(ts.URL+"/reconcile/bank", "text/csv", bytes.NewReader([]byte("bad,not-number,x\n")))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid status = %d, want 400", resp2.StatusCode)
	}
}

func TestTransport_Statement(t *testing.T) {
	_, accSvc, _, reconSvc := setupEnv(t)
	ctx := context.Background()

	acc, _ := accSvc.Create(ctx, "cust_1")
	accSvc.Recharge(ctx, acc.ID, 1000, "v1", "")

	h := reconciliation.NewHandler(reconSvc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/accounts/" + acc.ID + "/statement")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		AccountID string       `json:"account_id"`
		Closing   *money.Money `json:"closing_balance"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if got.AccountID != acc.ID || got.Closing == nil || got.Closing.Amount() != 1000 {
		t.Errorf("statement = %+v", got)
	}

	// 不存在 → 404
	resp2, err := http.Get(ts.URL + "/accounts/acc_missing/statement")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("missing status = %d, want 404", resp2.StatusCode)
	}

	// 空 id → 400
	resp3, err := http.Get(ts.URL + "/accounts/%20/statement")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp3.StatusCode)
	}
}
