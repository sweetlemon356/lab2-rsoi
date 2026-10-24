package httpadapter_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	h "github.com/sweetlemon356/lab2-rsoi/internal/reservation/adapters/http"
	a "github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
	d "github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

const (
	id        = "9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a"
	hotelID   = "049161bb-badd-4fa8-9d90-87c9a82b0668"
	paymentID = "5c9ced89-f43a-4ac4-a516-e6660e5bddcb"
)

type useCase struct {
	calls     int
	user, uid string
	filled    bool
	command   a.CreateCommand
	err       error
}

func (s *useCase) ListHotels(_ context.Context, q a.PageQuery) (a.HotelPage, error) {
	s.calls++
	if s.filled {
		return a.HotelPage{Page: q.Page, PageSize: q.Size, TotalElements: 1, Items: []d.Hotel{{UID: hotelID, Name: "Hotel", Country: "Россия", City: "Москва", Address: "Address", Stars: 5, Price: 10000}}}, nil
	}
	return a.HotelPage{Page: q.Page, PageSize: q.Size, TotalElements: 0}, s.err
}

func (s *useCase) Quote(_ context.Context, uid string, start, end time.Time, discount int) (a.Quote, error) {
	s.calls++
	if uid != hotelID || discount != 10 {
		return a.Quote{}, d.ErrInvalid
	}
	stay, err := d.NewStay(start, end)
	return a.Quote{HotelUID: uid, Stay: stay, Price: 27000}, err
}

func (s *useCase) ListReservations(_ context.Context, user string) ([]a.Details, error) {
	s.calls++
	s.user = user
	return nil, s.err
}

func (s *useCase) GetReservation(_ context.Context, user, uid string) (a.Details, error) {
	s.calls++
	s.user = user
	s.uid = uid
	if s.err != nil {
		return a.Details{}, s.err
	}
	stay, _ := d.NewStay(time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC), time.Date(2021, 10, 11, 0, 0, 0, 0, time.UTC))
	return a.Details{Reservation: d.Reservation{UID: uid, Username: user, PaymentUID: paymentID, HotelUID: hotelID, Stay: stay, Status: "PAID"}, Hotel: d.Hotel{UID: hotelID, Name: "Hotel", Country: "Россия", City: "Москва", Address: "Address", Stars: 5}}, nil
}

func (s *useCase) Create(_ context.Context, c a.CreateCommand) error {
	s.calls++
	s.command = c
	return s.err
}

func (s *useCase) Cancel(_ context.Context, user, uid string) error {
	s.calls++
	s.user = user
	s.uid = uid
	return s.err
}

