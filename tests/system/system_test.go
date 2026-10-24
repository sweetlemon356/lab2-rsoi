package system_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

const hotelUID = "049161bb-badd-4fa8-9d90-87c9a82b0668"

type system struct {
	t      *testing.T
	base   string
	user   string
	client *http.Client
}

func open(t *testing.T) *system {
	t.Helper()
	base := os.Getenv("SYSTEM_TEST_URL")
	if base == "" {
		t.Skip("set SYSTEM_TEST_URL for a running disposable stack")
	}
	return &system{t: t, base: base, user: "system-" + uuid.NewString(), client: &http.Client{Timeout: 10 * time.Second}}
}

func (s *system) request(method, path, user string, body any, status int, target any) {
	s.t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			s.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, s.base+path, bytes.NewReader(data))
	if err != nil {
		s.t.Fatal(err)
	}
	if user != "" {
		req.Header.Set("X-User-Name", user)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := s.client.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer response.Body.Close()
	data, err = io.ReadAll(response.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	if response.StatusCode != status {
		s.t.Fatalf("%s %s: status=%d, want=%d, body=%s", method, path, response.StatusCode, status, data)
	}
	if target != nil {
		if err := json.Unmarshal(data, target); err != nil {
			s.t.Fatalf("decode %s: %v", data, err)
		}
	}
}

type booking struct {
	UID      string `json:"reservationUid"`
	Status   string `json:"status"`
	Discount int    `json:"discount"`
	Payment  struct {
		Status string `json:"status"`
		Price  int64  `json:"price"`
	} `json:"payment"`
}

func (s *system) loyalty(count int, status string, discount int) {
	s.t.Helper()
	var got struct {
		Count    int    `json:"reservationCount"`
		Status   string `json:"status"`
		Discount int    `json:"discount"`
	}
	s.request("GET", "/api/v1/loyalty", s.user, nil, 200, &got)
	if got.Count != count || got.Status != status || got.Discount != discount {
		s.t.Fatalf("loyalty=%+v, want count=%d status=%s discount=%d", got, count, status, discount)
	}
}

func TestBookingOwnershipTierChangesAndRepeatedCancellation(t *testing.T) {
	s := open(t)
	s.loyalty(0, "BRONZE", 5)
	var bookings []booking
	t.Cleanup(func() {
		for _, b := range bookings {
			s.request("DELETE", "/api/v1/reservations/"+b.UID, s.user, nil, 204, nil)
		}
	})
	for i := range 11 {
		var got booking
		s.request("POST", "/api/v1/reservations", s.user, map[string]string{
			"hotelUid": hotelUID, "startDate": "2021-10-08", "endDate": "2021-10-09",
		}, 200, &got)
		bookings = append(bookings, got)
		discount, price := 5, int64(9500)
		if i == 10 {
			discount, price = 7, 9300
		}
		if got.UID == "" || got.Discount != discount || got.Payment.Price != price || got.Status != "PAID" || got.Payment.Status != "PAID" {
			t.Fatalf("booking %d=%+v", i+1, got)
		}
		if i == 9 {
			s.loyalty(10, "SILVER", 7)
		}
	}
	s.loyalty(11, "SILVER", 7)
	path := "/api/v1/reservations/" + bookings[0].UID
	s.request("GET", path, "other-"+s.user, nil, 404, nil)
	s.request("DELETE", path, "other-"+s.user, nil, 404, nil)
	var own booking
	s.request("GET", path, s.user, nil, 200, &own)
	if own.Status != "PAID" || own.Payment.Status != "PAID" {
		t.Fatalf("foreign cancellation changed booking: %+v", own)
	}
	for _, index := range []int{10, 9, 9} {
		s.request("DELETE", "/api/v1/reservations/"+bookings[index].UID, s.user, nil, 204, nil)
	}
	s.loyalty(9, "BRONZE", 5)
	var canceled booking
	s.request("GET", "/api/v1/reservations/"+bookings[9].UID, s.user, nil, 200, &canceled)
	if canceled.Status != "CANCELED" || canceled.Payment.Status != "CANCELED" {
		t.Fatalf("cancellation=%+v", canceled)
	}
	var me struct {
		Reservations []booking `json:"reservations"`
		Loyalty      struct {
			Count int `json:"reservationCount"`
		} `json:"loyalty"`
	}
	s.request("GET", "/api/v1/me", s.user, nil, 200, &me)
	if len(me.Reservations) != 11 || me.Loyalty.Count != 9 {
		t.Fatalf("me=%+v", me)
	}
}

func TestInvalidBookingDoesNotChangeLoyalty(t *testing.T) {
	s := open(t)
	for _, body := range []map[string]string{
		{"hotelUid": hotelUID, "startDate": "2021-10-08", "endDate": "2021-10-08"},
		{"hotelUid": "invalid", "startDate": "2021-10-08", "endDate": "2021-10-09"},
	} {
		s.request("POST", "/api/v1/reservations", s.user, body, 400, nil)
	}
	s.request("POST", "/api/v1/reservations", s.user, map[string]string{
		"hotelUid": uuid.NewString(), "startDate": "2021-10-08", "endDate": "2021-10-09",
	}, 404, nil)
	s.loyalty(0, "BRONZE", 5)
	var rows []booking
	s.request("GET", "/api/v1/reservations", s.user, nil, 200, &rows)
	if rows == nil || len(rows) != 0 {
		t.Fatalf("reservations=%+v", rows)
	}
}
