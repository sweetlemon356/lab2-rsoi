package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
	"github.com/sweetlemon356/lab2-rsoi/internal/reservation/application"
)

type Handler struct {
	service *application.Service
}

func NewHandler(service *application.Service, health func(context.Context) error, timeout time.Duration, logger *slog.Logger) http.Handler {
	h := &Handler{service: service}
	router := httpapi.NewRouter(timeout, logger, classify)
	router.Handle("GET /internal/v1/hotels", h.hotels)
	router.Handle("POST /internal/v1/reservations/quote", h.quote)
	router.Handle("GET /internal/v1/reservations", h.list)
	router.Handle("GET /internal/v1/reservations/{uid}", h.get)
	router.Handle("PUT /internal/v1/reservations/{uid}", h.create)
	router.Handle("PUT /internal/v1/reservations/{uid}/cancellation", h.cancel)
	router.Health(health)
	return router
}
