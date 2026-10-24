package application

import (
	"context"
	"fmt"
)

func (s *Service) GetLoyalty(ctx context.Context, username string) (LoyaltyInfo, error) {
	if err := validateUser(ctx, username); err != nil {
		return LoyaltyInfo{}, err
	}
	info, err := s.loyalty.Get(ctx, username)
	if err != nil {
		return LoyaltyInfo{}, fmt.Errorf("get loyalty: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return LoyaltyInfo{}, err
	}
	return info, nil
}

func (s *Service) GetUserInfo(ctx context.Context, username string) (UserInfoView, error) {
	reservations, err := s.listReservationViews(ctx, username)
	if err != nil {
		return UserInfoView{}, err
	}
	loyalty, err := s.loyalty.Get(ctx, username)
	if err != nil {
		return UserInfoView{}, fmt.Errorf("get loyalty: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return UserInfoView{}, err
	}
	return UserInfoView{Reservations: reservations, Loyalty: loyalty}, nil
}
