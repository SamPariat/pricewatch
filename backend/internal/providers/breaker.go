package providers

import (
	"context"

	"github.com/sony/gobreaker/v2"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type breakerProvider struct {
	next domain.Provider
	cb   *gobreaker.CircuitBreaker[[]domain.Quote]
}

// WithBreaker fails fast once a provider has been failing consistently,
// instead of letting every watch for that provider individually wait out a
// full retry chain against a service that's already down — the point made
// in PLAN.md: one dead upstream must not stall the whole digest run.
func WithBreaker(next domain.Provider) domain.Provider {
	settings := gobreaker.Settings{
		Name: string(next.Kind()),
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
	}
	return &breakerProvider{
		next: next,
		cb:   gobreaker.NewCircuitBreaker[[]domain.Quote](settings),
	}
}

func (b *breakerProvider) Kind() domain.AssetKind { return b.next.Kind() }

func (b *breakerProvider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return b.cb.Execute(func() ([]domain.Quote, error) {
		return b.next.Fetch(ctx, w)
	})
}
