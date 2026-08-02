package coupon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
)

func newTestServer(t *testing.T) (*httptest.Server, *coupon.Service) {
	t.Helper()
	svc, _ := newService(t)
	h := coupon.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts, svc
}

func issueBody() map[string]any {
	return map[string]any{
		"type": "full_reduction", "threshold": 10000, "amount": 2000,
		"scope": "all", "expires_at": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		"count": 2, "batch_no": "batch-http-1",
	}
}

func postCoupons(t *testing.T, url, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestTransport_Issue(t *testing.T) {
	ts, _ := newTestServer(t)
	b, _ := json.Marshal(issueBody())

	resp := postCoupons(t, ts.URL+"/accounts/acc_1/coupons", string(b))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	json.NewDecoder(resp.Body).Decode(&got)
	if got["batch_no"] != "batch-http-1" || got["count"] != float64(2) {
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
		{"bad json", ts.URL + "/accounts/acc_1/coupons", `not-json`, 400},
		{"invalid", ts.URL + "/accounts/acc_1/coupons", `{"type":"bad"}`, 400},
		{"empty id", ts.URL + "/accounts/%20/coupons",
			`{"type":"discount","rate":90,"scope":"all","count":1,"batch_no":"b1"}`, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := postCoupons(t, c.url, c.body)
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

	h := coupon.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	b, _ := json.Marshal(issueBody())
	resp := postCoupons(t, ts.URL+"/accounts/acc_1/coupons", string(b))
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestTransport_List(t *testing.T) {
	ts, svc := newTestServer(t)
	ctx := context.Background()
	svc.Issue(ctx, validIssue())

	resp, err := http.Get(ts.URL + "/accounts/acc_1/coupons")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var got struct {
		Coupons []coupon.Coupon `json:"coupons"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	if len(got.Coupons) != 3 {
		t.Errorf("coupons = %d, want 3", len(got.Coupons))
	}

	resp2, _ := http.Get(ts.URL + "/accounts/%20/coupons")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("empty id status = %d, want 400", resp2.StatusCode)
	}
}

func TestTransport_List_ServiceError(t *testing.T) {
	svc, db := newService(t)
	sqlDB, _ := db.DB()
	sqlDB.Close()

	h := coupon.NewHandler(svc)
	mux := http.NewServeMux()
	h.Register(mux)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/accounts/acc_1/coupons")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}
