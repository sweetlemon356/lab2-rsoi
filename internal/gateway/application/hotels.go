package application

import (
	"context"
	"fmt"
	"math"
)

func (s *Service) ListHotels(ctx context.Context, query PageQuery) (HotelPage, error) {
	if err := ctx.Err(); err != nil {
		return HotelPage{}, err
	}
	if query.Page < 0 || query.Size < 1 || query.Size > 100 || query.Page > math.MaxInt/query.Size {
		return HotelPage{}, &ValidationError{Fields: []FieldError{{Field: "pagination", Error: "page must be nonnegative and size must be between 1 and 100"}}}
	}
	page, err := s.reservations.ListHotels(ctx, query)
	if err != nil {
		return HotelPage{}, fmt.Errorf("list hotels: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return HotelPage{}, err
	}
	if page.Items == nil {
		page.Items = []Hotel{}
	}
	return page, nil
}
