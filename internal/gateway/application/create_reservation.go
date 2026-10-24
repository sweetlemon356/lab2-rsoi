package application

import (
	"context"
	"errors"
	"fmt"
)

func (s *Service) CreateReservation(ctx context.Context, command CreateReservationCommand) (CreateReservationResult, error) {
	if err := validateUser(ctx, command.Username); err != nil {
		return CreateReservationResult{}, err
	}
	if err := validateBooking(command); err != nil {
		return CreateReservationResult{}, err
	}
	loyalty, err := s.loyalty.Get(ctx, command.Username)
	if err != nil {
		return CreateReservationResult{}, fmt.Errorf("get booking discount: %w", err)
	}
	if loyalty.Discount < 0 || loyalty.Discount > 100 {
		return CreateReservationResult{}, &DependencyError{Service: "loyalty", Err: errors.New("invalid discount")}
	}
	if err := ctx.Err(); err != nil {
		return CreateReservationResult{}, err
	}
	quote, err := s.reservations.Quote(ctx, QuoteRequest{
		HotelUID:  command.HotelUID,
		StartDate: command.StartDate,
		EndDate:   command.EndDate,
		Discount:  loyalty.Discount,
	})
	if err != nil {
		return CreateReservationResult{}, fmt.Errorf("quote reservation: %w", err)
	}
	if quote.HotelUID != command.HotelUID || !quote.StartDate.Equal(command.StartDate) || !quote.EndDate.Equal(command.EndDate) || quote.Price < 0 {
		return CreateReservationResult{}, &DependencyError{Service: "reservation", Err: errors.New("quote does not match booking")}
	}
	if err := ctx.Err(); err != nil {
		return CreateReservationResult{}, err
	}
	reservationUID, err := s.newID()
	if err != nil {
		return CreateReservationResult{}, fmt.Errorf("generate reservation ID: %w", err)
	}
	paymentUID, err := s.newID()
	if err != nil {
		return CreateReservationResult{}, fmt.Errorf("generate payment ID: %w", err)
	}
	if reservationUID == "" || paymentUID == "" {
		return CreateReservationResult{}, errors.New("ID generator returned an empty ID")
	}

	// Register each undo before its write: a timeout does not prove that the write failed.
	undo := []func(context.Context) error{
		func(cleanup context.Context) error { return s.payments.Cancel(cleanup, paymentUID) },
	}
	payment, err := s.payments.Create(ctx, CreatePaymentCommand{PaymentUID: paymentUID, Price: quote.Price})
	if err == nil && (payment.Status != "PAID" || payment.Price != quote.Price) {
		err = &DependencyError{Service: "payment", Err: errors.New("created payment does not match quote")}
	}
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return CreateReservationResult{}, s.compensate(ctx, fmt.Errorf("create payment: %w", err), undo)
	}

	undo = append(undo, func(cleanup context.Context) error {
		return s.reservations.Cancel(cleanup, command.Username, reservationUID)
	})
	err = s.reservations.Create(ctx, SaveReservationCommand{
		ReservationUID: reservationUID,
		Username:       command.Username,
		PaymentUID:     paymentUID,
		HotelUID:       command.HotelUID,
		StartDate:      command.StartDate,
		EndDate:        command.EndDate,
	})
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return CreateReservationResult{}, s.compensate(ctx, fmt.Errorf("save reservation: %w", err), undo)
	}

	undo = append(undo, func(cleanup context.Context) error {
		return s.loyalty.RemoveReservation(cleanup, command.Username, reservationUID)
	})
	err = s.loyalty.AddReservation(ctx, command.Username, reservationUID)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return CreateReservationResult{}, s.compensate(ctx, fmt.Errorf("update loyalty: %w", err), undo)
	}

	return CreateReservationResult{
		ReservationUID: reservationUID,
		HotelUID:       command.HotelUID,
		StartDate:      command.StartDate,
		EndDate:        command.EndDate,
		Discount:       loyalty.Discount,
		Status:         "PAID",
		Payment:        payment,
	}, nil
}

func (s *Service) compensate(ctx context.Context, cause error, undo []func(context.Context) error) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cleanupTimeout)
	defer cancel()
	result := cause
	for i := len(undo) - 1; i >= 0; i-- {
		if err := undo[i](cleanup); err != nil {
			result = errors.Join(result, fmt.Errorf("booking compensation: %w", err))
		}
	}
	return result
}

func validateBooking(command CreateReservationCommand) error {
	var fields []FieldError
	if command.HotelUID == "" {
		fields = append(fields, FieldError{Field: "hotelUid", Error: "is required"})
	}
	if command.StartDate.IsZero() || command.StartDate.Year() < 1 || command.StartDate.Year() > 9999 {
		fields = append(fields, FieldError{Field: "startDate", Error: "is required"})
	}
	if command.EndDate.IsZero() || command.EndDate.Year() < 1 || command.EndDate.Year() > 9999 || !command.EndDate.After(command.StartDate) {
		fields = append(fields, FieldError{Field: "endDate", Error: "must follow startDate"})
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}
