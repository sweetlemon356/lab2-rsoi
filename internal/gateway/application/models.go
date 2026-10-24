package application

import "time"

type PageQuery struct{ Page, Size int }

type Hotel struct {
	HotelUID, Name, Country, City, Address string
	Stars                                  int
	Price                                  int64
}

type HotelPage struct {
	Page, PageSize, TotalElements int
	Items                         []Hotel
}

type HotelInfo struct {
	HotelUID, Name, FullAddress string
	Stars                       int
}

type PaymentInfo struct {
	Status string
	Price  int64
}

type LoyaltyInfo struct {
	Status                     string
	Discount, ReservationCount int
}

type ReservationData struct {
	ReservationUID, Username, PaymentUID string
	Hotel                                HotelInfo
	StartDate, EndDate                   time.Time
	Status                               string
}

type ReservationView struct {
	ReservationUID     string
	Hotel              HotelInfo
	StartDate, EndDate time.Time
	Status             string
	Payment            PaymentInfo
}

type UserInfoView struct {
	Reservations []ReservationView
	Loyalty      LoyaltyInfo
}

type CreateReservationCommand struct {
	Username, HotelUID string
	StartDate, EndDate time.Time
}

type QuoteRequest struct {
	HotelUID           string
	StartDate, EndDate time.Time
	Discount           int
}

type ReservationQuote struct {
	HotelUID           string
	StartDate, EndDate time.Time
	Price              int64
}

type SaveReservationCommand struct {
	ReservationUID, Username, PaymentUID, HotelUID string
	StartDate, EndDate                             time.Time
}

type CreatePaymentCommand struct {
	PaymentUID string
	Price      int64
}

type CreateReservationResult struct {
	ReservationUID, HotelUID string
	StartDate, EndDate       time.Time
	Discount                 int
	Status                   string
	Payment                  PaymentInfo
}
