package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/config"
)

// These HTTP fixtures exercise the real Gateway wiring, not implementations of
// the three backend services. They model the agreed internal contracts.
type upstreamFixtures struct {
	mu           sync.Mutex
	reservations map[string]map[string]any
	payments     map[string]map[string]any
	counted      map[string]bool
	removed      map[string]bool
}

func fixtureServers(t *testing.T) (config.Config, *upstreamFixtures) {
	t.Helper()
	const hotelID = "049161bb-badd-4fa8-9d90-87c9a82b0668"
	f := &upstreamFixtures{reservations: map[string]map[string]any{}, payments: map[string]map[string]any{}, counted: map[string]bool{}, removed: map[string]bool{}}
	jsonResponse := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	decode := func(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
		var value map[string]any
		if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
			t.Errorf("invalid fixture request: %v", err)
			http.Error(w, "invalid JSON", 400)
			return nil, false
		}
		return value, true
	}
	hotel := map[string]any{"hotelUid": hotelID, "name": "Ararat Park Hyatt Moscow", "fullAddress": "Россия, Москва, Неглинная ул., 4", "stars": 5}
	reservationMux := http.NewServeMux()
	reservationMux.HandleFunc("GET /internal/v1/hotels", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"page": 1, "pageSize": 10, "totalElements": 1, "items": []any{map[string]any{"hotelUid": hotelID, "name": "Ararat Park Hyatt Moscow", "country": "Россия", "city": "Москва", "address": "Неглинная ул., 4", "stars": 5, "price": 10000}}})
	})
	reservationMux.HandleFunc("POST /internal/v1/reservations/quote", func(w http.ResponseWriter, r *http.Request) {
		body, ok := decode(w, r)
		if !ok {
			return
		}
		if body["hotelUid"] != hotelID {
			http.NotFound(w, r)
			return
		}
		if body["discount"] != float64(10) {
			t.Errorf("quote discount=%v", body["discount"])
		}
		jsonResponse(w, 200, map[string]any{"hotelUid": hotelID, "startDate": body["startDate"], "endDate": body["endDate"], "price": 27000})
	})
	reservationMux.HandleFunc("PUT /internal/v1/reservations/{uid}", func(w http.ResponseWriter, r *http.Request) {
		body, ok := decode(w, r)
		if !ok {
			return
		}
		id := r.PathValue("uid")
		f.mu.Lock()
		defer f.mu.Unlock()
		body["reservationUid"] = id
		body["hotel"] = hotel
		body["status"] = "PAID"
		f.reservations[id] = body
		w.WriteHeader(201)
	})
	reservationMux.HandleFunc("GET /internal/v1/reservations/{uid}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		row, ok := f.reservations[r.PathValue("uid")]
		if !ok || row["username"] != r.Header.Get("X-User-Name") {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, row)
	})
	reservationMux.HandleFunc("GET /internal/v1/reservations", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		rows := []any{}
		for _, row := range f.reservations {
			if row["username"] == r.Header.Get("X-User-Name") {
				rows = append(rows, row)
			}
		}
		jsonResponse(w, 200, rows)
	})
	reservationMux.HandleFunc("PUT /internal/v1/reservations/{uid}/cancellation", func(w http.ResponseWriter, r *http.Request) {
		body, ok := decode(w, r)
		if !ok {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		row, exists := f.reservations[r.PathValue("uid")]
		if exists && row["username"] != body["username"] {
			http.NotFound(w, r)
			return
		}
		if exists {
			row["status"] = "CANCELED"
		}
		w.WriteHeader(204)
	})
	paymentMux := http.NewServeMux()
	paymentMux.HandleFunc("PUT /internal/v1/payments/{uid}", func(w http.ResponseWriter, r *http.Request) {
		body, ok := decode(w, r)
		if !ok {
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("uid")
		if _, exists := f.payments[id]; !exists {
			f.payments[id] = map[string]any{"status": "PAID", "price": body["price"]}
		}
		jsonResponse(w, 201, f.payments[id])
	})
	paymentMux.HandleFunc("GET /internal/v1/payments/{uid}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		payment, exists := f.payments[r.PathValue("uid")]
		if !exists {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, 200, payment)
	})
	paymentMux.HandleFunc("PUT /internal/v1/payments/{uid}/cancellation", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("uid")
		if f.payments[id] == nil {
			f.payments[id] = map[string]any{"price": 0}
		}
		f.payments[id]["status"] = "CANCELED"
		w.WriteHeader(204)
	})
	loyaltyMux := http.NewServeMux()
	loyaltyMux.HandleFunc("GET /internal/v1/loyalty/{username}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		jsonResponse(w, 200, map[string]any{"status": "GOLD", "discount": 10, "reservationCount": 25 + len(f.counted)})
	})
	loyaltyMux.HandleFunc("PUT /internal/v1/loyalty/{username}/reservations/{uid}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("uid")
		if !f.removed[id] {
			f.counted[id] = true
		}
		w.WriteHeader(204)
	})
	loyaltyMux.HandleFunc("DELETE /internal/v1/loyalty/{username}/reservations/{uid}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		id := r.PathValue("uid")
		delete(f.counted, id)
		f.removed[id] = true
		w.WriteHeader(204)
	})
	reservationServer := httptest.NewServer(reservationMux)
	paymentServer := httptest.NewServer(paymentMux)
	loyaltyServer := httptest.NewServer(loyaltyMux)
	t.Cleanup(reservationServer.Close)
	t.Cleanup(paymentServer.Close)
	t.Cleanup(loyaltyServer.Close)
	return config.Config{ReservationURL: reservationServer.URL, PaymentURL: paymentServer.URL, LoyaltyURL: loyaltyServer.URL, RequestTimeout: time.Second, ClientTimeout: time.Second, CleanupTimeout: time.Second}, f
}

