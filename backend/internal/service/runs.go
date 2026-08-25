package service

import (
	"context"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type RunsService struct {
	Repo domain.Repository
}

func (s *RunsService) List(ctx context.Context, limit int) ([]domain.DigestRun, error) {
	return s.Repo.ListRecentDigestRuns(ctx, limit)
}

func (s *RunsService) Events(ctx context.Context, runID domain.RunID) ([]domain.RunEvent, error) {
	return s.Repo.ListRunEvents(ctx, runID)
}
