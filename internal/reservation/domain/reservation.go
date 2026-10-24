package domain

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid reservation")
	ErrNotFound = errors.New("hotel or reservation not found")
	ErrConflict = errors.New("reservation identity already has another payload")
	ErrCanceled = errors.New("reservation is permanently canceled")
)

type Stay struct{ StartDate, EndDate time.Time }

func NewStay(start, end time.Time) (Stay, error) {
	if start.IsZero() || end.IsZero() || start.Year() < 1 || end.Year() > 9999 {
		return Stay{}, ErrInvalid
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	if !end.After(start) {
		return Stay{}, ErrInvalid
	}
	return Stay{StartDate: start, EndDate: end}, nil
}

func (s Stay) Nights() int64 { return s.EndDate.Unix()/86400 - s.StartDate.Unix()/86400 }

func (s Stay) Price(nightlyPrice int64, discount int) (int64, error) {
	nights := s.Nights()
	if nights <= 0 || nightlyPrice < 0 || discount < 0 || discount > 100 || nightlyPrice > math.MaxInt64/nights {
		return 0, ErrInvalid
	}
	gross := nightlyPrice * nights
	rate := int64(100 - discount)
	// Split before multiplication to keep intermediate amounts within int64.
	return (gross/100)*rate + ((gross%100)*rate+50)/100, nil
}

type Hotel struct {
	UID, Name, Country, City, Address string
	Stars                             int
	Price                             int64
}

func (h Hotel) FullAddress() string {
	return strings.Join([]string{h.Country, h.City, h.Address}, ", ")
}

type Reservation struct {
	UID, Username, PaymentUID, HotelUID string
	Stay                                Stay
	Status                              string
}

func NewReservation(uid, username, paymentUID, hotelUID string, stay Stay) (Reservation, error) {
	if uid == "" || strings.TrimSpace(username) == "" || utf8.RuneCountInString(username) > 80 || paymentUID == "" || hotelUID == "" {
		return Reservation{}, ErrInvalid
	}
	valid, err := NewStay(stay.StartDate, stay.EndDate)
	if err != nil {
		return Reservation{}, err
	}
	return Reservation{UID: uid, Username: username, PaymentUID: paymentUID, HotelUID: hotelUID, Stay: valid, Status: "PAID"}, nil
}

func (r Reservation) AcceptReplay(candidate Reservation) error {
	if r.Status == "CANCELED" {
		return ErrCanceled
	}
	if r.UID != candidate.UID || r.Username != candidate.Username || r.PaymentUID != candidate.PaymentUID || r.HotelUID != candidate.HotelUID || !r.Stay.StartDate.Equal(candidate.Stay.StartDate) || !r.Stay.EndDate.Equal(candidate.Stay.EndDate) {
		return ErrConflict
	}
	return nil
}

func (r Reservation) Cancel() Reservation { r.Status = "CANCELED"; return r }
