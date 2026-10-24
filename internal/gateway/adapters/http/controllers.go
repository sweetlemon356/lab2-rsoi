package httpadapter

import (
	"context"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type Hotels interface {
	ListHotels(context.Context, application.PageQuery) (application.HotelPage, error)
}

type Reservations interface {
	ListReservations(context.Context, string) ([]application.ReservationView, error)
	GetReservation(context.Context, string, string) (application.ReservationView, error)
	CreateReservation(context.Context, application.CreateReservationCommand) (application.CreateReservationResult, error)
	CancelReservation(context.Context, string, string) error
}

type Loyalty interface {
	GetLoyalty(context.Context, string) (application.LoyaltyInfo, error)
}

type UserInfo interface {
	GetUserInfo(context.Context, string) (application.UserInfoView, error)
}

type HotelController struct{ hotels Hotels }

func NewHotelController(hotels Hotels) *HotelController {
	return &HotelController{hotels: hotels}
}

type ReservationController struct{ reservations Reservations }

func NewReservationController(reservations Reservations) *ReservationController {
	return &ReservationController{reservations: reservations}
}

type LoyaltyController struct{ loyalty Loyalty }

func NewLoyaltyController(loyalty Loyalty) *LoyaltyController {
	return &LoyaltyController{loyalty: loyalty}
}

type ProfileController struct{ userInfo UserInfo }

func NewProfileController(userInfo UserInfo) *ProfileController {
	return &ProfileController{userInfo: userInfo}
}
