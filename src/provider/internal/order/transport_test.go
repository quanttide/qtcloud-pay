package order_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/order"
)

func newTestServer(t *testing.T) (*httptest.Server, *env) {
	t.Helper()
	e := setupEnv(t)
	h := order.NewHandler(e.orderSvc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, e
}

// fundedAccount 创建有余额的账户。
func fundedAccount(t *testing.T, e *env, ctx context.Context) string {
	t.Helper()
	acc, err := e.accountSvc.Create(ctx, "cust_1")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.accountSvc.Recharge(ctx, acc.ID, 10000, "voucher-http", ""); err != nil {
		t.Fatal(err)
	}
	return acc.ID
}

func settleBody(accountID string) map[string]any {
	return map[string]any{
		"order_id": "ORD-HTTP-1", "customer_id": "cust_1", "account_id": accountID,
		"product_id": "course-1", "scope": "course", "amount": 10000,
	}
}

func TestTransport_Settle(t *testing.T) {
	ts, e := newTestServer(t)
	ctx := context.Background()
	accID := fundedAccount(t, e, ctx)

	b, _ := json.Marshal(settleBody(accID))
	resp, err := http.Post(ts.URL+"/orders", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var got order.Order
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != "ORD-HTTP-1" || got.Status != order.StatusSettled {
		t.Errorf("order = %+v", got)
	}
}

func TestTransport_Settle_Errors(t *testing.T) {
	ts, _ := newTestServer(t)

	cases := []struct {
		name string
		body string
		want int
	}{
		{"bad json", `not-json`, 400},
		{"invalid request", `{"order_id":"","account_id":"acc_1","amount":0}`, 400},
		{"account not found", `{"order_id":"ORD-X","account_id":"acc_missing","scope":"course","amount":100}`, 404},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/orders", "application/json", bytes.NewReader([]byte(c.body)))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != c.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, c.want)
			}
		})
	}
}

func TestTransport_Settle_Insufficient(t *testing.T) {
	ts, e := newTestServer(t)
	ctx := context.Background()
	acc, _ := e.accountSvc.Create(ctx, "cust_1")
	e.accountSvc.Recharge(ctx, acc.ID, 100, "v-small", "") // 余额不足

	body := `{"order_id":"ORD-X","account_id":"` + acc.ID + `","scope":"course","amount":10000}`
	resp, err := http.Post(ts.URL+"/orders", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestTransport_Get(t *testing.T) {
	ts, e := newTestServer(t)
	ctx := context.Background()
	accID := fundedAccount(t, e, ctx)
	e.orderSvc.Settle(ctx, &order.SettleRequest{
		OrderID: "ORD-G", AccountID: accID, Scope: "course", Amount: 10000,
	})

	resp, err := http.Get(ts.URL + "/orders/ORD-G")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got order.Order
	json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != "ORD-G" {
		t.Errorf("order = %+v", got)
	}

	resp2, err := http.Get(ts.URL + "/orders/ORD-MISSING")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("missing status = %d, want 404", resp2.StatusCode)
	}

	resp3, err := http.Get(ts.URL + "/orders/%20")
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp3.StatusCode)
	}
}

func TestTransport_Settle_ServiceError(t *testing.T) {
	e := setupEnv(t)
	sqlDB, _ := e.db.DB()
	sqlDB.Close()

	h := order.NewHandler(e.orderSvc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	b, _ := json.Marshal(settleBody("acc_1"))
	resp, err := http.Post(ts.URL+"/orders", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}
