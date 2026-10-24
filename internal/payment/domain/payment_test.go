package domain_test

import (
	"errors"
	"math"
	"testing"

	d "github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
)

func TestPaymentCreationAndTerminalCancellation(t *testing.T) {
	for _, price := range []int64{0, 27000, math.MaxInt64} {
		p, err := d.NewPayment("payment-id", price)
		if err != nil || p.Price != price || p.UID != "payment-id" || p.Status != "PAID" {
			t.Fatalf("payment=%+v, err=%v", p, err)
		}
		if err := p.AcceptReplay(p); err != nil {
			t.Fatal(err)
		}
		canceled := p.Cancel()
		if canceled.Status != "CANCELED" || canceled.Price != price || canceled.Cancel() != canceled {
			t.Fatalf("canceled=%+v", canceled)
		}
		if !errors.Is(canceled.AcceptReplay(p), d.ErrCanceled) {
			t.Fatal("late create revived canceled payment")
		}
	}
}

func TestPaymentRejectsInvalidValuesAndConflictingReplay(t *testing.T) {
	for _, tc := range []struct {
		uid   string
		price int64
	}{{"", 10}, {"id", -1}} {
		if _, err := d.NewPayment(tc.uid, tc.price); !errors.Is(err, d.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	}
	p, _ := d.NewPayment("id", 100)
	for _, candidate := range []d.Payment{{UID: "id", Price: 200, Status: "PAID"}, {UID: "other", Price: 100, Status: "PAID"}} {
		if !errors.Is(p.AcceptReplay(candidate), d.ErrConflict) {
			t.Fatal("conflicting replay accepted")
		}
	}
}
