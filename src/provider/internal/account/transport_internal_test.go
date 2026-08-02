package account

import (
	"context"
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
		{ErrNotFound, http.StatusNotFound},
		{ErrExists, http.StatusConflict},
		{ErrInvalidAmount, http.StatusBadRequest},
		{ErrInvalidRecharge, http.StatusBadRequest},
		{context.Canceled, http.StatusBadRequest},
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

func TestParsePagination(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	if l, o := parsePagination(r); l != 20 || o != 0 {
		t.Errorf("default = %d/%d, want 20/0", l, o)
	}

	r = httptest.NewRequest(http.MethodGet, "/x?limit=5&offset=10", nil)
	if l, o := parsePagination(r); l != 5 || o != 10 {
		t.Errorf("custom = %d/%d, want 5/10", l, o)
	}

	r = httptest.NewRequest(http.MethodGet, "/x?limit=999&offset=-1", nil)
	if l, o := parsePagination(r); l != 100 || o != 0 {
		t.Errorf("invalid = %d/%d, want 100/0", l, o)
	}

	r = httptest.NewRequest(http.MethodGet, "/x?limit=abc&offset=xyz", nil)
	if l, o := parsePagination(r); l != 20 || o != 0 {
		t.Errorf("non-numeric = %d/%d, want 20/0", l, o)
	}
}

func TestWriteJSON_EncodeError(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, make(chan int)) // 不可序列化，走错误分支
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
