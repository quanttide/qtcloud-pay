package order

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/quanttide/qtcloud-pay/src/provider/internal/account"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/billing"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/coupon"
	"github.com/quanttide/qtcloud-pay/src/provider/internal/voucher"
)

func TestWriteServiceError(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{account.ErrNotFound, http.StatusNotFound},
		{billing.ErrInsufficientBalance, http.StatusUnprocessableEntity},
		{ErrInvalidRequest, http.StatusBadRequest},
		{billing.ErrInvalidAmount, http.StatusBadRequest},
		{coupon.ErrUnavailable, http.StatusConflict},
		{voucher.ErrUnavailable, http.StatusConflict},
		{errors.New("boom"), http.StatusInternalServerError},
	}
	for _, c := range cases {
		w := httptest.NewRecorder()
		writeServiceError(w, c.err)
		if w.Code != c.want {
			t.Errorf("err=%v status=%d, want %d", c.err, w.Code, c.want)
		}
	}
}

func TestWriteJSON_EncodeError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, make(chan int))
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
