package httpadapter

import (
	"errors"
	"net/http"

	"github.com/sweetlemon356/lab2-rsoi/internal/payment/domain"
	"github.com/sweetlemon356/lab2-rsoi/internal/platform/httpapi"
)

type paymentResponse struct {
	Status string `json:"status"`
	Price  int64  `json:"price"`
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	payment, err := h.service.Get(r.Context(), id)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 200, paymentResponse{Status: payment.Status, Price: payment.Price})
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) error {
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
	payment, err := h.service.Create(r.Context(), id, *body.Price)
	if err != nil {
		return err
	}
	return httpapi.JSON(w, 201, paymentResponse{Status: payment.Status, Price: payment.Price})
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) error {
	id, err := httpapi.UUID(r.PathValue("uid"))
	if err != nil {
		return err
	}
	if err := h.service.Cancel(r.Context(), id); err != nil {
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
