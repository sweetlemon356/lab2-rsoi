package clients

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type LoyaltyHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewLoyaltyHTTPClient(baseURL string, client *http.Client) *LoyaltyHTTPClient {
	return &LoyaltyHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: client}
}

func (c *LoyaltyHTTPClient) Get(ctx context.Context, username string) (application.LoyaltyInfo, error) {
	var response struct {
		Status           string `json:"status"`
		Discount         *int   `json:"discount"`
		ReservationCount *int   `json:"reservationCount"`
	}
	address := c.baseURL + "/internal/v1/loyalty/" + usernameSegment(username)
	if err := do(ctx, c.httpClient, "loyalty", http.MethodGet, address, "", nil, &response, false, 200); err != nil {
		return application.LoyaltyInfo{}, err
	}
	if (response.Status != "BRONZE" && response.Status != "SILVER" && response.Status != "GOLD") || response.Discount == nil || *response.Discount < 0 || *response.Discount > 100 || response.ReservationCount == nil || *response.ReservationCount < 0 {
		return application.LoyaltyInfo{}, dependencyError("loyalty", errors.New("incomplete loyalty response"))
	}
	return application.LoyaltyInfo{Status: response.Status, Discount: *response.Discount, ReservationCount: *response.ReservationCount}, nil
}

func (c *LoyaltyHTTPClient) AddReservation(ctx context.Context, username, reservationUID string) error {
	return c.change(ctx, http.MethodPut, username, reservationUID)
}

func (c *LoyaltyHTTPClient) RemoveReservation(ctx context.Context, username, reservationUID string) error {
	return c.change(ctx, http.MethodDelete, username, reservationUID)
}

func (c *LoyaltyHTTPClient) change(ctx context.Context, method, username, reservationUID string) error {
	address := c.baseURL + "/internal/v1/loyalty/" + usernameSegment(username) + "/reservations/" + url.PathEscape(reservationUID)
	return do(ctx, c.httpClient, "loyalty", method, address, "", nil, nil, false, 200, 204)
}

func usernameSegment(username string) string {
	// PathEscape preserves dot-only segments, which routers otherwise normalize.
	switch username {
	case ".":
		return "%2E"
	case "..":
		return "%2E%2E"
	default:
		return url.PathEscape(username)
	}
}
