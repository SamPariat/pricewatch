package service

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/SamPariat/pricewatch/internal/domain"
	"github.com/SamPariat/pricewatch/internal/i18n"
	"github.com/SamPariat/pricewatch/internal/logging"
	"github.com/SamPariat/pricewatch/internal/scheduler"
)

// TripService owns every use case around trips — the scheduled entity a
// watch (flight or lodging leg) belongs to. Schedule validation
// (cron_expr/timezone) lives here now, not in WatchService, since a
// watch no longer carries its own schedule.
type TripService struct {
	Repo  domain.Repository
	Sched *scheduler.Scheduler
}

func (s *TripService) List(ctx context.Context) ([]domain.Trip, error) {
	return s.Repo.ListTrips(ctx)
}

func (s *TripService) Get(ctx context.Context, id domain.TripID) (domain.Trip, error) {
	t, err := s.Repo.GetTrip(ctx, id)
	if err != nil {
		return domain.Trip{}, ErrNotFound
	}
	return t, nil
}

func (s *TripService) Create(ctx context.Context, name, cronExpr, timezone string, enabled bool) (domain.Trip, error) {
	t := domain.Trip{Name: name, CronExpr: cronExpr, Timezone: timezone, Enabled: enabled}
	if err := validateTrip(ctx, t); err != nil {
		return domain.Trip{}, ValidationError{err}
	}

	created, err := s.Repo.CreateTrip(ctx, t)
	if err != nil {
		return domain.Trip{}, err
	}
	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after trip create")
	}
	return created, nil
}

func (s *TripService) Update(ctx context.Context, id domain.TripID, name, cronExpr, timezone string, enabled bool) (domain.Trip, error) {
	existing, err := s.Repo.GetTrip(ctx, id)
	if err != nil {
		return domain.Trip{}, ErrNotFound
	}
	updated := existing
	updated.Name, updated.CronExpr, updated.Timezone, updated.Enabled = name, cronExpr, timezone, enabled
	if err := validateTrip(ctx, updated); err != nil {
		return domain.Trip{}, ValidationError{err}
	}

	saved, err := s.Repo.UpdateTrip(ctx, updated)
	if err != nil {
		return domain.Trip{}, err
	}
	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after trip update")
	}
	return saved, nil
}

// Delete cascade-deletes every leg belonging to this trip — see the
// trips/watches.trip_id migration's ON DELETE CASCADE.
func (s *TripService) Delete(ctx context.Context, id domain.TripID) error {
	if err := s.Repo.DeleteTrip(ctx, id); err != nil {
		return ErrNotFound
	}
	if err := s.Sched.Reload(ctx); err != nil {
		logging.From(ctx).Error().Err(err).Msg("reload after trip delete")
	}
	return nil
}

func validateTrip(ctx context.Context, t domain.Trip) error {
	loc := i18n.From(ctx)
	if _, err := cron.ParseStandard(t.CronExpr); err != nil {
		return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_trip_cron", "Error", err.Error()))
	}
	if t.Timezone != "" {
		if _, err := time.LoadLocation(t.Timezone); err != nil {
			return fmt.Errorf("%s", i18n.T(loc, "validation.invalid_trip_timezone", "Error", err.Error()))
		}
	}
	return nil
}
