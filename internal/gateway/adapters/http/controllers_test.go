package httpadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	web "github.com/sweetlemon356/lab2-rsoi/internal/gateway/adapters/http"
	app "github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

const (
	hotelID       = "049161bb-badd-4fa8-9d90-87c9a82b0668"
	reservationID = "9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a"
	validBody     = `{"hotelUid":"049161bb-badd-4fa8-9d90-87c9a82b0668","startDate":"2021-10-08","endDate":"2021-10-11"}`
)

type httpFixture struct {
	hotels  func(context.Context, app.PageQuery) (app.HotelPage, error)
	list    func(context.Context, string) ([]app.ReservationView, error)
	get     func(context.Context, string, string) (app.ReservationView, error)
	create  func(context.Context, app.CreateReservationCommand) (app.CreateReservationResult, error)
	cancel  func(context.Context, string, string) error
	loyalty func(context.Context, string) (app.LoyaltyInfo, error)
	profile func(context.Context, string) (app.UserInfoView, error)
	calls   int
}

func (f *httpFixture) ListHotels(ctx context.Context, query app.PageQuery) (app.HotelPage, error) {
	return f.hotels(ctx, query)
}

func (f *httpFixture) ListReservations(ctx context.Context, username string) ([]app.ReservationView, error) {
	return f.list(ctx, username)
}

func (f *httpFixture) GetReservation(ctx context.Context, username, uid string) (app.ReservationView, error) {
	return f.get(ctx, username, uid)
}

func (f *httpFixture) CreateReservation(ctx context.Context, command app.CreateReservationCommand) (app.CreateReservationResult, error) {
	return f.create(ctx, command)
}

func (f *httpFixture) CancelReservation(ctx context.Context, username, uid string) error {
	return f.cancel(ctx, username, uid)
}

func (f *httpFixture) GetLoyalty(ctx context.Context, username string) (app.LoyaltyInfo, error) {
	return f.loyalty(ctx, username)
}

func (f *httpFixture) GetUserInfo(ctx context.Context, username string) (app.UserInfoView, error) {
	return f.profile(ctx, username)
}

func newFixture() *httpFixture {
	f := &httpFixture{}
	date := func(day int) time.Time { return time.Date(2021, 10, day, 0, 0, 0, 0, time.UTC) }
	view := app.ReservationView{ReservationUID: reservationID, Hotel: app.HotelInfo{HotelUID: hotelID, Name: "Ararat Park Hyatt Moscow", FullAddress: "Россия, Москва, Неглинная ул., 4", Stars: 5}, StartDate: date(8), EndDate: date(11), Status: "PAID", Payment: app.PaymentInfo{Status: "PAID", Price: 27000}}
	f.hotels = func(_ context.Context, q app.PageQuery) (app.HotelPage, error) {
		f.calls++
		return app.HotelPage{Page: q.Page, PageSize: q.Size, TotalElements: 1, Items: []app.Hotel{{HotelUID: hotelID, Name: "Ararat Park Hyatt Moscow", Country: "Россия", City: "Москва", Address: "Неглинная ул., 4", Stars: 5, Price: 10000}}}, nil
	}
	f.list = func(context.Context, string) ([]app.ReservationView, error) {
		f.calls++
		return []app.ReservationView{view}, nil
	}
	f.get = func(context.Context, string, string) (app.ReservationView, error) { f.calls++; return view, nil }
	f.create = func(context.Context, app.CreateReservationCommand) (app.CreateReservationResult, error) {
		f.calls++
		return app.CreateReservationResult{ReservationUID: reservationID, HotelUID: hotelID, StartDate: date(8), EndDate: date(11), Discount: 10, Status: "PAID", Payment: view.Payment}, nil
	}
	f.cancel = func(context.Context, string, string) error { f.calls++; return nil }
	f.loyalty = func(context.Context, string) (app.LoyaltyInfo, error) {
		f.calls++
		return app.LoyaltyInfo{Status: "GOLD", Discount: 10, ReservationCount: 25}, nil
	}
	f.profile = func(context.Context, string) (app.UserInfoView, error) {
		f.calls++
		return app.UserInfoView{Reservations: []app.ReservationView{view}, Loyalty: app.LoyaltyInfo{Status: "GOLD", Discount: 10, ReservationCount: 25}}, nil
	}
	return f
}

