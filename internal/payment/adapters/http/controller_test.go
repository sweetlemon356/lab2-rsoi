package httpadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	h "github.com/sweetlemon356/lab2-rsoi/internal/payment/adapters/http"
	d "github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
)

const id = "5c9ced89-f43a-4ac4-a516-e6660e5bddcb"

type useCase struct {
	calls int
	err   error
}

func (s *useCase) Get(context.Context, string) (d.Payment, error) {
	s.calls++
	return d.Payment{UID: id, Status: "CANCELED", Price: 27000}, s.err
}

func (s *useCase) Create(ctx context.Context, uid string, price int64) (d.Payment, error) {
	s.calls++
	if uid != id {
		return d.Payment{}, errors.New("wrong UID")
	}
	if _, ok := ctx.Deadline(); !ok {
		return d.Payment{}, errors.New("missing deadline")
	}
	if s.err != nil {
		return d.Payment{}, s.err
	}
	return d.NewPayment(uid, price)
}

func (s *useCase) Cancel(context.Context, string) error { s.calls++; return s.err }

func handler(s *useCase) http.Handler {
	return h.NewHandler(s, func(context.Context) error { return nil }, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func call(s *useCase, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	handler(s).ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(body)))
	return w
}

func TestPaymentHTTPContractAndValidation(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		status             int
		want               string
	}{
		{"PUT", "/internal/v1/payments/" + id, `{"price":27000}`, 201, `{"status":"PAID","price":27000}`},
		{"GET", "/internal/v1/payments/" + id, "", 200, `{"status":"CANCELED","price":27000}`},
		{"PUT", "/internal/v1/payments/" + id + "/cancellation", "", 204, ""},
	} {
		s := &useCase{}
		w := call(s, tc.method, tc.path, tc.body)
		if w.Code != tc.status || strings.TrimSpace(w.Body.String()) != tc.want || s.calls != 1 {
			t.Fatalf("status=%d,body=%s,calls=%d", w.Code, w.Body.String(), s.calls)
		}
	}
	for _, body := range []string{`{}`, `null`, `{"price":null}`, `{"price":1.5}`, `{`, `{"price":10} {}`} {
		s := &useCase{}
		w := call(s, "PUT", "/internal/v1/payments/"+id, body)
		if w.Code != 400 || s.calls != 0 {
			t.Fatalf("body=%s,status=%d,calls=%d", body, w.Code, s.calls)
		}
	}
	s := &useCase{}
	if w := call(s, "GET", "/internal/v1/payments/bad", ""); w.Code != 400 || s.calls != 0 {
		t.Fatal("bad UUID accepted")
	}
}

func TestPaymentErrorsAreSanitizedAndHealthChecksDatabase(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{d.ErrNotFound, 404}, {d.ErrConflict, 409}, {d.ErrCanceled, 409}, {d.ErrInvalid, 400}, {context.DeadlineExceeded, 504}, {errors.New("secret DSN"), 500}} {
		w := call(&useCase{err: tc.err}, "GET", "/internal/v1/payments/"+id, "")
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("status=%d,body=%s", w.Code, w.Body.String())
		}
	}
	s := &useCase{}
	w := httptest.NewRecorder()
	h.NewHandler(s, func(context.Context) error { return errors.New("database down") }, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(w, httptest.NewRequest("GET", "/manage/health", nil))
	if w.Code != 503 || s.calls != 0 {
		t.Fatalf("health=%d", w.Code)
	}
}
