package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	a "github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
	d "github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

type hotels struct {
	get  func(context.Context, string) (d.Hotel, error)
	list func(context.Context, a.PageQuery) (a.HotelPage, error)
}

func (r hotels) GetHotel(ctx context.Context, id string) (d.Hotel, error) { return r.get(ctx, id) }

func (r hotels) List(ctx context.Context, q a.PageQuery) (a.HotelPage, error) { return r.list(ctx, q) }

type reservations struct {
	create func(context.Context, d.Reservation) error
	get    func(context.Context, string, string) (a.Details, error)
	list   func(context.Context, string) ([]a.Details, error)
	cancel func(context.Context, string, string) error
}

func (r reservations) Create(ctx context.Context, v d.Reservation) error { return r.create(ctx, v) }

func (r reservations) GetByUser(ctx context.Context, u, id string) (a.Details, error) {
	return r.get(ctx, u, id)
}

func (r reservations) ListByUser(ctx context.Context, u string) ([]a.Details, error) {
	return r.list(ctx, u)
}

func (r reservations) Cancel(ctx context.Context, u, id string) error { return r.cancel(ctx, u, id) }

func dates() (time.Time, time.Time) {
	return time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC), time.Date(2021, 10, 11, 0, 0, 0, 0, time.UTC)
}

func TestQuoteUsesHotelPriceAndDomainRules(t *testing.T) {
	start, end := dates()
	s := a.NewService(hotels{get: func(_ context.Context, id string) (d.Hotel, error) {
		if id != "hotel" {
			t.Fatal(id)
		}
		return d.Hotel{UID: id, Price: 10000}, nil
	}}, reservations{})
	got, err := s.Quote(context.Background(), "hotel", start, end, 10)
	if err != nil || got.Price != 27000 || got.HotelUID != "hotel" || got.Stay.Nights() != 3 {
		t.Fatalf("quote=%+v,err=%v", got, err)
	}
	if _, err := a.NewService(hotels{}, reservations{}).Quote(context.Background(), "hotel", end, start, 10); !errors.Is(err, d.ErrInvalid) {
		t.Fatal(err)
	}
	s = a.NewService(hotels{get: func(context.Context, string) (d.Hotel, error) { return d.Hotel{}, d.ErrNotFound }}, reservations{})
	if _, err := s.Quote(context.Background(), "missing", start, end, 10); !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestCreateValidatesStayAndHotelBeforeSaving(t *testing.T) {
	start, end := dates()
	saved := false
	s := a.NewService(hotels{get: func(context.Context, string) (d.Hotel, error) { return d.Hotel{UID: "hotel"}, nil }}, reservations{create: func(_ context.Context, r d.Reservation) error {
		saved = true
		if r.UID != "id" || r.Username != "user" || r.PaymentUID != "payment" || r.Status != "PAID" || r.Stay.Nights() != 3 {
			t.Fatalf("reservation=%+v", r)
		}
		return nil
	}})
	command := a.CreateCommand{UID: "id", Username: "user", PaymentUID: "payment", HotelUID: "hotel", StartDate: start, EndDate: end}
	if err := s.Create(context.Background(), command); err != nil || !saved {
		t.Fatal(err)
	}
	command.EndDate = start
	if err := a.NewService(hotels{}, reservations{}).Create(context.Background(), command); !errors.Is(err, d.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestReservationQueriesEnforceOwnershipAndNormalizeEmptyLists(t *testing.T) {
	foreign := a.Details{Reservation: d.Reservation{UID: "id", Username: "other"}}
	s := a.NewService(hotels{}, reservations{get: func(context.Context, string, string) (a.Details, error) { return foreign, nil }, list: func(context.Context, string) ([]a.Details, error) { return []a.Details{foreign}, nil }})
	if _, err := s.GetReservation(context.Background(), "user", "id"); !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := s.ListReservations(context.Background(), "user"); !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
	s = a.NewService(hotels{}, reservations{list: func(context.Context, string) ([]a.Details, error) { return nil, nil }})
	if got, err := s.ListReservations(context.Background(), "user"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got=%v,err=%v", got, err)
	}
}

func TestCancellationForwardsOwnerAndContext(t *testing.T) {
	failure := errors.New("db failure")
	s := a.NewService(hotels{}, reservations{cancel: func(_ context.Context, u, id string) error {
		if u != "user" || id != "id" {
			t.Fatal("identity lost")
		}
		return failure
	}})
	if err := s.Cancel(context.Background(), "user", "id"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.NewService(hotels{}, reservations{}).Cancel(ctx, "user", "id"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
