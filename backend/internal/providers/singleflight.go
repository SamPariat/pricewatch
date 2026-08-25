package providers

import (
	"context"

	"golang.org/x/sync/singleflight"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type singleflightProvider struct {
	next  domain.Provider
	group singleflight.Group
}

// WithSingleflight collapses concurrent Fetch calls for the same watch into
// one upstream request — the direct answer to "someone keeps refreshing":
// 50 simultaneous requests for one watch on a cold cache produce one
// provider call, and the other 49 wait on its result.
func WithSingleflight(next domain.Provider) domain.Provider {
	return &singleflightProvider{next: next}
}

func (s *singleflightProvider) Kind() domain.AssetKind { return s.next.Kind() }

func (s *singleflightProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	v, err, _ := s.group.Do(string(w.ID), func() (any, error) {
		return s.next.Fetch(ctx, w)
	})
	if err != nil {
		return nil, err
	}
	return v.([]domain.Quote), nil
}
