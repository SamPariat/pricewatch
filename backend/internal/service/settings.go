package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type SettingsService struct {
	Repo domain.Repository
}

func (s *SettingsService) Get(ctx context.Context) (domain.Settings, error) {
	return s.Repo.GetSettings(ctx)
}

func (s *SettingsService) Update(ctx context.Context, in domain.Settings) (domain.Settings, error) {
	if err := s.Repo.UpdateSettings(ctx, in); err != nil {
		return domain.Settings{}, err
	}
	return in, nil
}
