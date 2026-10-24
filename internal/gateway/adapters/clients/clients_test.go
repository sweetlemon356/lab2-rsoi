package clients_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/adapters/clients"
	app "github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

const (
	hotelID         = "049161bb-badd-4fa8-9d90-87c9a82b0668"
	reservationID   = "9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a"
	paymentID       = "5c9ced89-f43a-4ac4-a516-e6660e5bddcb"
	reservationJSON = `{"reservationUid":"9b4ba1f7-e5ac-465b-ace4-7b54dec20f9a","username":"Test Max","paymentUid":"5c9ced89-f43a-4ac4-a516-e6660e5bddcb","hotel":{"hotelUid":"049161bb-badd-4fa8-9d90-87c9a82b0668","name":"Hotel","fullAddress":"Address","stars":5},"startDate":"2021-10-08","endDate":"2021-10-11","status":"PAID"}`
)

func TestReservationReaderUsesScopedRequestsAndParsesDates(t *testing.T) {
	for _, list := range []bool{false, true} {
		t.Run(fmt.Sprint(list), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantPath := "/internal/v1/reservations"
				if !list {
					wantPath += "/" + reservationID
				}
				if r.Method != "GET" || r.URL.Path != wantPath || r.Header.Get("X-User-Name") != "Test Max" {
					t.Errorf("request=%s %s, user=%q", r.Method, r.URL, r.Header.Get("X-User-Name"))
				}
				w.Header().Set("Content-Type", "application/json")
				if list {
					fmt.Fprint(w, "["+reservationJSON+"]")
				} else {
					fmt.Fprint(w, reservationJSON)
				}
			}))
			defer server.Close()
			client := clients.NewReservationHTTPClient(server.URL, server.Client())
			var got app.ReservationData
			var err error
			if list {
				var rows []app.ReservationData
				rows, err = client.ListByUser(context.Background(), "Test Max")
				if err == nil {
					if len(rows) != 1 {
						t.Fatalf("rows=%v", rows)
					}
					got = rows[0]
				}
			} else {
				got, err = client.GetByUser(context.Background(), "Test Max", reservationID)
			}
			if err != nil || got.Username != "Test Max" || got.PaymentUID != paymentID || got.Hotel.HotelUID != hotelID || got.StartDate.Format(time.DateOnly) != "2021-10-08" || got.EndDate.Format(time.DateOnly) != "2021-10-11" {
				t.Fatalf("result=%+v, err=%v", got, err)
			}
		})
	}
}

func TestHotelReaderPassesPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/internal/v1/hotels" || r.URL.Query().Get("page") != "2" || r.URL.Query().Get("size") != "10" {
			t.Errorf("request=%s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"page":2,"pageSize":10,"totalElements":1,"items":[{"hotelUid":"`+hotelID+`","name":"Hotel","country":"Россия","city":"Москва","address":"Address","stars":5,"price":10000}]}`)
	}))
	defer server.Close()
	got, err := clients.NewReservationHTTPClient(server.URL, server.Client()).ListHotels(context.Background(), app.PageQuery{Page: 2, Size: 10})
	if err != nil || got.Page != 2 || len(got.Items) != 1 || got.Items[0].Price != 10000 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestWriteClientsUseStableIDsAndDocumentedWireBodies(t *testing.T) {
	start := time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 3)
	for _, tc := range []struct {
		name, method, path, response string
		status                       int
		body                         map[string]any
		invoke                       func(string, *http.Client) error
	}{
		{"quote", "POST", "/internal/v1/reservations/quote", `{"hotelUid":"` + hotelID + `","startDate":"2021-10-08","endDate":"2021-10-11","price":27000}`, 200, map[string]any{"hotelUid": hotelID, "startDate": "2021-10-08", "endDate": "2021-10-11", "discount": float64(10)}, func(base string, h *http.Client) error {
			q, e := clients.NewReservationHTTPClient(base, h).Quote(context.Background(), app.QuoteRequest{HotelUID: hotelID, StartDate: start, EndDate: end, Discount: 10})
			if e == nil && q.Price != 27000 {
				t.Errorf("quote=%+v", q)
			}
			return e
		}},
		{"reservation", "PUT", "/internal/v1/reservations/" + reservationID, "", 201, map[string]any{"username": "Test Max", "paymentUid": paymentID, "hotelUid": hotelID, "startDate": "2021-10-08", "endDate": "2021-10-11"}, func(base string, h *http.Client) error {
			return clients.NewReservationHTTPClient(base, h).Create(context.Background(), app.SaveReservationCommand{ReservationUID: reservationID, Username: "Test Max", PaymentUID: paymentID, HotelUID: hotelID, StartDate: start, EndDate: end})
		}},
		{"payment", "PUT", "/internal/v1/payments/" + paymentID, `{"status":"PAID","price":27000}`, 201, map[string]any{"price": float64(27000)}, func(base string, h *http.Client) error {
			_, e := clients.NewPaymentHTTPClient(base, h).Create(context.Background(), app.CreatePaymentCommand{PaymentUID: paymentID, Price: 27000})
			return e
		}},
		{"cancel payment", "PUT", "/internal/v1/payments/" + paymentID + "/cancellation", "", 204, nil, func(base string, h *http.Client) error {
			return clients.NewPaymentHTTPClient(base, h).Cancel(context.Background(), paymentID)
		}},
		{"cancel reservation", "PUT", "/internal/v1/reservations/" + reservationID + "/cancellation", "", 204, map[string]any{"username": "Test Max"}, func(base string, h *http.Client) error {
			return clients.NewReservationHTTPClient(base, h).Cancel(context.Background(), "Test Max", reservationID)
		}},
		{"add loyalty", "PUT", "/internal/v1/loyalty/Test%20Max/reservations/" + reservationID, "", 204, nil, func(base string, h *http.Client) error {
			return clients.NewLoyaltyHTTPClient(base, h).AddReservation(context.Background(), "Test Max", reservationID)
		}},
		{"remove loyalty", "DELETE", "/internal/v1/loyalty/Test%20Max/reservations/" + reservationID, "", 204, nil, func(base string, h *http.Client) error {
			return clients.NewLoyaltyHTTPClient(base, h).RemoveReservation(context.Background(), "Test Max", reservationID)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tc.method || r.URL.EscapedPath() != tc.path {
					t.Errorf("request=%s %s", r.Method, r.URL.EscapedPath())
				}
				if tc.body != nil {
					var body map[string]any
					if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
						t.Error(e)
					}
					for key, want := range tc.body {
						if body[key] != want {
							t.Errorf("body[%s]=%v, want %v", key, body[key], want)
						}
					}
					if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
						t.Error("missing JSON content type")
					}
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			if err := tc.invoke(server.URL, server.Client()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLoyaltyClientEscapesUsernamesAsOnePathSegment(t *testing.T) {
	user := "Test Max/Иван"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/internal/v1/loyalty/"+url.PathEscape(user) {
			t.Errorf("path=%s", r.URL.EscapedPath())
		}
		fmt.Fprint(w, `{"status":"BRONZE","discount":5,"reservationCount":0}`)
	}))
	defer server.Close()
	got, err := clients.NewLoyaltyHTTPClient(server.URL, server.Client()).Get(context.Background(), user)
	if err != nil || got.Discount != 5 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestPaymentReaderMapsResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/internal/v1/payments/"+paymentID {
			t.Errorf("request=%s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"status":"CANCELED","price":27000}`)
	}))
	defer server.Close()
	got, err := clients.NewPaymentHTTPClient(server.URL, server.Client()).Get(context.Background(), paymentID)
	if err != nil || got.Status != "CANCELED" || got.Price != 27000 {
		t.Fatalf("result=%+v, err=%v", got, err)
	}
}