func call(s *useCase, method, path, body, user string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("X-User-Name", user)
	h.NewHandler(s, s, func(context.Context) error { return nil }, time.Second, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(w, r)
	return w
}

func TestReservationIdentityAndCompleteQuoteHotelContracts(t *testing.T) {
	for _, path := range []string{"/internal/v1/reservations", "/internal/v1/reservations/" + id} {
		for _, user := range []string{"", "  "} {
			s := &useCase{}
			w := call(s, "GET", path, "", user)
			if w.Code != 400 || s.calls != 0 {
				t.Fatalf("missing owner accepted: %d", w.Code)
			}
		}
	}
	s := &useCase{}
	w := call(s, "GET", "/internal/v1/reservations", "", "Test Max")
	if w.Code != 200 || s.user != "Test Max" {
		t.Fatal("list owner lost")
	}
	s = &useCase{}
	w = call(s, "PUT", "/internal/v1/reservations/"+id+"/cancellation", `{"username":"Test Max"}`, "")
	if w.Code != 204 || s.user != "Test Max" || s.uid != id {
		t.Fatalf("cancel identity=%+v", s)
	}
	for _, path := range []string{"/internal/v1/reservations/bad", "/internal/v1/reservations/bad/cancellation"} {
		s := &useCase{}
		w := call(s, "PUT", path, `{}`, "")
		if w.Code != 400 || s.calls != 0 {
			t.Fatalf("bad UUID accepted: %d", w.Code)
		}
	}
	w = call(&useCase{}, "POST", "/internal/v1/reservations/quote", `{"hotelUid":"`+hotelID+`","startDate":"2021-10-08","endDate":"2021-10-11","discount":10}`, "")
	var quote map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &quote); err != nil {
		t.Fatal(err)
	}
	if len(quote) != 4 || quote["hotelUid"] != hotelID || quote["startDate"] != "2021-10-08" || quote["endDate"] != "2021-10-11" || quote["price"] != float64(27000) {
		t.Fatalf("quote=%v", quote)
	}
	w = call(&useCase{filled: true}, "GET", "/internal/v1/hotels?page=1&size=10", "", "")
	var page struct {
		Page  int              `json:"page"`
		Size  int              `json:"pageSize"`
		Total int              `json:"totalElements"`
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || page.Page != 1 || page.Size != 10 || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
	item := page.Items[0]
	if len(item) != 7 || item["hotelUid"] != hotelID || item["name"] != "Hotel" || item["country"] != "Россия" || item["city"] != "Москва" || item["address"] != "Address" || item["stars"] != float64(5) || item["price"] != float64(10000) {
		t.Fatalf("hotel=%v", item)
	}
}

func TestReservationHTTPMapsCalendarDatesAndPrivateReferences(t *testing.T) {
	s := &useCase{}
	w := call(s, "GET", "/internal/v1/reservations/"+id, "", "Test Max")
	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	hotel, ok := got["hotel"].(map[string]any)
	if w.Code != 200 || !ok || hotel["fullAddress"] != "Россия, Москва, Address" || got["paymentUid"] != paymentID || got["username"] != "Test Max" || got["startDate"] != "2021-10-08" || got["endDate"] != "2021-10-11" {
		t.Fatalf("body=%s", w.Body.String())
	}
	quote := `{"hotelUid":"` + hotelID + `","startDate":"2021-10-08","endDate":"2021-10-11","discount":10}`
	w = call(&useCase{}, "POST", "/internal/v1/reservations/quote", quote, "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"price":27000`) {
		t.Fatalf("quote=%s", w.Body.String())
	}
	w = call(&useCase{}, "GET", "/internal/v1/reservations", "", "Test Max")
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("list=%s", w.Body.String())
	}
	w = call(&useCase{}, "GET", "/internal/v1/hotels?page=0&size=1", "", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatalf("hotels=%s", w.Body.String())
	}
}

func TestReservationWritesValidateAndPassIdentity(t *testing.T) {
	body := `{"username":"Test Max","paymentUid":"` + paymentID + `","hotelUid":"` + hotelID + `","startDate":"2021-10-08","endDate":"2021-10-11"}`
	s := &useCase{}
	w := call(s, "PUT", "/internal/v1/reservations/"+id, body, "")
	if w.Code != 201 || s.command.UID != id || s.command.PaymentUID != paymentID || s.command.Username != "Test Max" || s.command.StartDate.Format(time.DateOnly) != "2021-10-08" {
		t.Fatalf("status=%d,command=%+v", w.Code, s.command)
	}
	for _, bad := range []string{`null`, `{}`, strings.Replace(body, paymentID, "bad", 1), strings.Replace(body, "2021-10-08", "2021-02-30", 1), body + ` {}`} {
		s := &useCase{}
		w := call(s, "PUT", "/internal/v1/reservations/"+id, bad, "")
		if w.Code != 400 || s.calls != 0 {
			t.Fatalf("status=%d,body=%s", w.Code, bad)
		}
	}
	w = call(&useCase{err: d.ErrNotFound}, "GET", "/internal/v1/reservations/"+id, "", "other")
	if w.Code != 404 {
		t.Fatalf("foreign status=%d", w.Code)
	}
	w = call(&useCase{}, "PUT", "/internal/v1/reservations/"+id+"/cancellation", `{"username":"Test Max"}`, "")
	if w.Code != 204 || w.Body.Len() != 0 {
		t.Fatalf("cancel=%d", w.Code)
	}
}
