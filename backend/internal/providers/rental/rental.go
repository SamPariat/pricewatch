// Package rental is a registered stub for AssetRental (Airbnb-style
// vacation rentals). Airbnb has no official public API — the real options
// are an unofficial RapidAPI wrapper or an Apify actor (~$50/mo), both
// brittle. This stub exists so the Provider port and the panel UI are
// ready for a real adapter without inventing one against nothing, and so
// a rental watch fails clearly ("not configured") instead of silently
// returning no data. See PLAN.md § Known gaps and risks.
package rental

import (
	"context"
	"fmt"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type Provider struct{}

func New() *Provider { return &Provider{} }

func (p *Provider) Kind() domain.AssetKind { return domain.AssetRental }

func (p *Provider) Fetch(ctx context.Context, w domain.Watch) ([]domain.Quote, error) {
	return nil, fmt.Errorf("rental: not configured — Airbnb has no official API, see PLAN.md § Known gaps and risks")
}