func TestClientsDistinguishMissingBookingFromBrokenDependencies(t *testing.T) {
	for _, tc := range []struct {
		status   int
		body     string
		payment  bool
		notFound bool
	}{
		{404, `{"message":"missing"}`, false, true}, {404, `{"message":"missing"}`, true, false},
		{500, `secret`, false, false}, {200, `{`, false, false}, {200, `null`, false, false},
		{200, strings.Replace(reservationJSON, "2021-10-08", "2021-02-30", 1), false, false},
		{200, reservationJSON + ` {}`, false, false}, {200, strings.Repeat(" ", 1024*1024+1) + reservationJSON, false, false},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
		var err error
		if tc.payment {
			_, err = clients.NewPaymentHTTPClient(server.URL, server.Client()).Get(context.Background(), paymentID)
		} else {
			_, err = clients.NewReservationHTTPClient(server.URL, server.Client()).GetByUser(context.Background(), "Test Max", reservationID)
		}
		server.Close()
		if err == nil || errors.Is(err, app.ErrNotFound) != tc.notFound {
			t.Fatalf("status=%d, notFound=%v, err=%v", tc.status, tc.notFound, err)
		}
		if !tc.notFound {
			var dependency *app.DependencyError
			if !errors.As(err, &dependency) {
				t.Fatalf("not a dependency error: %v", err)
			}
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientsAttachRequestContextAndPreserveCancellation(t *testing.T) {
	type key struct{}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), key{}, "request"))
	defer cancel()
	h := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Context().Value(key{}) != "request" {
			t.Fatal("context not attached to outgoing request")
		}
		cancel()
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	_, err := clients.NewPaymentHTTPClient("http://payment", h).Get(ctx, paymentID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
}

func TestWriteClientsRejectIncompleteSuccessfulResponses(t *testing.T) {
	for _, operation := range []string{"payment", "quote"} {
		for _, body := range []string{`null`, `{}`, `{`, `{"status":"PAID"}`, `{"hotelUid":"` + hotelID + `","startDate":"2021-10-08","endDate":"2021-10-11"}`} {
			t.Run(operation+"/"+body, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if operation == "payment" {
						w.WriteHeader(201)
					}
					fmt.Fprint(w, body)
				}))
				defer server.Close()
				var err error
				if operation == "payment" {
					_, err = clients.NewPaymentHTTPClient(server.URL, server.Client()).Create(context.Background(), app.CreatePaymentCommand{PaymentUID: paymentID, Price: 27000})
				} else {
					start := time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC)
					_, err = clients.NewReservationHTTPClient(server.URL, server.Client()).Quote(context.Background(), app.QuoteRequest{HotelUID: hotelID, StartDate: start, EndDate: start.AddDate(0, 0, 3), Discount: 10})
				}
				var dependency *app.DependencyError
				if !errors.As(err, &dependency) {
					t.Fatalf("operation=%s, body=%s, err=%v", operation, body, err)
				}
			})
		}
	}
}

func TestLoyaltyClientPreservesDotUsernamesThroughRouter(t *testing.T) {
	for _, username := range []string{".", ".."} {
		t.Run(username, func(t *testing.T) {
			mux := http.NewServeMux()
			checkUser := func(r *http.Request) {
				if r.PathValue("username") != username {
					t.Errorf("username=%q, want %q", r.PathValue("username"), username)
				}
			}
			mux.HandleFunc("GET /internal/v1/loyalty/{username}", func(w http.ResponseWriter, r *http.Request) {
				checkUser(r)
				fmt.Fprint(w, `{"status":"BRONZE","discount":5,"reservationCount":0}`)
			})
			for _, method := range []string{"PUT", "DELETE"} {
				mux.HandleFunc(method+" /internal/v1/loyalty/{username}/reservations/{uid}", func(w http.ResponseWriter, r *http.Request) {
					checkUser(r)
					if r.PathValue("uid") != reservationID {
						t.Errorf("uid=%q", r.PathValue("uid"))
					}
					w.WriteHeader(204)
				})
			}
			server := httptest.NewServer(mux)
			defer server.Close()
			h := server.Client()
			h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			client := clients.NewLoyaltyHTTPClient(server.URL, h)
			if _, err := client.Get(context.Background(), username); err != nil {
				t.Fatal(err)
			}
			if err := client.AddReservation(context.Background(), username, reservationID); err != nil {
				t.Fatal(err)
			}
			if err := client.RemoveReservation(context.Background(), username, reservationID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
