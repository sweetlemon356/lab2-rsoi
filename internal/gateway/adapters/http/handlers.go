package httpadapter

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

func (c *HotelController) List(w http.ResponseWriter, r *http.Request) error {
	page, err := queryInteger(r, "page", 1, 0, int(^uint(0)>>1))
	if err != nil {
		return err
	}
	size, err := queryInteger(r, "size", 10, 1, 100)
	if err != nil {
		return err
	}
	if page > math.MaxInt/size {
		return invalidField("page", "pagination offset is too large")
	}
	result, err := c.hotels.ListHotels(r.Context(), application.PageQuery{Page: page, Size: size})
	if err != nil {
		return err
	}
	items := make([]hotelResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, hotelResponse{
			HotelUID: item.HotelUID, Name: item.Name,
			Country: item.Country, City: item.City, Address: item.Address,
			Stars: item.Stars, Price: item.Price,
		})
	}
	return writeJSON(w, 200, hotelPageResponse{Page: result.Page, PageSize: result.PageSize, TotalElements: result.TotalElements, Items: items})
}

func (c *ProfileController) GetMe(w http.ResponseWriter, r *http.Request) error {
	username, err := requestUser(r)
	if err != nil {
		return err
	}
	result, err := c.userInfo.GetUserInfo(r.Context(), username)
	if err != nil {
		return err
	}
	return writeJSON(w, 200, userInfoResponse{Reservations: reservationResponses(result.Reservations), Loyalty: loyaltyJSON(result.Loyalty)})
}

func (c *LoyaltyController) Get(w http.ResponseWriter, r *http.Request) error {
	username, err := requestUser(r)
	if err != nil {
		return err
	}
	result, err := c.loyalty.GetLoyalty(r.Context(), username)
	if err != nil {
		return err
	}
	return writeJSON(w, 200, loyaltyJSON(result))
}

func (c *ReservationController) List(w http.ResponseWriter, r *http.Request) error {
	username, err := requestUser(r)
	if err != nil {
		return err
	}
	result, err := c.reservations.ListReservations(r.Context(), username)
	if err != nil {
		return err
	}
	return writeJSON(w, 200, reservationResponses(result))
}

func (c *ReservationController) Get(w http.ResponseWriter, r *http.Request) error {
	username, reservationUID, err := reservationIdentity(r)
	if err != nil {
		return err
	}
	result, err := c.reservations.GetReservation(r.Context(), username, reservationUID)
	if err != nil {
		return err
	}
	return writeJSON(w, 200, reservationJSON(result))
}

func (c *ReservationController) Cancel(w http.ResponseWriter, r *http.Request) error {
	username, reservationUID, err := reservationIdentity(r)
	if err != nil {
		return err
	}
	if err := c.reservations.CancelReservation(r.Context(), username, reservationUID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (c *ReservationController) Create(w http.ResponseWriter, r *http.Request) error {
	username, err := requestUser(r)
	if err != nil {
		return err
	}
	var request *createReservationRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(&request); err != nil || request == nil {
		return invalidField("body", "must contain a JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return invalidField("body", "must contain exactly one JSON object within the size limit")
	}
	hotelUID, err := requestUUID(request.HotelUID, "hotelUid")
	if err != nil {
		return err
	}
	start, err := time.Parse(time.DateOnly, request.StartDate)
	if err != nil || start.Year() < 1 {
		return invalidField("startDate", "must be a valid YYYY-MM-DD date")
	}
	end, err := time.Parse(time.DateOnly, request.EndDate)
	if err != nil || end.Year() < 1 {
		return invalidField("endDate", "must be a valid YYYY-MM-DD date")
	}
	command := application.CreateReservationCommand{
		Username: username, HotelUID: hotelUID, StartDate: start, EndDate: end,
	}
	result, err := c.reservations.CreateReservation(r.Context(), command)
	if err != nil {
		return err
	}
	return writeJSON(w, 200, createReservationResponse{
		ReservationUID: result.ReservationUID,
		HotelUID:       result.HotelUID,
		StartDate:      result.StartDate.Format(time.DateOnly),
		EndDate:        result.EndDate.Format(time.DateOnly),
		Discount:       result.Discount,
		Status:         result.Status,
		Payment:        paymentResponse{Status: result.Payment.Status, Price: result.Payment.Price},
	})
}

func requestUser(r *http.Request) (string, error) {
	username := strings.TrimSpace(r.Header.Get("X-User-Name"))
	if username == "" {
		return "", invalidField("X-User-Name", "is required")
	}
	if !utf8.ValidString(username) || utf8.RuneCountInString(username) > 80 {
		return "", invalidField("X-User-Name", "must contain valid UTF-8 and at most 80 characters")
	}
	return username, nil
}

func reservationIdentity(r *http.Request) (string, string, error) {
	username, err := requestUser(r)
	if err != nil {
		return "", "", err
	}
	reservationUID, err := requestUUID(r.PathValue("reservationUid"), "reservationUid")
	return username, reservationUID, err
}

func requestUUID(value, field string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", invalidField(field, "must be a valid UUID")
	}
	return id.String(), nil
}

func queryInteger(r *http.Request, key string, fallback, min, max int) (int, error) {
	raw, exists := r.URL.Query()[key]
	if !exists {
		return fallback, nil
	}
	if len(raw) != 1 {
		return 0, invalidField(key, "must occur once")
	}
	value, err := strconv.Atoi(raw[0])
	if err != nil || value < min || value > max {
		return 0, invalidField(key, "is outside the allowed integer range")
	}
	return value, nil
}

func invalidField(field, message string) error {
	return &application.ValidationError{Fields: []application.FieldError{{Field: field, Error: message}}}
}
