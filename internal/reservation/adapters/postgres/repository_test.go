package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/postgrestest"
	p "github.com/sweetlemon356/lab2-rsoi/internal/reservation/adapters/postgres"
	a "github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
	d "github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

const hotelID = "049161bb-badd-4fa8-9d90-87c9a82b0668"

func candidate() d.Reservation {
	stay, _ := d.NewStay(time.Date(2021, 10, 8, 0, 0, 0, 0, time.UTC), time.Date(2021, 10, 11, 0, 0, 0, 0, time.UTC))
	r, _ := d.NewReservation(uuid.NewString(), uuid.NewString(), uuid.NewString(), hotelID, stay)
	return r
}

func TestReservationRepositoryOwnershipReplayAndCancellation(t *testing.T) {
	pool := postgrestest.Open(t, "RESERVATION_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	value := candidate()
	for range 2 {
		if err := r.Create(ctx, value); err != nil {
			t.Fatal(err)
		}
	}
	row, err := r.GetByUser(ctx, value.Username, value.UID)
	if err != nil || row.Reservation != value || row.Hotel.Price != 10000 || row.Hotel.FullAddress() != "Россия, Москва, Неглинная ул., 4" {
		t.Fatalf("row=%+v,err=%v", row, err)
	}
	if _, err := r.GetByUser(ctx, "other", value.UID); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("foreign read=%v", err)
	}
	if err := r.Cancel(ctx, "other", value.UID); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("foreign cancel=%v", err)
	}
	conflict := value
	conflict.PaymentUID = uuid.NewString()
	if err := r.Create(ctx, conflict); !errors.Is(err, d.ErrConflict) {
		t.Fatalf("conflict=%v", err)
	}
	for range 2 {
		if err := r.Cancel(ctx, value.Username, value.UID); err != nil {
			t.Fatal(err)
		}
	}
	row, err = r.GetByUser(ctx, value.Username, value.UID)
	if err != nil || row.Reservation.Status != "CANCELED" {
		t.Fatalf("row=%+v,err=%v", row, err)
	}
	if err := r.Create(ctx, value); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("late create=%v", err)
	}
	late := candidate()
	if err := r.Cancel(ctx, late.Username, late.UID); err != nil {
		t.Fatal(err)
	}
	if err := r.Cancel(ctx, "other", late.UID); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("foreign tombstone cancel=%v", err)
	}
	if err := r.Create(ctx, late); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("cancel-before-create=%v", err)
	}
	rows, err := r.ListByUser(ctx, value.Username)
	if err != nil || len(rows) != 1 || rows[0].Reservation.UID != value.UID {
		t.Fatalf("rows=%v,err=%v", rows, err)
	}
}

func TestHotelPaginationAndUnknownHotel(t *testing.T) {
	pool := postgrestest.Open(t, "RESERVATION_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	for _, page := range []int{0, 1, 2} {
		got, err := r.List(ctx, a.PageQuery{Page: page, Size: 1})
		want := 1
		if page == 2 {
			want = 0
		}
		if err != nil || got.Page != page || got.TotalElements != 1 || len(got.Items) != want {
			t.Fatalf("page=%+v,err=%v", got, err)
		}
	}
	if _, err := r.GetHotel(ctx, uuid.NewString()); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("missing=%v", err)
	}
}

func TestReservationConcurrentReplayAndCancelRace(t *testing.T) {
	pool := postgrestest.Open(t, "RESERVATION_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	value := candidate()
	var wg sync.WaitGroup
	results := make(chan error, 16)
	for range 16 {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.Create(ctx, value) }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM reservation WHERE reservation_uid=$1", value.UID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d,err=%v", count, err)
	}
	value = candidate()
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); results <- r.Create(ctx, value) }()
	go func() { defer wg.Done(); results <- r.Cancel(ctx, value.Username, value.UID) }()
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, d.ErrCanceled) {
			t.Fatal(err)
		}
	}
	if err := r.Create(ctx, value); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("resurrection=%v", err)
	}
	if final, err := r.GetByUser(ctx, value.Username, value.UID); err == nil && final.Reservation.Status != "CANCELED" {
		t.Fatalf("still paid: %+v", final)
	} else if err != nil && !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
}
