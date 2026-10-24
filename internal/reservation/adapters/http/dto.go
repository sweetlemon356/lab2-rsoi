package httpadapter

import (
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
)

type hotelResponse struct {
	UID     string `json:"hotelUid"`
	Name    string `json:"name"`
	Country string `json:"country"`
	City    string `json:"city"`
	Address string `json:"address"`
	Stars   int    `json:"stars"`
	Price   int64  `json:"price"`
}

type hotelInfo struct {
	UID         string `json:"hotelUid"`
	Name        string `json:"name"`
	FullAddress string `json:"fullAddress"`
	Stars       int    `json:"stars"`
}

type reservationResponse struct {
	UID        string    `json:"reservationUid"`
	Username   string    `json:"username"`
	PaymentUID string    `json:"paymentUid"`
	Hotel      hotelInfo `json:"hotel"`
	StartDate  string    `json:"startDate"`
	EndDate    string    `json:"endDate"`
	Status     string    `json:"status"`
}

func reservationJSON(value application.Details) reservationResponse {
	reservation, hotel := value.Reservation, value.Hotel
	return reservationResponse{
		UID: reservation.UID, Username: reservation.Username, PaymentUID: reservation.PaymentUID,
		Hotel:     hotelInfo{UID: hotel.UID, Name: hotel.Name, FullAddress: hotel.FullAddress(), Stars: hotel.Stars},
		StartDate: reservation.Stay.StartDate.Format(time.DateOnly), EndDate: reservation.Stay.EndDate.Format(time.DateOnly), Status: reservation.Status,
	}
}
