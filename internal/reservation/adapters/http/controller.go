package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/domain"
)

type Catalog interface {
	ListHotels(context.Context, application.PageQuery) (application.HotelPage, error)
	Quote(context.Context, string, time.Time, time.Time, int) (application.Quote, error)
}

type Bookings interface {
	ListReservations(context.Context, string) ([]application.Details, error)
	GetReservation(context.Context, string, string) (application.Details, error)
	Create(context.Context, application.CreateCommand) error
	Cancel(context.Context, string, string) error
}

type Controller struct {
	catalog  Catalog
	bookings Bookings
}

func NewHandler(catalog Catalog, bookings Bookings, health func(context.Context) error, timeout time.Duration, logger *slog.Logger) http.Handler {
	c := &Controller{catalog: catalog, bookings: bookings}
	router := httpapi.NewRouter(timeout, logger, classify)
	router.Handle("GET /internal/v1/hotels", c.hotels)
	router.Handle("POST /internal/v1/reservations/quote", c.quote)
	router.Handle("GET /internal/v1/reservations", c.list)
	router.Handle("GET /internal/v1/reservations/{uid}", c.get)
	router.Handle("PUT /internal/v1/reservations/{uid}", c.create)
	router.Handle("PUT /internal/v1/reservations/{uid}/cancellation", c.cancel)
	router.Health(health)
	return router
}

func (c *Controller) hotels(w http.ResponseWriter, r *http.Request) error {
	page, err := httpapi.Integer(r, "page", 1, 0, math.MaxInt)
	if err != nil {
		return err
	}
	size, err := httpapi.Integer(r, "size", 10, 1, 100)
	if err != nil {
		return err
	}
	result, err := c.catalog.ListHotels(r.Context(), application.PageQuery{Page: page, Size: size})
	if err != nil {
		return err
	}
	items := make([]hotelResponse, 0, len(result.Items))
	for _, hotel := range result.Items {
		items = append(items, hotelResponse{UID: hotel.UID, Name: hotel.Name, Country: hotel.Country, City: hotel.City, Address: hotel.Address, Stars: hotel.Stars, Price: hotel.Price})
	}
	return httpapi.JSON(w, 200, struct {
		Page  int             `json:"page"`
		Size  int             `json:"pageSize"`
		Total int             `json:"totalElements"`
		Items []hotelResponse `json:"items"`
	}{result.Page, result.PageSize, result.TotalElements, items})
}

func (c *Controller) quote(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		HotelUID  string `json:"hotelUid"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Discount  *int   `json:"discount"`
	}
	if err := httpapi.Decode(w, r, &body); err != nil {
		return err
	}
	if body.Discount == nil {
		return httpapi.BadRequest("discount is required")
	}
	hotel, err := httpapi.UUID(body.HotelUID)
	if err != nil {
		return err
	}
	start, err := httpapi.Date(body.StartDate)
	if err != nil {
		return err
	}
	end, err := httpapi.Date(body.EndDate)
	if err != nil {
		return err
	}
	result, err := c.catalog.Quote(r.Context(), hotel, start, end, *body.Discount)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 200, struct {
		HotelUID  string `json:"hotelUid"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Price     int64  `json:"price"`
	}{result.HotelUID, result.Stay.StartDate.Format(time.DateOnly), result.Stay.EndDate.Format(time.DateOnly), result.Price})
}

func (c *Controller) list(w http.ResponseWriter, r *http.Request) error {
	user, err := httpapi.User(r.Header.Get("X-User-Name"))
	if err != nil {
		return err
	}
	rows, err := c.bookings.ListReservations(r.Context(), user)
	if err != nil {
		return err
	}
	results := make([]reservationResponse, 0, len(rows))
	for _, row := range rows {
		results = append(results, reservationJSON(row))
	}
	return httpapi.JSON(w, 200, results)
}

func (c *Controller) get(w http.ResponseWriter, r *http.Request) error {
	user, err := httpapi.User(r.Header.Get("X-User-Name"))
	if err != nil {
		return err
	}
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	result, err := c.bookings.GetReservation(r.Context(), user, id)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 200, reservationJSON(result))
}

func (c *Controller) create(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	var body struct {
		Username   string `json:"username"`
		PaymentUID string `json:"paymentUid"`
		HotelUID   string `json:"hotelUid"`
		StartDate  string `json:"startDate"`
		EndDate    string `json:"endDate"`
	}
	if err := httpapi.Decode(w, r, &body); err != nil {
		return err
	}
	user, err := httpapi.User(body.Username)
	if err != nil {
		return err
	}
	payment, err := httpapi.UUID(body.PaymentUID)
	if err != nil {
		return err
	}
	hotel, err := httpapi.UUID(body.HotelUID)
	if err != nil {
		return err
	}
	start, err := httpapi.Date(body.StartDate)
	if err != nil {
		return err
	}
	end, err := httpapi.Date(body.EndDate)
	if err != nil {
		return err
	}
	if err := c.bookings.Create(r.Context(), application.CreateCommand{UID: id, Username: user, PaymentUID: payment, HotelUID: hotel, StartDate: start, EndDate: end}); err != nil {
		return err
	}
	w.WriteHeader(201)
	return nil
}

func (c *Controller) cancel(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	var body struct {
		Username string `json:"username"`
	}
	if err := httpapi.Decode(w, r, &body); err != nil {
		return err
	}
	user, err := httpapi.User(body.Username)
	if err != nil {
		return err
	}
	if err := c.bookings.Cancel(r.Context(), user, id); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		return 400, "invalid reservation"
	case errors.Is(err, domain.ErrNotFound):
		return 404, "hotel or reservation not found"
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrCanceled):
		return 409, "reservation conflicts with existing operation"
	default:
		return 500, "internal server error"
	}
}
