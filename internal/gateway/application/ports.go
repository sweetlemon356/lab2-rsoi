package application

import "context"

// Writes use caller-assigned IDs and must be idempotent. Cancellation is terminal,
// including when the original write has not arrived yet.
type ReservationPort interface {
	ListHotels(context.Context, PageQuery) (HotelPage, error)
	ListByUser(context.Context, string) ([]ReservationData, error)
	GetByUser(context.Context, string, string) (ReservationData, error)
	Quote(context.Context, QuoteRequest) (ReservationQuote, error)
	Create(context.Context, SaveReservationCommand) error
	Cancel(context.Context, string, string) error
}

type PaymentPort interface {
	Get(context.Context, string) (PaymentInfo, error)
	Create(context.Context, CreatePaymentCommand) (PaymentInfo, error)
	Cancel(context.Context, string) error
}

type LoyaltyPort interface {
	Get(context.Context, string) (LoyaltyInfo, error)
	AddReservation(context.Context, string, string) error
	// RemoveReservation must fence a delayed AddReservation for the same booking.
	RemoveReservation(context.Context, string, string) error
}
