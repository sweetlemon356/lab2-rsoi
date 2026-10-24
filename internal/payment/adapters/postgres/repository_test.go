package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	p "github.com/sweetlemon356/lab2-rsoi/internal/payment/adapters/postgres"
	d "github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/postgrestest"
)

func TestPaymentRepositoryIdempotencyConflictAndTerminalCancellation(t *testing.T) {
	pool := postgrestest.Open(t, "PAYMENT_TEST_DATABASE_URL")
	ctx := context.Background()
	if _, err := pool.Exec(ctx, "TRUNCATE payment, payment_cancellations"); err != nil {
		t.Fatal(err)
	}
	r := p.NewRepository(pool)
	candidate, _ := d.NewPayment(uuid.NewString(), 27000)
	for range 2 {
		got, err := r.Create(ctx, candidate)
		if err != nil || got != candidate {
			t.Fatalf("payment=%+v,err=%v", got, err)
		}
	}
	conflict := candidate
	conflict.Price++
	if _, err := r.Create(ctx, conflict); !errors.Is(err, d.ErrConflict) {
		t.Fatalf("conflict=%v", err)
	}
	for range 2 {
		if err := r.Cancel(ctx, candidate.UID); err != nil {
			t.Fatal(err)
		}
	}
	got, err := r.Get(ctx, candidate.UID)
	if err != nil || got.Status != "CANCELED" || got.Price != 27000 {
		t.Fatalf("payment=%+v,err=%v", got, err)
	}
	if _, err := r.Create(ctx, candidate); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("late create=%v", err)
	}
	unknown := uuid.NewString()
	if err := r.Cancel(ctx, unknown); err != nil {
		t.Fatal(err)
	}
	late, _ := d.NewPayment(unknown, 100)
	if _, err := r.Create(ctx, late); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("cancel-before-create=%v", err)
	}
	if _, err := r.Get(ctx, unknown); !errors.Is(err, d.ErrNotFound) {
		t.Fatalf("unknown payment=%v", err)
	}
}

func TestPaymentRepositoryConcurrentDuplicatesAndCancelRace(t *testing.T) {
	pool := postgrestest.Open(t, "PAYMENT_TEST_DATABASE_URL")
	r := p.NewRepository(pool)
	ctx := context.Background()
	candidate, _ := d.NewPayment(uuid.NewString(), 100)
	var wg sync.WaitGroup
	results := make(chan error, 24)
	for range 24 {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := r.Create(ctx, candidate); results <- err }()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM payment WHERE payment_uid=$1", candidate.UID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("count=%d,err=%v", count, err)
	}
	tracing, _ := d.NewPayment(uuid.NewString(), 100)
	results = make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, err := r.Create(ctx, tracing); results <- err }()
	go func() { defer wg.Done(); results <- r.Cancel(ctx, tracing.UID) }()
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil && !errors.Is(err, d.ErrCanceled) {
			t.Fatal(err)
		}
	}
	if _, err := r.Create(ctx, tracing); !errors.Is(err, d.ErrCanceled) {
		t.Fatalf("resurrection=%v", err)
	}
	if value, err := r.Get(ctx, tracing.UID); err == nil && value.Status != "CANCELED" {
		t.Fatalf("still active=%+v", value)
	} else if err != nil && !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
}
