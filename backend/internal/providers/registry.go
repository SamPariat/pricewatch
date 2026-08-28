// Package providers holds the Provider port's adapters (aviasales) plus
// the resilience/caching decorator stack described in PLAN.md — each
// decorator satisfies domain.Provider itself, so they compose without
// any adapter knowing about retry, caching, or rate limits.
package providers

import (
	"fmt"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// Registry looks up a Provider by the AssetKind it serves. Adding a new
// price source is "register another entry", not "edit a switch statement".
type Registry struct {
	byKind map[domain.AssetKind]domain.Provider
}

func NewRegistry() *Registry {
	return &Registry{byKind: make(map[domain.AssetKind]domain.Provider)}
}

func (r *Registry) Register(p domain.Provider) {
	r.byKind[p.Kind()] = p
}

func (r *Registry) For(kind domain.AssetKind) (domain.Provider, error) {
	p, ok := r.byKind[kind]
	if !ok {
		return nil, fmt.Errorf("providers: no provider registered for kind %q", kind)
	}
	return p, nil
}
