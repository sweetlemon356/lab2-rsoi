package clients

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type ReservationHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewReservationHTTPClient(baseURL string, client *http.Client) *ReservationHTTPClient {
	return &ReservationHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: client}
}

func (c *ReservationHTTPClient) ListHotels(ctx context.Context, query application.PageQuery) (application.HotelPage, error) {
	var response struct {
		Page          *int       `json:"page"`
		PageSize      *int       `json:"pageSize"`
		TotalElements *int       `json:"totalElements"`
		Items         []hotelDTO `json:"items"`
	}
	address := fmt.Sprintf("%s/internal/v1/hotels?page=%d&size=%d", c.baseURL, query.Page, query.Size)
	if err := do(ctx, c.httpClient, "reservation", http.MethodGet, address, "", nil, &response, false, 200); err != nil {
		return application.HotelPage{}, err
	}
	if response.Page == nil || response.PageSize == nil || response.TotalElements == nil || *response.Page < 0 || *response.PageSize < 1 || *response.PageSize > 100 || *response.TotalElements < 0 {
		return application.HotelPage{}, dependencyError("reservation", errors.New("incomplete hotel pagination"))
	}
	items := make([]application.Hotel, 0, len(response.Items))
	for _, item := range response.Items {
		if !validID(item.HotelUID) || item.Name == "" || item.Country == "" || item.City == "" || item.Address == "" || item.Price == nil || *item.Price < 0 {
			return application.HotelPage{}, dependencyError("reservation", errors.New("incomplete hotel response"))
		}
		items = append(items, application.Hotel{HotelUID: item.HotelUID, Name: item.Name, Country: item.Country, City: item.City, Address: item.Address, Stars: item.Stars, Price: *item.Price})
	}
	return application.HotelPage{Page: *response.Page, PageSize: *response.PageSize, TotalElements: *response.TotalElements, Items: items}, nil
}

func (c *ReservationHTTPClient) ListByUser(ctx context.Context, username string) ([]application.ReservationData, error) {
	var response []reservationDTO
	if err := do(ctx, c.httpClient, "reservation", http.MethodGet, c.baseURL+"/internal/v1/reservations", username, nil, &response, false, 200); err != nil {
		return nil, err
	}
	rows := make([]application.ReservationData, 0, len(response))
	for _, item := range response {
		row, err := item.model()
		if err != nil {
			return nil, dependencyError("reservation", err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (c *ReservationHTTPClient) GetByUser(ctx context.Context, username, reservationUID string) (application.ReservationData, error) {
	var response reservationDTO
	if err := do(ctx, c.httpClient, "reservation", http.MethodGet, c.baseURL+"/internal/v1/reservations/"+url.PathEscape(reservationUID), username, nil, &response, true, 200); err != nil {
		return application.ReservationData{}, err
	}
	row, err := response.model()
	if err != nil {
		return application.ReservationData{}, dependencyError("reservation", err)
	}
	return row, nil
}

func (c *ReservationHTTPClient) Quote(ctx context.Context, query application.QuoteRequest) (application.ReservationQuote, error) {
	body := struct {
		HotelUID  string `json:"hotelUid"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Discount  int    `json:"discount"`
	}{query.HotelUID, query.StartDate.Format(time.DateOnly), query.EndDate.Format(time.DateOnly), query.Discount}
	var response struct {
		HotelUID  string `json:"hotelUid"`
		StartDate string `json:"startDate"`
		EndDate   string `json:"endDate"`
		Price     *int64 `json:"price"`
	}
	if err := do(ctx, c.httpClient, "reservation", http.MethodPost, c.baseURL+"/internal/v1/reservations/quote", "", body, &response, true, 200); err != nil {
		return application.ReservationQuote{}, err
	}
	start, startErr := time.Parse(time.DateOnly, response.StartDate)
	end, endErr := time.Parse(time.DateOnly, response.EndDate)
	if startErr != nil || endErr != nil || !end.After(start) || !validID(response.HotelUID) || response.Price == nil || *response.Price < 0 {
		return application.ReservationQuote{}, dependencyError("reservation", errors.New("incomplete quote response"))
	}
	return application.ReservationQuote{HotelUID: response.HotelUID, StartDate: start, EndDate: end, Price: *response.Price}, nil
}

func (c *ReservationHTTPClient) Create(ctx context.Context, command application.SaveReservationCommand) error {
	body := struct {
		Username   string `json:"username"`
		PaymentUID string `json:"paymentUid"`
		HotelUID   string `json:"hotelUid"`
		StartDate  string `json:"startDate"`
		EndDate    string `json:"endDate"`
	}{command.Username, command.PaymentUID, command.HotelUID, command.StartDate.Format(time.DateOnly), command.EndDate.Format(time.DateOnly)}
	return do(ctx, c.httpClient, "reservation", http.MethodPut, c.baseURL+"/internal/v1/reservations/"+url.PathEscape(command.ReservationUID), "", body, nil, false, 200, 201)
}

func (c *ReservationHTTPClient) Cancel(ctx context.Context, username, reservationUID string) error {
	body := struct {
		Username string `json:"username"`
	}{username}
	return do(ctx, c.httpClient, "reservation", http.MethodPut, c.baseURL+"/internal/v1/reservations/"+url.PathEscape(reservationUID)+"/cancellation", "", body, nil, false, 200, 204)
}
