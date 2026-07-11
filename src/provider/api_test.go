package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// mockProvider for API testing
type apiMockProvider struct{}

func (m *apiMockProvider) Name() string { return "mock" }

func (m *apiMockProvider) Pay(_ context.Context, req *PayRequest) (*PayResponse, error) {
	return &PayResponse{TradeID: "mock_tx_" + req.OrderID, PayURL: "https://pay.example.com/" + req.OrderID}, nil
}

func (m *apiMockProvider) Query(_ context.Context, orderID string) (*OrderStatus, error) {
	return &OrderStatus{
		TradeID: "mock_tx_" + orderID, OrderID: orderID,
		Status: "SUCCESS", Amount: 99.99, PaidAt: "2025-07-11T10:00:00+08:00",
	}, nil
}

func (m *apiMockProvider) Refund(_ context.Context, req *RefundRequest) (*RefundResponse, error) {
	return &RefundResponse{RefundID: "mock_rf_" + req.OrderID, Status: "SUCCESS"}, nil
}

type apiErrProvider struct{}

func (m *apiErrProvider) Name() string { return "err" }
func (m *apiErrProvider) Pay(_ context.Context, _ *PayRequest) (*PayResponse, error) {
	return nil, fmt.Errorf("provider unavailable")
}
func (m *apiErrProvider) Query(_ context.Context, _ string) (*OrderStatus, error) {
	return nil, fmt.Errorf("order not found")
}
func (m *apiErrProvider) Refund(_ context.Context, _ *RefundRequest) (*RefundResponse, error) {
	return nil, fmt.Errorf("refund rejected")
}

func TestAPI_Pay_Success(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	body := bytes.NewReader(mustJSON(t, &PayRequest{OrderID: "ORD001", Amount: 99.99, Subject: "测试课程"}))
	resp, err := http.Post(hs.URL+"/pay", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got PayResponse
	json.NewDecoder(resp.Body).Decode(&got)
	if got.TradeID != "mock_tx_ORD001" {
		t.Errorf("TradeID = %q", got.TradeID)
	}
}

func TestAPI_Pay_InvalidBody(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Post(hs.URL+"/pay", "application/json", bytes.NewReader([]byte(`not-json`)))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPI_Pay_ProviderError(t *testing.T) {
	s := NewServer(":0", &apiErrProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	body := bytes.NewReader(mustJSON(t, &PayRequest{OrderID: "ORD001", Amount: 99.99, Subject: "测试"}))
	resp, _ := http.Post(hs.URL+"/pay", "application/json", body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAPI_Query_Success(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Get(hs.URL + "/query/ORD001")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got OrderStatus
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if got.OrderID != "ORD001" || got.Status != "SUCCESS" {
		t.Errorf("got OrderID=%q Status=%q", got.OrderID, got.Status)
	}
}

func TestAPI_Query_EmptyOrderID(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Get(hs.URL + "/query/%20")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPI_Query_NotFound(t *testing.T) {
	s := NewServer(":0", &apiErrProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Get(hs.URL + "/query/NONEXISTENT")
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAPI_Refund_Success(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	body := bytes.NewReader(mustJSON(t, &RefundRequest{OrderID: "ORD001", RefundAmount: 50.00, Reason: "部分退款"}))
	resp, _ := http.Post(hs.URL+"/refund", "application/json", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var got RefundResponse
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if got.RefundID != "mock_rf_ORD001" || got.Status != "SUCCESS" {
		t.Errorf("got RefundID=%q Status=%q", got.RefundID, got.Status)
	}
}

func TestAPI_Refund_ProviderError(t *testing.T) {
	s := NewServer(":0", &apiErrProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	body := bytes.NewReader(mustJSON(t, &RefundRequest{OrderID: "ORD001", RefundAmount: 99.99, Reason: "退款"}))
	resp, _ := http.Post(hs.URL+"/refund", "application/json", body)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", resp.StatusCode)
	}
}

func TestAPI_Refund_MissingBody(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Post(hs.URL+"/refund", "application/json", strings.NewReader(``))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestAPI_Pay_WrongMethod(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Get(hs.URL + "/pay")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestAPI_Query_WrongMethod(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Post(hs.URL+"/query/ORD001", "text/plain", nil)
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestAPI_Refund_WrongMethod(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Get(hs.URL + "/refund")
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

func TestAPI_Refund_InvalidBody(t *testing.T) {
	s := NewServer(":0", &apiMockProvider{})
	hs := httptest.NewServer(s.Handler())
	defer hs.Close()

	resp, _ := http.Post(hs.URL+"/refund", "application/json", bytes.NewReader([]byte(`not-json`)))
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
