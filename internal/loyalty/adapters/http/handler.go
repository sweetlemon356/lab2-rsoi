package httpadapter

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/loyalty/application"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
)

type Handler struct {
	service *application.Service
}

func NewHandler(service *application.Service, health func(context.Context) error, timeout time.Duration, logger *slog.Logger) http.Handler {
	h := &Handler{service: service}
	router := httpapi.NewRouter(timeout, logger, classify)
	router.Handle("GET /internal/v1/loyalty/{username}", h.get)
	router.Handle("PUT /internal/v1/loyalty/{username}/reservations/{uid}", h.add)
	router.Handle("DELETE /internal/v1/loyalty/{username}/reservations/{uid}", h.remove)
	router.Health(health)
	return router
}
