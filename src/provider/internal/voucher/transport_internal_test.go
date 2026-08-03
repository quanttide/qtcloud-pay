package voucher

import (
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
		{"invalid issue", ErrInvalidIssue, http.StatusBadRequest},
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