func (f *httpFixture) router(timeout time.Duration) http.Handler {
	return web.NewRouter(web.NewHotelController(f), web.NewReservationController(f), web.NewLoyaltyController(f), web.NewProfileController(f), timeout, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func request(handler http.Handler, method, path, body, user string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if user != "" {
		r.Header.Set("X-User-Name", user)
	}
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestUsernameBoundaryMatchesBackendStorage(t *testing.T) {
	for _, tc := range []struct {
		user   string
		status int
	}{
		{strings.Repeat("Я", 80), 200},
		{strings.Repeat("a", 81), 400},
		{string([]byte{0xff}), 400},
	} {
		f := newFixture()
		w := request(f.router(time.Second), "GET", "/api/v1/loyalty", "", tc.user)
		if w.Code != tc.status || (tc.status == 400 && f.calls != 0) {
			t.Fatalf("status=%d calls=%d, want=%d", w.Code, f.calls, tc.status)
		}
	}
}

func TestRejectsUnrepresentablePaginationAndDates(t *testing.T) {
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/v1/hotels?page=9223372036854775807&size=10", ""},
		{"POST", "/api/v1/reservations", strings.Replace(validBody, "2021-10-08", "0000-01-01", 1)},
	} {
		f := newFixture()
		w := request(f.router(time.Second), tc.method, tc.path, tc.body, "Test Max")
		if w.Code != 400 || f.calls != 0 {
			t.Fatalf("status=%d calls=%d", w.Code, f.calls)
		}
	}
}

func TestPublicRoutesAndResponseShapes(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, body string
		status                   int
		fields                   []string
	}{
		{"hotels", "GET", "/api/v1/hotels?page=1&size=10", "", 200, []string{"page", "pageSize", "totalElements", "items"}},
		{"profile", "GET", "/api/v1/me", "", 200, []string{"reservations", "loyalty"}},
		{"reservations", "GET", "/api/v1/reservations", "", 200, nil},
		{"reservation", "GET", "/api/v1/reservations/" + reservationID, "", 200, []string{"reservationUid", "hotel", "startDate", "endDate", "status", "payment"}},
		{"create", "POST", "/api/v1/reservations", validBody, 200, []string{"reservationUid", "hotelUid", "startDate", "endDate", "discount", "status", "payment"}},
		{"cancel", "DELETE", "/api/v1/reservations/" + reservationID, "", 204, nil},
		{"loyalty", "GET", "/api/v1/loyalty", "", 200, []string{"status", "discount", "reservationCount"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			w := request(f.router(time.Second), tc.method, tc.path, tc.body, "Test Max")
			if w.Code != tc.status || f.calls != 1 {
				t.Fatalf("status=%d, calls=%d, body=%s", w.Code, f.calls, w.Body.String())
			}
			if tc.status == 204 {
				if w.Body.Len() != 0 {
					t.Fatal("204 has a body")
				}
				return
			}
			if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
				t.Fatal("not a JSON response")
			}
			if tc.name == "reservations" {
				var list []any
				if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 1 {
					t.Fatalf("list=%v, err=%v", list, err)
				}
				return
			}
			var object map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &object); err != nil {
				t.Fatal(err)
			}
			for _, field := range tc.fields {
				if _, ok := object[field]; !ok {
					t.Fatalf("missing %s: %v", field, object)
				}
			}
			if tc.name == "create" {
				if object["discount"] != float64(10) || object["startDate"] != "2021-10-08" || object["endDate"] != "2021-10-11" {
					t.Fatalf("response=%v", object)
				}
			}
		})
	}
}

func TestCreateControllerPassesParsedDatesAndUsername(t *testing.T) {
	f := newFixture()
	f.create = func(_ context.Context, c app.CreateReservationCommand) (app.CreateReservationResult, error) {
		f.calls++
		if c.Username != "Test Max" || c.HotelUID != hotelID || c.StartDate.Format(time.DateOnly) != "2021-10-08" || c.EndDate.Format(time.DateOnly) != "2021-10-11" {
			t.Fatalf("command=%+v", c)
		}
		return app.CreateReservationResult{ReservationUID: reservationID, HotelUID: c.HotelUID, StartDate: c.StartDate, EndDate: c.EndDate, Status: "PAID", Discount: 10, Payment: app.PaymentInfo{Status: "PAID", Price: 27000}}, nil
	}
	w := request(f.router(time.Second), "POST", "/api/v1/reservations", validBody, "Test Max")
	if w.Code != 200 || f.calls != 1 {
		t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
	}
}

