package application_test

import (
	"context"
	"errors"
	"testing"

	a "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/application"
	d "github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
)

type repository struct {
	get   func(context.Context, string) (d.Account, error)
	apply func(context.Context, string, string, d.Action) (d.Account, error)
}

func (r repository) Get(ctx context.Context, u string) (d.Account, error) { return r.get(ctx, u) }

func (r repository) ApplyBooking(ctx context.Context, u, id string, action d.Action) (d.Account, error) {
	return r.apply(ctx, u, id, action)
}

func TestLoyaltyChangesAreKeyedByUserAndBooking(t *testing.T) {
	var actions []d.Action
	s := a.NewService(repository{get: func(_ context.Context, u string) (d.Account, error) { return d.NewAccount(u, 0) }, apply: func(_ context.Context, u, id string, action d.Action) (d.Account, error) {
		if u != "user" || id != "booking" {
			t.Fatalf("user=%s,id=%s", u, id)
		}
		actions = append(actions, action)
		return d.NewAccount(u, 1)
	}})
	if got, err := s.Get(context.Background(), "user"); err != nil || got.Discount != 5 {
		t.Fatalf("account=%+v, err=%v", got, err)
	}
	if err := s.Add(context.Background(), "user", "booking"); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(context.Background(), "user", "booking"); err != nil {
		t.Fatal(err)
	}
	if len(actions) != 2 || actions[0] != d.AddBooking || actions[1] != d.CancelBooking {
		t.Fatalf("actions=%v", actions)
	}
}

func TestLoyaltyRejectsInvalidIdentityAndPreservesRepositoryErrors(t *testing.T) {
	s := a.NewService(repository{})
	if _, err := s.Get(context.Background(), " "); !errors.Is(err, d.ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), "user", ""); !errors.Is(err, d.ErrInvalid) {
		t.Fatal(err)
	}
	failure := errors.New("db failure")
	s = a.NewService(repository{apply: func(context.Context, string, string, d.Action) (d.Account, error) { return d.Account{}, failure }})
	if err := s.Remove(context.Background(), "user", "booking"); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := a.NewService(repository{}).Remove(ctx, "user", "booking"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
