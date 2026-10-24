package application

import (
	"context"

	"github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
)

type Repository interface {
	Get(context.Context, string) (domain.Account, error)
	ApplyBooking(context.Context, string, string, domain.Action) (domain.Account, error)
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Get(ctx context.Context, user string) (domain.Account, error) {
	if err := ctx.Err(); err != nil {
		return domain.Account{}, err
	}
	if _, err := domain.NewAccount(user, 0); err != nil {
		return domain.Account{}, err
	}
	return s.repository.Get(ctx, user)
}

func (s *Service) Add(ctx context.Context, user, id string) error {
	return s.change(ctx, user, id, domain.AddBooking)
}

func (s *Service) Remove(ctx context.Context, user, id string) error {
	return s.change(ctx, user, id, domain.CancelBooking)
}

func (s *Service) change(ctx context.Context, user, id string, action domain.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := domain.NewAccount(user, 0); err != nil {
		return err
	}
	if id == "" {
		return domain.ErrInvalid
	}
	_, err := s.repository.ApplyBooking(ctx, user, id, action)
	return err
}
