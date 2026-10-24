package httpadapter_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	h "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/adapters/http"
	d "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
)

const id = "9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a"

type useCase struct {
	user, uid, action string
	err               error
}

func TestLoyaltyReceivesEncodedSlashAndUnicodeAsOneUsername(t *testing.T) {
	user := "Test Max/Иван"
	path := "/internal/v1/loyalty/" + url.PathEscape(user)
	s := &useCase{}
	w := call(s, "GET", path)
	if w.Code != 200 || s.user != user {
		t.Fatalf("status=%d,user=%q", w.Code, s.user)
	}
	s = &useCase{}
	w = call(s, "PUT", path+"/reservations/"+id)
	if w.Code != 204 || s.user != user || s.uid != id {
		t.Fatalf("status=%d,case=%+v", w.Code, s)
	}
}

func (s *useCase) Get(_ context.Context, u string) (d.Account, error) {
	s.user = u
	if s.err != nil {
		return d.Account{}, s.err
	}
	return d.NewAccount(u, 25)
}

func (s *useCase) Add(_ context.Context, u, id string) error {
	s.user = u
	s.uid = id
	s.action = "add"
	return s.err
}

func (s *useCase) Remove(_ context.Context, u, id string) error {
	s.user = u
	s.uid = id
	s.action = "remove"
	return s.err
}

func call(s *useCase, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.NewHandler(s, func(context.Context) error { return nil }, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestLoyaltyHTTPContractAndEncodedIdentity(t *testing.T) {
	s := &useCase{}
	w := call(s, "GET", "/internal/v1/loyalty/Test%20Max")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != `{"status":"GOLD","discount":10,"reservationCount":25}` || s.user != "Test Max" {
		t.Fatalf("status=%d,body=%s,user=%s", w.Code, w.Body.String(), s.user)
	}
	for _, tc := range []struct{ method, path, user, action string }{{"PUT", "/internal/v1/loyalty/Test%20Max/reservations/" + id, "Test Max", "add"}, {"DELETE", "/internal/v1/loyalty/%2E%2E/reservations/" + id, "..", "remove"}} {
		s := &useCase{}
		w := call(s, tc.method, tc.path)
		if w.Code != 204 || s.user != tc.user || s.uid != id || s.action != tc.action || w.Body.Len() != 0 {
			t.Fatalf("status=%d,usecase=%+v", w.Code, s)
		}
	}
	for _, path := range []string{"/internal/v1/loyalty/%20/reservations/" + id, "/internal/v1/loyalty/user/reservations/bad"} {
		s := &useCase{}
		w := call(s, "PUT", path)
		if w.Code != 400 || s.action != "" {
			t.Fatalf("status=%d", w.Code)
		}
	}
}

func TestLoyaltyConflictAndFailureMapping(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{d.ErrConflict, 409}, {d.ErrCanceled, 409}, {d.ErrInvalid, 400}, {errors.New("secret"), 500}} {
		w := call(&useCase{err: tc.err}, "PUT", "/internal/v1/loyalty/user/reservations/"+id)
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("status=%d,body=%s", w.Code, w.Body.String())
		}
	}
}
