package httpadapter

import (
	"errors"
	"net/http"

	"github.com/sweetlemon356/lab2-rsoi/internal/loyalty/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
)

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	user, err := httpapi.User(r.PathValue("username"))
	if err != nil {
		return err
	}
	account, err := h.service.Get(r.Context(), user)
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

func (h *Handler) add(w http.ResponseWriter, r *http.Request) error {
	user, id, err := identity(r)
	if err != nil {
		return err
	}
	if err := h.service.Add(r.Context(), user, id); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) error {
	user, id, err := identity(r)
	if err != nil {
		return err
	}
	if err := h.service.Remove(r.Context(), user, id); err != nil {
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
