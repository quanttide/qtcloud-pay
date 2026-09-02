package voucher_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quanttide/quanttide-pay-toolkit/packages/go/pkg/money"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func newTestServer(t *testing.T) (*httptest.Server, *voucher.Service) {
	t.Helper()
	svc, _ := newService(t)
	h := voucher.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, svc
}

func newAdminTestServer(t *testing.T) (*httptest.Server, *voucher.Service) {
	t.Helper()
	svc, _ := newService(t)
	h := voucher.NewHandlerWithAdmin(svc, "secret")
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, svc
}

func issueBody() map[string]any {
	return map[string]any{
		"amount": money.New(3000, money.CNY), "scope": "all",
		"expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"count":      2, "batch_no": "batch-v-http",
	}
}

func TestTransport_Issue(t *testing.T) {
	ts, _ := newTestServer(t)
	b, _ := json.Marshal(issueBody())

	resp, err := http.Post(ts.URL+"/accounts/acc_1/vouchers", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	json.NewDecoder(resp.Body).Decode(&got)
	if got["batch_no"] != "batch-v-http" || got["count"] != float64(2) {
		t.Errorf("body = %v", got)
	}
}

func TestTransport_Issue_Errors(t *testing.T) {
	ts, _ := newTestServer(t)

	cases := []struct {
		name string
		url  string
		body string
		want int
	}{
		{"bad json", ts.URL + "/accounts/acc_1/vouchers", `not-json`, 400},
		{"invalid", ts.URL + "/accounts/acc_1/vouchers", `{"amount":0}`, 400},
		{"empty id", ts.URL + "/accounts/%20/vouchers",
			`{"amount":100,"scope":"all","count":1,"batch_no":"b1"}`, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := http.Post(c.url, "application/json", bytes.NewReader([]byte(c.body)))
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

func TestTransport_Issue_ServiceError(t *testing.T) {
	svc, db := newService(t)
	sqlDB, _ := db.DB()
	sqlDB.Close()

	h := voucher.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	b, _ := json.Marshal(issueBody())
	resp, err := http.Post(ts.URL+"/accounts/acc_1/vouchers", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestTransport_List(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	svc.Issue(ctx, validIssue())

	resp, err := http.Get(ts.URL + "/accounts/acc_1/vouchers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		Vouchers []voucher.Voucher `json:"vouchers"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got.Vouchers) != 2 {
		t.Errorf("vouchers = %d, want 2", len(got.Vouchers))
	}

	resp2, err := http.Get(ts.URL + "/accounts/%20/vouchers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp2.StatusCode)
	}
}

func TestTransport_List_ServiceError(t *testing.T) {
	svc, db := newService(t)
	sqlDB, _ := db.DB()
	sqlDB.Close()

	h := voucher.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/accounts/acc_1/vouchers")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestTransport_PricingRuleSet(t *testing.T) {
	ts, _ := newAdminTestServer(t)
	body, _ := json.Marshal(map[string]any{
		"source":  "payment-engineering/qtclass/voucher-pricing.json",
		"version": "2026-09-01",
		"payload": json.RawMessage(validPricingPayload()),
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/admin/voucher-pricing-rules/qtclass-voucher-pricing", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200", resp.StatusCode)
	}
	var putGot struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		Payload struct {
			Issuance struct {
				Channels []struct {
					Voucher struct {
						AmountCents int64  `json:"amount_cents"`
						Scope       string `json:"scope"`
					} `json:"voucher"`
				} `json:"channels"`
			} `json:"issuance"`
		} `json:"payload"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&putGot); err != nil {
		t.Fatal(err)
	}
	if putGot.ID != "qtclass-voucher-pricing" || putGot.Payload.Issuance.Channels[0].Voucher.AmountCents != 10000 || putGot.Payload.Issuance.Channels[0].Voucher.Scope != "all" {
		t.Fatalf("PUT body = %+v", putGot)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/admin/voucher-pricing-rules/qtclass-voucher-pricing", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT duplicate status = %d, want 200", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/admin/voucher-pricing-rules/qtclass-voucher-pricing", nil)
	req.Header.Set("X-Admin-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/admin/voucher-pricing-rules", nil)
	req.Header.Set("X-Admin-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("LIST status = %d, want 200", resp.StatusCode)
	}
}

func TestTransport_PricingRuleSet_ForbiddenAndInvalid(t *testing.T) {
	ts, _ := newAdminTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/admin/voucher-pricing-rules", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized status = %d, want 403", resp.StatusCode)
	}

	body := []byte(`{"payload":{"issuance":{"channels":[{"name":"x","trigger":"x","voucher":{"amount_cents":100,"scope":"bad","expires_at_rule":"x"},"count_per_event":1}]},"redemption":{"scenarios":[{"scenario":"s","name":"n","pricing_model":"per_count_flat","quotas":[{"application_type":"a","name":"n","free_limit":0,"exceed_price_cents":100}]}]},"billing_semantics":{"voucher_is_money":true}}}`)
	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/admin/voucher-pricing-rules/qtclass", bytes.NewReader(body))
	req.Header.Set("X-Admin-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 400", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/admin/voucher-pricing-rules/missing", nil)
	req.Header.Set("X-Admin-Token", "secret")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", resp.StatusCode)
	}
}
