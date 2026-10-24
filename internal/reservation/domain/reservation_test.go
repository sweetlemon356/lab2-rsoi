package domain_test

import (
	"errors"
	"math"
	"testing"
	"time"

	d "github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

func date(day string) time.Time {
	v, err := time.Parse(time.DateOnly, day)
	if err != nil {
		panic(err)
	}
	return v
}

func TestStayCountsCalendarNightsAndDiscounts(t *testing.T) {
	stay, err := d.NewStay(date("2021-10-08"), date("2021-10-11"))
	if err != nil || stay.Nights() != 3 {
		t.Fatalf("stay=%+v, err=%v", stay, err)
	}
	for _, tc := range []struct {
		discount int
		want     int64
	}{{0, 30000}, {5, 28500}, {7, 27900}, {10, 27000}, {100, 0}} {
		price, err := stay.Price(10000, tc.discount)
		if err != nil || price != tc.want {
			t.Fatalf("discount=%d, price=%d, err=%v", tc.discount, price, err)
		}
	}
	leap, _ := d.NewStay(date("2024-02-28"), date("2024-03-01"))
	if leap.Nights() != 2 {
		t.Fatalf("leap nights=%d", leap.Nights())
	}
	zone := time.FixedZone("offset", 3*3600)
	local, _ := d.NewStay(time.Date(2021, 10, 8, 18, 0, 0, 0, zone), time.Date(2021, 10, 11, 1, 0, 0, 0, zone))
	if local.Nights() != 3 || !local.StartDate.Equal(date("2021-10-08")) {
		t.Fatalf("local=%+v", local)
	}
}

func TestStayMoneyRoundingAndOverflow(t *testing.T) {
	three, _ := d.NewStay(date("2021-10-08"), date("2021-10-11"))
	if price, err := three.Price(1, 50); err != nil || price != 2 {
		t.Fatalf("total rounding=%d, err=%v", price, err)
	}
	long, _ := d.NewStay(date("2000-01-01"), date("2400-01-01"))
	if long.Nights() != 146097 {
		t.Fatalf("long period=%d", long.Nights())
	}
	one, _ := d.NewStay(date("2021-10-08"), date("2021-10-09"))
	for _, tc := range []struct {
		price    int64
		discount int
		want     int64
	}{{1, 50, 1}, {1, 51, 0}, {math.MaxInt64, 0, math.MaxInt64}, {math.MaxInt64, 50, 4611686018427387904}} {
		got, err := one.Price(tc.price, tc.discount)
		if err != nil || got != tc.want {
			t.Fatalf("price=%d, err=%v, want=%d", got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		price    int64
		discount int
	}{{-1, 0}, {100, -1}, {100, 101}} {
		if _, err := one.Price(tc.price, tc.discount); !errors.Is(err, d.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	}
	two, _ := d.NewStay(date("2021-10-08"), date("2021-10-10"))
	if _, err := two.Price(math.MaxInt64, 0); !errors.Is(err, d.ErrInvalid) {
		t.Fatal("overflow accepted")
	}
}

func TestStayRejectsInvalidPeriods(t *testing.T) {
	for _, tc := range []struct{ start, end time.Time }{{time.Time{}, date("2021-10-11")}, {date("2021-10-08"), date("2021-10-08")}, {date("2021-10-11"), date("2021-10-08")}} {
		if _, err := d.NewStay(tc.start, tc.end); !errors.Is(err, d.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestReservationReplayAndCancellation(t *testing.T) {
	stay, _ := d.NewStay(date("2021-10-08"), date("2021-10-11"))
	r, err := d.NewReservation("id", "user", "payment", "hotel", stay)
	if err != nil || r.Status != "PAID" {
		t.Fatalf("reservation=%+v, err=%v", r, err)
	}
	if err := r.AcceptReplay(r); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*d.Reservation){
		func(v *d.Reservation) { v.UID = "other" }, func(v *d.Reservation) { v.HotelUID = "other" },
		func(v *d.Reservation) { v.Stay.StartDate = v.Stay.StartDate.AddDate(0, 0, 1) },
		func(v *d.Reservation) { v.Stay.EndDate = v.Stay.EndDate.AddDate(0, 0, 1) },
	} {
		candidate := r
		change(&candidate)
		if !errors.Is(r.AcceptReplay(candidate), d.ErrConflict) {
			t.Fatalf("conflict accepted: %+v", candidate)
		}
	}
	other := r
	other.PaymentUID = "another"
	if !errors.Is(r.AcceptReplay(other), d.ErrConflict) {
		t.Fatal("conflicting payment accepted")
	}
	other = r
	other.Username = "another"
	if !errors.Is(r.AcceptReplay(other), d.ErrConflict) {
		t.Fatal("conflicting owner accepted")
	}
	if !errors.Is(r.Cancel().AcceptReplay(r), d.ErrCanceled) {
		t.Fatal("late create revived canceled booking")
	}
	if r.Cancel().Cancel() != r.Cancel() {
		t.Fatal("cancel is not idempotent")
	}
	if _, err := d.NewReservation("id", "", "payment", "hotel", stay); !errors.Is(err, d.ErrInvalid) {
		t.Fatal("blank owner accepted")
	}
	if (d.Hotel{Country: "Россия", City: "Москва", Address: "Неглинная ул., 4"}).FullAddress() != "Россия, Москва, Неглинная ул., 4" {
		t.Fatal("incorrect full address")
	}
}