func TestGatewayBookingLifecycleThroughRealHTTPClients(t *testing.T) {
	cfg, state := fixtureServers(t)
	client := &http.Client{Timeout: cfg.ClientTimeout}
	t.Cleanup(client.CloseIdleConnections)
	handler := buildHandler(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), client)
	call := func(method, path, body, user string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if user != "" {
			r.Header.Set("X-User-Name", user)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: status=%d, want=%d, body=%s", method, path, w.Code, status, w.Body.String())
		}
		return w
	}
	call("GET", "/manage/health", "", "", 200)
	call("GET", "/api/v1/hotels?page=1&size=10", "", "", 200)
	call("GET", "/api/v1/loyalty", "", "Test Max", 200)
	created := call("POST", "/api/v1/reservations", `{"hotelUid":"049161bb-badd-4fa8-9d90-87c9a82b0668","startDate":"2021-10-08","endDate":"2021-10-11"}`, "Test Max", 200)
	var booking struct {
		ReservationUID string `json:"reservationUid"`
		Discount       int    `json:"discount"`
		Payment        struct {
			Status string `json:"status"`
			Price  int64  `json:"price"`
		} `json:"payment"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &booking); err != nil {
		t.Fatal(err)
	}
	if _, err := uuid.Parse(booking.ReservationUID); err != nil || booking.Discount != 10 || booking.Payment.Price != 27000 || booking.Payment.Status != "PAID" {
		t.Fatalf("booking=%+v, err=%v", booking, err)
	}
	path := "/api/v1/reservations/" + booking.ReservationUID
	call("GET", path, "", "Test Max", 200)
	call("GET", path, "", "Another user", 404)
	call("DELETE", path, "", "Another user", 404)
	call("GET", "/api/v1/reservations", "", "Test Max", 200)
	profile := call("GET", "/api/v1/me", "", "Test Max", 200)
	if !strings.Contains(profile.Body.String(), `"reservationCount":26`) || !strings.Contains(profile.Body.String(), `"fullAddress":"Россия, Москва, Неглинная ул., 4"`) {
		t.Fatalf("profile=%s", profile.Body.String())
	}
	for range 2 {
		call("DELETE", path, "", "Test Max", 204)
	}
	canceled := call("GET", path, "", "Test Max", 200)
	if strings.Count(canceled.Body.String(), `"status":"CANCELED"`) != 2 {
		t.Fatalf("canceled=%s", canceled.Body.String())
	}
	loyalty := call("GET", "/api/v1/loyalty", "", "Test Max", 200)
	if !strings.Contains(loyalty.Body.String(), `"reservationCount":25`) {
		t.Fatalf("loyalty=%s", loyalty.Body.String())
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if len(state.payments) != 1 || len(state.reservations) != 1 || len(state.counted) != 0 {
		t.Fatal(fmt.Sprintf("unexpected state: payments=%d, reservations=%d, counted=%d", len(state.payments), len(state.reservations), len(state.counted)))
	}
}
