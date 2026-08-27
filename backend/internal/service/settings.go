package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
)

type SettingsService struct {
	Repo domain.Repository
}

func (s *SettingsService) Get(ctx context.Context) (domain.Settings, error) {
	return s.Repo.GetSettings(ctx)
}

// Update persists a full-replace of Settings. Language falls back to
// English on anything i18n doesn't recognize (empty, a typo, a locale
// this app has no catalog for) rather than persisting it verbatim —
// Settings.Language drives the Telegram digest's actual text, so a bad
// value here would silently corrupt every future digest, not just fail
// a form submission.
func (s *SettingsService) Update(ctx context.Context, in domain.Settings) (domain.Settings, error) {
	if !i18n.Valid(in.Language) {
		in.Language = string(i18n.EN)
	}
	if err := s.Repo.UpdateSettings(ctx, in); err != nil {
		return domain.Settings{}, err
	}
	return in, nil
}
