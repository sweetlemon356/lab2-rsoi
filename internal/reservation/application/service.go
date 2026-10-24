package application

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

type PageQuery struct{ Page, Size int }

type HotelPage struct {
	Page, PageSize, TotalElements int
	Items                         []domain.Hotel
}

type Details struct {
	Reservation domain.Reservation
	Hotel       domain.Hotel
}

type Quote struct {
	HotelUID string
	Stay     domain.Stay
	Price    int64
}

type CreateCommand struct {
	UID, Username, PaymentUID, HotelUID string
	StartDate, EndDate                  time.Time
}

type HotelRepository interface {
	List(context.Context, PageQuery) (HotelPage, error)
	GetHotel(context.Context, string) (domain.Hotel, error)
}

type ReservationRepository interface {
	ListByUser(context.Context, string) ([]Details, error)
	GetByUser(context.Context, string, string) (Details, error)
	Create(context.Context, domain.Reservation) error
	Cancel(context.Context, string, string) error
}

type Service struct {
	hotels       HotelRepository
	reservations ReservationRepository
}

func NewService(hotels HotelRepository, reservations ReservationRepository) *Service {
	return &Service{hotels: hotels, reservations: reservations}
}

func (s *Service) ListHotels(ctx context.Context, q PageQuery) (HotelPage, error) {
	if err := ctx.Err(); err != nil {
		return HotelPage{}, err
	}
	if q.Page < 0 || q.Size < 1 || q.Size > 100 || q.Page > math.MaxInt/q.Size {
		return HotelPage{}, domain.ErrInvalid
	}
	page, err := s.hotels.List(ctx, q)
	if err != nil {
		return HotelPage{}, err
	}
	if page.Items == nil {
		page.Items = []domain.Hotel{}
	}
	return page, nil
}

func (s *Service) Quote(ctx context.Context, id string, start, end time.Time, discount int) (Quote, error) {
	if err := ctx.Err(); err != nil {
		return Quote{}, err
	}
	stay, err := domain.NewStay(start, end)
	if err != nil {
		return Quote{}, err
	}
	hotel, err := s.hotels.GetHotel(ctx, id)
	if err != nil {
		return Quote{}, err
	}
	price, err := stay.Price(hotel.Price, discount)
	if err != nil {
		return Quote{}, err
	}
	return Quote{HotelUID: hotel.UID, Stay: stay, Price: price}, nil
}

func (s *Service) ListReservations(ctx context.Context, user string) ([]Details, error) {
	if err := validUser(ctx, user); err != nil {
		return nil, err
	}
	rows, err := s.reservations.ListByUser(ctx, user)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []Details{}
	}
	for _, row := range rows {
		if row.Reservation.Username != user {
			return nil, domain.ErrNotFound
		}
	}
	return rows, nil
}

func (s *Service) GetReservation(ctx context.Context, user, id string) (Details, error) {
	if err := validUser(ctx, user); err != nil {
		return Details{}, err
	}
	row, err := s.reservations.GetByUser(ctx, user, id)
	if err != nil {
		return Details{}, err
	}
	if row.Reservation.Username != user || row.Reservation.UID != id {
		return Details{}, domain.ErrNotFound
	}
	return row, nil
}

func (s *Service) Create(ctx context.Context, c CreateCommand) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	stay, err := domain.NewStay(c.StartDate, c.EndDate)
	if err != nil {
		return err
	}
	candidate, err := domain.NewReservation(c.UID, c.Username, c.PaymentUID, c.HotelUID, stay)
	if err != nil {
		return err
	}
	if _, err := s.hotels.GetHotel(ctx, c.HotelUID); err != nil {
		return err
	}
	return s.reservations.Create(ctx, candidate)
}

func (s *Service) Cancel(ctx context.Context, user, id string) error {
	if err := validUser(ctx, user); err != nil {
		return err
	}
	if id == "" {
		return domain.ErrInvalid
	}
	return s.reservations.Cancel(ctx, user, id)
}

func validUser(ctx context.Context, user string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(user) == "" || utf8.RuneCountInString(user) > 80 {
		return domain.ErrInvalid
	}
	return nil
}