func TestSuccessfulResponsesMatchExactPublicJSONContract(t *testing.T) {
	booking := `{"reservationUid":"` + reservationID + `","hotel":{"hotelUid":"` + hotelID + `","name":"Ararat Park Hyatt Moscow","fullAddress":"Россия, Москва, Неглинная ул., 4","stars":5},"startDate":"2021-10-08","endDate":"2021-10-11","status":"PAID","payment":{"status":"PAID","price":27000}}`
	loyalty := `{"status":"GOLD","discount":10,"reservationCount":25}`
	for _, tc := range []struct{ method, path, body, want string }{
		{"GET", "/api/v1/reservations/" + reservationID, "", booking},
		{"GET", "/api/v1/reservations", "", "[" + booking + "]"},
		{"GET", "/api/v1/loyalty", "", loyalty},
		{"GET", "/api/v1/me", "", `{"reservations":[` + booking + `],"loyalty":` + loyalty + `}`},
		{"POST", "/api/v1/reservations", validBody, `{"reservationUid":"` + reservationID + `","hotelUid":"` + hotelID + `","startDate":"2021-10-08","endDate":"2021-10-11","discount":10,"status":"PAID","payment":{"status":"PAID","price":27000}}`},
		{"GET", "/api/v1/hotels?page=1&size=10", "", `{"page":1,"pageSize":10,"totalElements":1,"items":[{"hotelUid":"` + hotelID + `","name":"Ararat Park Hyatt Moscow","country":"Россия","city":"Москва","address":"Неглинная ул., 4","stars":5,"price":10000}]}`},
	} {
		w := request(newFixture().router(time.Second), tc.method, tc.path, tc.body, "Test Max")
		var got, want any
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(tc.want), &want); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s %s: status=%d\ngot=%s\nwant=%s", tc.method, tc.path, w.Code, w.Body.String(), tc.want)
		}
	}
}

func TestProtectedRoutesRequireNonblankUsername(t *testing.T) {
	for _, route := range []struct{ method, path, body string }{
		{"GET", "/api/v1/me", ""}, {"GET", "/api/v1/loyalty", ""}, {"GET", "/api/v1/reservations", ""},
		{"GET", "/api/v1/reservations/" + reservationID, ""}, {"DELETE", "/api/v1/reservations/" + reservationID, ""}, {"POST", "/api/v1/reservations", validBody},
	} {
		for _, user := range []string{"", "   "} {
			f := newFixture()
			w := request(f.router(time.Second), route.method, route.path, route.body, user)
			if w.Code != 400 || f.calls != 0 {
				t.Fatalf("%s %s: status=%d, calls=%d", route.method, route.path, w.Code, f.calls)
			}
		}
	}
}

