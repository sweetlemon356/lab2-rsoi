package domain

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid  = errors.New("invalid loyalty operation")
	ErrConflict = errors.New("booking belongs to another loyalty account")
	ErrCanceled = errors.New("loyalty booking is permanently canceled")
)

type Account struct {
	Username         string
	ReservationCount int
	Status           string
	Discount         int
}

type BookingState string

const (
	Absent   BookingState = ""
	Active   BookingState = "ACTIVE"
	Canceled BookingState = "CANCELED"
)

type Action string

const (
	AddBooking    Action = "ADD"
	CancelBooking Action = "CANCEL"
)

func NewAccount(username string, count int) (Account, error) {
	if strings.TrimSpace(username) == "" || utf8.RuneCountInString(username) > 80 || count < 0 || count > math.MaxInt32 {
		return Account{}, ErrInvalid
	}
	account := Account{Username: username, ReservationCount: count, Status: "BRONZE", Discount: 5}
	if count >= 20 {
		account.Status = "GOLD"
		account.Discount = 10
	} else if count >= 10 {
		account.Status = "SILVER"
		account.Discount = 7
	}
	return account, nil
}

func (a Account) WithDelta(delta int) (Account, error) {
	if delta < -1 || delta > 1 {
		return Account{}, ErrInvalid
	}
	return NewAccount(a.Username, a.ReservationCount+delta)
}

func Transition(state BookingState, action Action) (BookingState, int, error) {
	if state != Absent && state != Active && state != Canceled {
		return state, 0, ErrInvalid
	}
	switch action {
	case AddBooking:
		if state == Canceled {
			return Canceled, 0, ErrCanceled
		}
		if state == Active {
			return Active, 0, nil
		}
		return Active, 1, nil
	case CancelBooking:
		if state == Active {
			return Canceled, -1, nil
		}
		return Canceled, 0, nil
	default:
		return state, 0, ErrInvalid
	}
}
