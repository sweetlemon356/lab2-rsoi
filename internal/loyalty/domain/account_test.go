package domain_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	d "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
)

func TestLoyaltyThresholds(t *testing.T) {
	for _, tc := range []struct {
		count    int
		status   string
		discount int
	}{{0, "BRONZE", 5}, {9, "BRONZE", 5}, {10, "SILVER", 7}, {19, "SILVER", 7}, {20, "GOLD", 10}, {25, "GOLD", 10}} {
		a, err := d.NewAccount("Test Max", tc.count)
		if err != nil || a.Username != "Test Max" || a.ReservationCount != tc.count || a.Status != tc.status || a.Discount != tc.discount {
			t.Fatalf("account=%+v, err=%v", a, err)
		}
	}
}

func TestLoyaltyPromotionAndDemotion(t *testing.T) {
	for _, tc := range []struct {
		count, delta, wantCount int
		status                  string
		discount                int
	}{{9, 1, 10, "SILVER", 7}, {19, 1, 20, "GOLD", 10}, {20, -1, 19, "SILVER", 7}, {10, -1, 9, "BRONZE", 5}, {1, -1, 0, "BRONZE", 5}} {
		a, _ := d.NewAccount("user", tc.count)
		got, err := a.WithDelta(tc.delta)
		if err != nil || got.ReservationCount != tc.wantCount || got.Status != tc.status || got.Discount != tc.discount || a.ReservationCount != tc.count {
			t.Fatalf("account=%+v, err=%v", got, err)
		}
	}
}

func TestLoyaltyValidation(t *testing.T) {
	for _, tc := range []struct {
		user  string
		count int
	}{{"", 0}, {" ", 0}, {strings.Repeat("я", 81), 0}, {"user", -1}} {
		if _, err := d.NewAccount(tc.user, tc.count); !errors.Is(err, d.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	}
	if _, err := d.NewAccount(strings.Repeat("я", 80), 0); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ count, delta int }{{0, -1}, {math.MaxInt32, 1}, {0, 2}} {
		a, _ := d.NewAccount("user", tc.count)
		if _, err := a.WithDelta(tc.delta); !errors.Is(err, d.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	}
}

func TestBookingLedgerTransitionsAreIdempotentAndTerminal(t *testing.T) {
	for _, tc := range []struct {
		state    d.BookingState
		action   d.Action
		want     d.BookingState
		delta    int
		canceled bool
	}{
		{d.Absent, d.AddBooking, d.Active, 1, false}, {d.Active, d.AddBooking, d.Active, 0, false},
		{d.Active, d.CancelBooking, d.Canceled, -1, false}, {d.Absent, d.CancelBooking, d.Canceled, 0, false},
		{d.Canceled, d.CancelBooking, d.Canceled, 0, false}, {d.Canceled, d.AddBooking, d.Canceled, 0, true},
	} {
		state, delta, err := d.Transition(tc.state, tc.action)
		if !tc.canceled && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state != tc.want || delta != tc.delta || errors.Is(err, d.ErrCanceled) != tc.canceled {
			t.Fatalf("%s/%s => %s,%d,%v", tc.state, tc.action, state, delta, err)
		}
	}
	if _, _, err := d.Transition(d.Active, "UNKNOWN"); !errors.Is(err, d.ErrInvalid) {
		t.Fatal("unknown action accepted")
	}
}
