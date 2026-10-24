package application

import (
	"context"

	"github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
)

type Repository interface {
	Get(context.Context, string) (domain.Payment, error)
	Create(context.Context, domain.Payment) (domain.Payment, error)
	Cancel(context.Context, string) error
}

type Service struct{ repository Repository }

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Get(ctx context.Context, id string) (domain.Payment, error) {
	if err := ctx.Err(); err != nil {
		return domain.Payment{}, err
	}
	if id == "" {
		return domain.Payment{}, domain.ErrInvalid
	}
	return s.repository.Get(ctx, id)
}

func (s *Service) Create(ctx context.Context, id string, price int64) (domain.Payment, error) {
	if err := ctx.Err(); err != nil {
		return domain.Payment{}, err
	}
	payment, err := domain.NewPayment(id, price)
	if err != nil {
		return domain.Payment{}, err
	}
	return s.repository.Create(ctx, payment)
}

func (s *Service) Cancel(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if id == "" {
		return domain.ErrInvalid
	}
	return s.repository.Cancel(ctx, id)
}
