// Package httpapi contains transport helpers shared by the three internal APIs.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

func BadRequest(message string) error { return &Error{Status: 400, Message: message} }

type Endpoint func(http.ResponseWriter, *http.Request) error
type Router struct {
	mux      *http.ServeMux
	timeout  time.Duration
	logger   *slog.Logger
	classify func(error) (int, string)
}

func NewRouter(timeout time.Duration, logger *slog.Logger, classify func(error) (int, string)) *Router {
	return &Router{mux: http.NewServeMux(), timeout: timeout, logger: logger, classify: classify}
}

func (s *Router) Handle(pattern string, endpoint Endpoint) {
	s.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if err := endpoint(w, r); err != nil {
			s.fail(w, r, err)
		}
	})
}

func (s *Router) Health(check func(context.Context) error) {
	s.Handle("GET /manage/health", func(w http.ResponseWriter, r *http.Request) error {
		if err := check(r.Context()); err != nil {
			return &Error{Status: 503, Message: "database unavailable"}
		}
		return JSON(w, 200, struct {
			Status string `json:"status"`
		}{"UP"})
	})
}

func (s *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tracked := &responseWriter{ResponseWriter: w}
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	defer func() {
		if value := recover(); value != nil {
			s.logger.Error("handler panic", "panic", value)
			if !tracked.written {
				_ = JSON(tracked, 500, map[string]string{"message": "internal server error"})
			}
		}
	}()
	s.mux.ServeHTTP(tracked, r.WithContext(ctx))
}

func (s *Router) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, message := s.classify(err)
	var transport *Error
	switch {
	case errors.As(err, &transport):
		status, message = transport.Status, transport.Message
	case errors.Is(err, context.DeadlineExceeded):
		status, message = 504, "request deadline exceeded"
	case errors.Is(err, context.Canceled):
		status, message = 408, "request canceled"
	}
	if status >= 500 {
		s.logger.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	}
	if !w.(*responseWriter).written {
		_ = JSON(w, status, struct {
			Message string `json:"message"`
		}{message})
	}
}

type responseWriter struct {
	http.ResponseWriter
	written bool
}

func (w *responseWriter) WriteHeader(status int) {
	w.written = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if !w.written {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(body)
}

func JSON(w http.ResponseWriter, status int, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(append(body, '\n'))
	return err
}

func Decode(w http.ResponseWriter, r *http.Request, target any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return BadRequest("invalid or oversized JSON body")
	}
	if bytes.Equal(bytes.TrimSpace(body), []byte("null")) {
		return BadRequest("JSON object is required")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(target); err != nil {
		return BadRequest("invalid JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return BadRequest("exactly one JSON object is required")
	}
	return nil
}

func UUID(value string) (string, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return "", BadRequest("invalid UUID")
	}
	return id.String(), nil
}

func User(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 80 {
		return "", BadRequest("username must contain 1 to 80 characters")
	}
	return value, nil
}

func Date(value string) (time.Time, error) {
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		return time.Time{}, BadRequest("date must use YYYY-MM-DD")
	}
	return date, nil
}

func Integer(r *http.Request, key string, fallback, min, max int) (int, error) {
	values, exists := r.URL.Query()[key]
	if !exists {
		return fallback, nil
	}
	if len(values) != 1 {
		return 0, BadRequest("parameter must occur once")
	}
	value, err := strconv.Atoi(values[0])
	if err != nil || value < min || value > max {
		return 0, BadRequest("invalid integer parameter")
	}
	return value, nil
}
