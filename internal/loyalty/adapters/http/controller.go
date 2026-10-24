package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
)

type UseCase interface {
	Get(context.Context, string) (domain.Account, error)
	Add(context.Context, string, string) error
	Remove(context.Context, string, string) error
}

type Controller struct{ useCase UseCase }

func NewHandler(useCase UseCase, health func(context.Context) error, timeout time.Duration, logger *slog.Logger) http.Handler {
	c := &Controller{useCase: useCase}
	router := httpapi.NewRouter(timeout, logger, classify)
	router.Handle("GET /internal/v1/loyalty/{username}", c.get)
	router.Handle("PUT /internal/v1/loyalty/{username}/reservations/{uid}", c.add)
	router.Handle("DELETE /internal/v1/loyalty/{username}/reservations/{uid}", c.remove)
	router.Health(health)
	return router
}

func (c *Controller) get(w http.ResponseWriter, r *http.Request) error {
	user, err := httpapi.User(r.PathValue("username"))
	if err != nil {
		return err
	}
	account, err := c.useCase.Get(r.Context(), user)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 200, struct {
		Status   string `json:"status"`
		Discount int    `json:"discount"`
		Count    int    `json:"reservationCount"`
	}{account.Status, account.Discount, account.ReservationCount})
}

func identity(r *http.Request) (string, string, error) {
	user, err := httpapi.User(r.PathValue("username"))
	if err != nil {
		return "", "", err
	}
	id, err := httpapi.UUID(r.PathValue("uid"))
	return user, id, err
}

func (c *Controller) add(w http.ResponseWriter, r *http.Request) error {
	user, id, err := identity(r)
	if err != nil {
		return err
	}
	if err := c.useCase.Add(r.Context(), user, id); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func (c *Controller) remove(w http.ResponseWriter, r *http.Request) error {
	user, id, err := identity(r)
	if err != nil {
		return err
	}
	if err := c.useCase.Remove(r.Context(), user, id); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		return 400, "invalid loyalty operation"
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrCanceled):
		return 409, "loyalty operation conflicts with existing booking"
	default:
		return 500, "internal server error"
	}
}
