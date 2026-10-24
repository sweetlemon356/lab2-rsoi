package domain

import (
	"errors"
	"strings"
)

var (
	ErrInvalid  = errors.New("invalid payment")
	ErrNotFound = errors.New("payment not found")
	ErrConflict = errors.New("payment identity already has another payload")
	ErrCanceled = errors.New("payment is permanently canceled")
)

type Payment struct {
	UID    string
	Price  int64
	Status string
}

func NewPayment(uid string, price int64) (Payment, error) {
	if strings.TrimSpace(uid) == "" || price < 0 {
		return Payment{}, ErrInvalid
	}
	return Payment{UID: uid, Price: price, Status: "PAID"}, nil
}

func (p Payment) AcceptReplay(candidate Payment) error {
	if p.Status == "CANCELED" {
		return ErrCanceled
	}
	if p.UID != candidate.UID || p.Price != candidate.Price {
		return ErrConflict
	}
	return nil
}

func (p Payment) Cancel() Payment { p.Status = "CANCELED"; return p }
