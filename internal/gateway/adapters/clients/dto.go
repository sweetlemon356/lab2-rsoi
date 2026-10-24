package clients

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type hotelDTO struct {
	HotelUID string `json:"hotelUid"`
	Name     string `json:"name"`
	Country  string `json:"country"`
	City     string `json:"city"`
	Address  string `json:"address"`
	Stars    int    `json:"stars"`
	Price    *int64 `json:"price"`
}

type hotelInfoDTO struct {
	HotelUID    string `json:"hotelUid"`
	Name        string `json:"name"`
	FullAddress string `json:"fullAddress"`
	Stars       int    `json:"stars"`
}

type reservationDTO struct {
	ReservationUID string       `json:"reservationUid"`
	Username       string       `json:"username"`
	PaymentUID     string       `json:"paymentUid"`
	Hotel          hotelInfoDTO `json:"hotel"`
	StartDate      string       `json:"startDate"`
	EndDate        string       `json:"endDate"`
	Status         string       `json:"status"`
}

func (d reservationDTO) model() (application.ReservationData, error) {
	start, err := time.Parse(time.DateOnly, d.StartDate)
	if err != nil {
		return application.ReservationData{}, err
	}
	end, err := time.Parse(time.DateOnly, d.EndDate)
	if err != nil {
		return application.ReservationData{}, err
	}
	if !validID(d.ReservationUID) || !validID(d.PaymentUID) || !validID(d.Hotel.HotelUID) ||
		strings.TrimSpace(d.Username) == "" || !end.After(start) || !validStatus(d.Status) ||
		d.Hotel.Name == "" || d.Hotel.FullAddress == "" {
		return application.ReservationData{}, errors.New("incomplete reservation response")
	}
	return application.ReservationData{
		ReservationUID: d.ReservationUID,
		Username:       d.Username,
		PaymentUID:     d.PaymentUID,
		Hotel: application.HotelInfo{
			HotelUID:    d.Hotel.HotelUID,
			Name:        d.Hotel.Name,
			FullAddress: d.Hotel.FullAddress,
			Stars:       d.Hotel.Stars,
		},
		StartDate: start,
		EndDate:   end,
		Status:    d.Status,
	}, nil
}

type paymentDTO struct {
	Status string `json:"status"`
	Price  *int64 `json:"price"`
}

func (d paymentDTO) model() (application.PaymentInfo, error) {
	if !validStatus(d.Status) || d.Price == nil || *d.Price < 0 {
		return application.PaymentInfo{}, errors.New("incomplete payment response")
	}
	return application.PaymentInfo{Status: d.Status, Price: *d.Price}, nil
}

func validID(value string) bool     { _, err := uuid.Parse(value); return err == nil }
func validStatus(value string) bool { return value == "PAID" || value == "CANCELED" }
