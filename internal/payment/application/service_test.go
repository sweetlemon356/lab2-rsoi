package application_test

import (
	"context"
	"errors"
	"testing"

	a "github.com/sweetlemon356/lab2-rsoi/internal/payment/application"
	d "github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
)

type repository struct {
	create func(context.Context, d.Payment) (d.Payment, error)
	get    func(context.Context, string) (d.Payment, error)
	cancel func(context.Context, string) error
}

func (r repository) Create(ctx context.Context, p d.Payment) (d.Payment, error) {
	return r.create(ctx, p)
}

func (r repository) Get(ctx context.Context, id string) (d.Payment, error) { return r.get(ctx, id) }

func (r repository) Cancel(ctx context.Context, id string) error { return r.cancel(ctx, id) }

func TestCreateValidatesBeforePersistenceAndPassesRequestContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "request")
	s := a.NewService(repository{create: func(got context.Context, p d.Payment) (d.Payment, error) {
		if got.Value(key{}) != "request" || p.UID != "id" || p.Status != "PAID" || p.Price != 27000 {
			t.Fatalf("payment=%+v", p)
		}
		return p, nil
	}})
	if got, err := s.Create(ctx, "id", 27000); err != nil || got.Price != 27000 {
		t.Fatalf("payment=%+v, err=%v", got, err)
	}
	if _, err := a.NewService(repository{}).Create(ctx, "id", -1); !errors.Is(err, d.ErrInvalid) {
		t.Fatalf("err=%v", err)
	}
}

func TestReadCancelAndFailuresPreserveMeaning(t *testing.T) {
	failure := errors.New("database unavailable")
	s := a.NewService(repository{get: func(context.Context, string) (d.Payment, error) { return d.Payment{}, d.ErrNotFound }, cancel: func(_ context.Context, id string) error {
		if id != "id" {
			t.Fatal(id)
		}
		return failure
	}, create: func(context.Context, d.Payment) (d.Payment, error) { return d.Payment{}, d.ErrConflict }})
	if _, err := s.Get(context.Background(), "id"); !errors.Is(err, d.ErrNotFound) {
		t.Fatal(err)
	}
	if err := s.Cancel(context.Background(), "id"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	if _, err := s.Create(context.Background(), "id", 100); !errors.Is(err, d.ErrConflict) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := a.NewService(repository{}).Create(ctx, "id", 10); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
