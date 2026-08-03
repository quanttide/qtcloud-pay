package account

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestErrMapper(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", ErrNotFound, http.StatusNotFound},
		{"exists", ErrExists, http.StatusConflict},
		{"insufficient balance", ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{"invalid amount", ErrInvalidAmount, http.StatusBadRequest},
		{"invalid recharge", ErrInvalidRecharge, http.StatusBadRequest},
		{"invalid refund", ErrInvalidRefund, http.StatusBadRequest},
		{"canceled", context.Canceled, http.StatusBadRequest},
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
