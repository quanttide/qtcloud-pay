package order

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func TestErrMapper(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"account not found", account.ErrNotFound, http.StatusNotFound},
		{"insufficient balance", billing.ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{"invalid request", ErrInvalidRequest, http.StatusBadRequest},
		{"invalid amount", billing.ErrInvalidAmount, http.StatusBadRequest},
		{"coupon unavailable", coupon.ErrUnavailable, http.StatusConflict},
		{"voucher unavailable", voucher.ErrUnavailable, http.StatusConflict},
		{"unhandled", errors.New("boom"), 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errMapper(c.err); got != c.want {
				t.Errorf("errMapper(%v) = %d, want %d", c.err, got, c.want)
			}
		})
	}
}

func TestFormatSettleDetail(t *testing.T) {
	// 正常明细：存储分 → Money 对象（整数分 + CNY）
	raw := json.RawMessage(`[{"kind":"coupon","ref_id":1,"amount":1000}]`)
	got := formatSettleDetail(raw)
	want := `[{"kind":"coupon","ref_id":1,"amount":{"amount":1000,"currency":"CNY"}}]`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}

	// 非法 JSON：原样返回（不破坏响应）
	bad := json.RawMessage(`bad`)
	if string(formatSettleDetail(bad)) != "bad" {
		t.Errorf("invalid raw should pass through")
	}

	// 空 / null：原样返回
	if formatSettleDetail(nil) != nil {
		t.Errorf("nil raw should stay nil")
	}
	if string(formatSettleDetail(json.RawMessage(`null`))) != "null" {
		t.Errorf("null raw should stay null")
	}
}
