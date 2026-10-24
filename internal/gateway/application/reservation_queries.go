package application

import (
	"context"
	"fmt"
)

func (s *Service) ListReservations(ctx context.Context, username string) ([]ReservationView, error) {
	return s.listReservationViews(ctx, username)
}

func (s *Service) GetReservation(ctx context.Context, username, reservationUID string) (ReservationView, error) {
	if err := validateUser(ctx, username); err != nil {
		return ReservationView{}, err
	}
	row, err := s.reservations.GetByUser(ctx, username, reservationUID)
	if err != nil {
		return ReservationView{}, fmt.Errorf("get reservation: %w", err)
	}
	if row.Username != username || row.ReservationUID != reservationUID {
		return ReservationView{}, ErrNotFound
	}
	return s.reservationView(ctx, row)
}

func (s *Service) listReservationViews(ctx context.Context, username string) ([]ReservationView, error) {
	if err := validateUser(ctx, username); err != nil {
		return nil, err
	}
	rows, err := s.reservations.ListByUser(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("list reservations: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	views := make([]ReservationView, 0, len(rows))
	for _, row := range rows {
		if row.Username != username {
			return nil, ErrNotFound
		}
		view, err := s.reservationView(ctx, row)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) reservationView(ctx context.Context, row ReservationData) (ReservationView, error) {
	if err := ctx.Err(); err != nil {
		return ReservationView{}, err
	}
	payment, err := s.payments.Get(ctx, row.PaymentUID)
	if err != nil {
		return ReservationView{}, fmt.Errorf("get reservation payment: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return ReservationView{}, err
	}
	return ReservationView{
		ReservationUID: row.ReservationUID,
		Hotel:          row.Hotel,
		StartDate:      row.StartDate,
		EndDate:        row.EndDate,
		Status:         row.Status,
		Payment:        payment,
	}, nil
}
