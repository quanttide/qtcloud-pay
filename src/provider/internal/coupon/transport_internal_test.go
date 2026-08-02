package coupon

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteServiceError(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ErrInvalidIssue, http.StatusBadRequest},
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
	writeJSON(w, http.StatusOK, make(chan int)) // 不可序列化，走错误分支
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
