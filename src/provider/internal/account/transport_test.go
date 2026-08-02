package account_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
)

func newTestServer(t *testing.T) (*httptest.Server, *account.Service) {
	t.Helper()
	svc, _ := newService(t)
	h := account.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, svc
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestTransport_Create(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()

	resp := postJSON(t, ts.URL+"/accounts", map[string]any{"customer_id": "cust_1"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var got account.Account
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.CustomerID != "cust_1" || got.ID == "" {
		t.Errorf("account = %+v", got)
	}

	// 校验账户确实落库
	if _, err := svc.Get(ctx, got.ID); err != nil {
		t.Errorf("account not persisted: %v", err)
	}
}

func TestTransport_Create_BadJSON(t *testing.T) {
	ts, _ := newTestServer(t)
	resp, err := http.Post(ts.URL+"/accounts", "application/json", bytes.NewReader([]byte(`not-json`)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestTransport_Create_ServiceError(t *testing.T) {
	ts, _ := newTestServer(t)

	// 空 customer_id 触发服务错误 → 500
	resp := postJSON(t, ts.URL+"/accounts", map[string]any{"customer_id": ""})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestTransport_Recharge(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")

	resp := postJSON(t, ts.URL+"/accounts/"+acc.ID+"/recharges",
		map[string]any{"amount": 5000, "voucher_no": "v1", "note": "打款"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got, _ := svc.Get(ctx, acc.ID)
	if got.Balance != 5000 {
		t.Errorf("balance = %d, want 5000", got.Balance)
	}
}

func TestTransport_Recharge_Errors(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")

	cases := []struct {
		name string
		url  string
		body any
		want int
	}{
		{"bad json", ts.URL + "/accounts/" + acc.ID + "/recharges", "not-json", 400},
		{"invalid amount", ts.URL + "/accounts/" + acc.ID + "/recharges",
			map[string]any{"amount": 0, "voucher_no": "v1"}, 400},
		{"account not found", ts.URL + "/accounts/acc_missing/recharges",
			map[string]any{"amount": 100, "voucher_no": "v1"}, 404},
		{"empty id", ts.URL + "/accounts/%20/recharges",
			map[string]any{"amount": 100, "voucher_no": "v1"}, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := postJSON(t, c.url, c.body)
			defer resp.Body.Close()
			if resp.StatusCode != c.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestTransport_Refund(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")
	svc.Recharge(ctx, acc.ID, 10000, "v1", "")

	resp := postJSON(t, ts.URL+"/accounts/"+acc.ID+"/refunds",
		map[string]any{"amount": 4000, "voucher_no": "r1", "note": "多退"})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	got, _ := svc.Get(ctx, acc.ID)
	if got.Balance != 6000 {
		t.Errorf("balance = %d, want 6000", got.Balance)
	}
}

func TestTransport_Refund_Errors(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")
	svc.Recharge(ctx, acc.ID, 5000, "v1", "")

	cases := []struct {
		name string
		url  string
		body any
		want int
	}{
		{"bad json", ts.URL + "/accounts/" + acc.ID + "/refunds", "not-json", 400},
		{"invalid amount", ts.URL + "/accounts/" + acc.ID + "/refunds",
			map[string]any{"amount": 0, "voucher_no": "r1"}, 400},
		{"insufficient balance", ts.URL + "/accounts/" + acc.ID + "/refunds",
			map[string]any{"amount": 6000, "voucher_no": "r1"}, 422},
		{"account not found", ts.URL + "/accounts/acc_missing/refunds",
			map[string]any{"amount": 100, "voucher_no": "r1"}, 404},
		{"empty id", ts.URL + "/accounts/%20/refunds",
			map[string]any{"amount": 100, "voucher_no": "r1"}, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := postJSON(t, c.url, c.body)
			defer resp.Body.Close()
			if resp.StatusCode != c.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestTransport_Get(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")

	resp, err := http.Get(ts.URL + "/accounts/" + acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got account.Account
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != acc.ID || got.CustomerID != "cust_1" {
		t.Errorf("account = %+v", got)
	}
}

func TestTransport_Transactions_ServiceError(t *testing.T) {
	svc, db := newService(t)
	closeDB(t, db)
	h := account.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/accounts/acc_1/transactions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestTransport_Get_Errors(t *testing.T) {
	ts, _ := newTestServer(t)

	resp, err := http.Get(ts.URL + "/accounts/acc_missing")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()

	resp, err = http.Get(ts.URL + "/accounts/%20")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTransport_Transactions(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	acc, _ := svc.Create(ctx, "cust_1")
	svc.Recharge(ctx, acc.ID, 100, "v1", "")

	resp, err := http.Get(fmt.Sprintf("%s/accounts/%s/transactions?limit=1", ts.URL, acc.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Transactions []json.RawMessage `json:"transactions"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Transactions) != 1 {
		t.Errorf("transactions = %d, want 1", len(body.Transactions))
	}

	// 空账户返回空列表
	resp2, err := http.Get(ts.URL + "/accounts/acc_missing/transactions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("empty status = %d, want 200", resp2.StatusCode)
	}
	var body2 struct {
		Transactions []json.RawMessage `json:"transactions"`
	}
	json.NewDecoder(resp2.Body).Decode(&body2)
	if len(body2.Transactions) != 0 {
		t.Errorf("transactions = %d, want 0", len(body2.Transactions))
	}

	resp3, err := http.Get(ts.URL + "/accounts/%20/transactions")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp3.StatusCode)
	}
}