func TestCreateControllerRejectsMalformedInputBeforeScenario(t *testing.T) {
	for _, body := range []string{
		`{`, `null`, `[]`, `{}`, validBody + ` {}`,
		strings.Replace(validBody, hotelID, "not-a-uuid", 1),
		strings.Replace(validBody, `"2021-10-08"`, `123`, 1),
		strings.Replace(validBody, "2021-10-08", "2021-02-30", 1),
		strings.Replace(validBody, "2021-10-11", "2021-10-11T00:00:00Z", 1),
		`{"hotelUid":"` + hotelID + `","startDate":"2021-10-08"}`,
		strings.Repeat(" ", 1024*1024+1) + validBody,
	} {
		f := newFixture()
		w := request(f.router(time.Second), "POST", "/api/v1/reservations", body, "Test Max")
		if w.Code != 400 || f.calls != 0 {
			t.Fatalf("body length=%d: status=%d, calls=%d", len(body), w.Code, f.calls)
		}
		var response struct {
			Message string
			Errors  []struct{ Field, Error string }
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || response.Message == "" || len(response.Errors) == 0 {
			t.Fatalf("validation body=%s, err=%v", w.Body.String(), err)
		}
	}
}

func TestReservationControllerValidatesPathUUID(t *testing.T) {
	for _, method := range []string{"GET", "DELETE"} {
		f := newFixture()
		w := request(f.router(time.Second), method, "/api/v1/reservations/not-a-uuid", "", "Test Max")
		if w.Code != 400 || f.calls != 0 {
			t.Fatalf("status=%d, calls=%d", w.Code, f.calls)
		}
	}
}

func TestPaginationDefaultsAndBounds(t *testing.T) {
	for _, tc := range []struct {
		query              string
		page, size, status int
	}{
		{"", 1, 10, 200}, {"?page=0&size=1", 0, 1, 200}, {"?page=3&size=100", 3, 100, 200},
		{"?page=-1", 0, 0, 400}, {"?page=x", 0, 0, 400}, {"?size=0", 0, 0, 400}, {"?size=101", 0, 0, 400}, {"?size=x", 0, 0, 400},
	} {
		f := newFixture()
		f.hotels = func(_ context.Context, q app.PageQuery) (app.HotelPage, error) {
			f.calls++
			if q.Page != tc.page || q.Size != tc.size {
				t.Fatalf("query=%+v", q)
			}
			return app.HotelPage{Page: q.Page, PageSize: q.Size}, nil
		}
		w := request(f.router(time.Second), "GET", "/api/v1/hotels"+tc.query, "", "")
		if w.Code != tc.status || (tc.status == 400 && f.calls != 0) {
			t.Fatalf("query=%s, status=%d, calls=%d", tc.query, w.Code, f.calls)
		}
	}
}

func TestErrorsMapToPublicStatusWithoutLeakingDetails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"validation", &app.ValidationError{Fields: []app.FieldError{{Field: "endDate", Error: "must follow startDate"}}}, 400},
		{"missing", app.ErrNotFound, 404}, {"deadline", context.DeadlineExceeded, 504}, {"canceled", context.Canceled, 408},
		{"dependency", &app.DependencyError{Service: "payment", Err: errors.New("secret connection string")}, 502},
		{"unexpected", errors.New("secret connection string"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			f.profile = func(context.Context, string) (app.UserInfoView, error) { return app.UserInfoView{}, tc.err }
			w := request(f.router(time.Second), "GET", "/api/v1/me", "", "Test Max")
			if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
				t.Fatalf("status=%d, body=%s", w.Code, w.Body.String())
			}
		})
	}
}

func TestHTTPDeadlineCoversWholeScenario(t *testing.T) {
	f := newFixture()
	f.profile = func(ctx context.Context, _ string) (app.UserInfoView, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("request has no overall deadline")
		}
		<-ctx.Done()
		return app.UserInfoView{}, ctx.Err()
	}
	w := request(f.router(time.Millisecond), "GET", "/api/v1/me", "", "Test Max")
	if w.Code != 504 {
		t.Fatalf("status=%d", w.Code)
	}
}

func TestEmptyCollectionsAreJSONArrayAndInternalIDsStayPrivate(t *testing.T) {
	f := newFixture()
	f.list = func(context.Context, string) ([]app.ReservationView, error) { return nil, nil }
	w := request(f.router(time.Second), "GET", "/api/v1/reservations", "", "Test Max")
	if strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("body=%s", w.Body.String())
	}
	f.profile = func(context.Context, string) (app.UserInfoView, error) {
		return app.UserInfoView{Loyalty: app.LoyaltyInfo{Status: "BRONZE", Discount: 5}}, nil
	}
	w = request(f.router(time.Second), "GET", "/api/v1/me", "", "Test Max")
	if !strings.Contains(w.Body.String(), `"reservations":[]`) {
		t.Fatalf("body=%s", w.Body.String())
	}
	f = newFixture()
	w = request(f.router(time.Second), "GET", "/api/v1/reservations/"+reservationID, "", "Test Max")
	if strings.Contains(w.Body.String(), "paymentUid") || strings.Contains(w.Body.String(), "username") {
		t.Fatalf("private fields leaked: %s", w.Body.String())
	}
}

func TestHealthDoesNotRequireUserOrDownstreamServices(t *testing.T) {
	f := newFixture()
	w := request(f.router(time.Second), "GET", "/manage/health", "", "")
	if w.Code != 200 || f.calls != 0 {
		t.Fatalf("status=%d, calls=%d", w.Code, f.calls)
	}
}

func TestRouterPreservesMethodNotAllowed(t *testing.T) {
	f := newFixture()
	w := request(f.router(time.Second), "POST", "/api/v1/hotels", "", "")
	if w.Code != http.StatusMethodNotAllowed || f.calls != 0 || !strings.Contains(w.Header().Get("Allow"), "GET") {
		t.Fatalf("status=%d, allow=%q, calls=%d", w.Code, w.Header().Get("Allow"), f.calls)
	}
}
