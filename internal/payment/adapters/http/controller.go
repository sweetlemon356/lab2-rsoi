package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
)

type UseCase interface {
	Get(context.Context, string) (domain.Payment, error)
	Create(context.Context, string, int64) (domain.Payment, error)
	Cancel(context.Context, string) error
}

type Controller struct{ useCase UseCase }

func NewHandler(useCase UseCase, health func(context.Context) error, timeout time.Duration, logger *slog.Logger) http.Handler {
	c := &Controller{useCase: useCase}
	router := httpapi.NewRouter(timeout, logger, classify)
	router.Handle("GET /internal/v1/payments/{uid}", c.get)
	router.Handle("PUT /internal/v1/payments/{uid}", c.create)
	router.Handle("PUT /internal/v1/payments/{uid}/cancellation", c.cancel)
	router.Health(health)
	return router
}

type paymentResponse struct {
	Status string `json:"status"`
	Price  int64  `json:"price"`
}

func (c *Controller) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	payment, err := c.useCase.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 200, paymentResponse{Status: payment.Status, Price: payment.Price})
}

func (c *Controller) create(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	var body struct {
		Price *int64 `json:"price"`
	}
	if err := httpapi.Decode(w, r, &body); err != nil {
		return err
	}
	if body.Price == nil {
		return httpapi.BadRequest("price is required")
	}
	payment, err := c.useCase.Create(r.Context(), id, *body.Price)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 201, paymentResponse{Status: payment.Status, Price: payment.Price})
}

func (c *Controller) cancel(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	if err := c.useCase.Cancel(r.Context(), id); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func classify(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrInvalid):
		return 400, "invalid payment"
	case errors.Is(err, domain.ErrNotFound):
		return 404, "payment not found"
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrCanceled):
		return 409, "payment conflicts with existing operation"
	default:
		return 500, "internal server error"
	}
}
