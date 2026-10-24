package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/sweetlemon356/lab2-rsoi/internal/gateway/application"
)

type endpoint func(http.ResponseWriter, *http.Request) error

func NewRouter(hotels *HotelController, reservations *ReservationController, loyalty *LoyaltyController, profile *ProfileController, timeout time.Duration, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	registerSwagger(mux)
	handle := func(pattern string, handler endpoint) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if err := handler(w, r); err != nil {
				logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "error", err)
				if !w.(*responseWriter).written {
					writeError(w, err)
				}
			}
		})
	}
	handle("GET /api/v1/hotels", hotels.List)
	handle("GET /api/v1/me", profile.GetMe)
	handle("GET /api/v1/loyalty", loyalty.Get)
	handle("GET /api/v1/reservations", reservations.List)
	handle("GET /api/v1/reservations/{reservationUid}", reservations.Get)
	handle("POST /api/v1/reservations", reservations.Create)
	handle("DELETE /api/v1/reservations/{reservationUid}", reservations.Cancel)
	handle("GET /manage/health", func(w http.ResponseWriter, _ *http.Request) error {
		return writeJSON(w, 200, struct {
			Status string `json:"status"`
		}{"UP"})
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writer := &responseWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in request handler", "panic", recovered)
				if !writer.written {
					writeError(writer, errors.New("handler panic"))
				}
			}
		}()
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		mux.ServeHTTP(writer, r.WithContext(ctx))
	})
}

type responseWriter struct {
	http.ResponseWriter
	written bool
}

func (w *responseWriter) WriteHeader(status int) {
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}
func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(append(data, '\n'))
	return err
}

type fieldErrorResponse struct {
	Field string `json:"field"`
	Error string `json:"error"`
}
type errorResponse struct {
	Message string               `json:"message"`
	Errors  []fieldErrorResponse `json:"errors,omitempty"`
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	response := errorResponse{Message: "internal server error"}
	var validation *application.ValidationError
	var dependency *application.DependencyError
	switch {
	case errors.As(err, &validation):
		status = 400
		response.Message = "invalid request"
		for _, field := range validation.Fields {
			response.Errors = append(response.Errors, fieldErrorResponse{Field: field.Field, Error: field.Error})
		}
	case errors.Is(err, context.DeadlineExceeded):
		status = 504
		response.Message = "request deadline exceeded"
	case errors.Is(err, context.Canceled):
		status = 408
		response.Message = "request canceled"
	case errors.Is(err, application.ErrNotFound):
		status = 404
		response.Message = "resource not found"
	case errors.As(err, &dependency):
		status = 502
		response.Message = "dependent service unavailable"
	}
	_ = writeJSON(w, status, response)
}
