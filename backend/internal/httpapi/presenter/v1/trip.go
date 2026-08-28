package v1

import (
	"time"

	"github.com/SamPariat/pricewatch/internal/domain"
)

type Trip struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CronExpr  string    `json:"cron_expr"`
	Timezone  string    `json:"timezone"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	// Legs is populated by the list/get handler from a separate
	// ListWatches call — Trip itself carries no leg data.
	Legs []Watch `json:"legs,omitempty"`
}

func TripOf(t domain.Trip, legs []Watch) Trip {
	return Trip{
		ID: string(t.ID), Name: t.Name, CronExpr: t.CronExpr, Timezone: t.Timezone,
		Enabled: t.Enabled, CreatedAt: t.CreatedAt, Legs: legs,
	}
}

type Request struct {
	ID         string     `json:"id"`
	Kind       string     `json:"kind"`
	Status     string     `json:"status"`
	Title      string     `json:"title"`
	Note       string     `json:"note"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

func RequestOf(r domain.Request) Request {
	return Request{
		ID: string(r.ID), Kind: string(r.Kind), Status: string(r.Status),
		Title: r.Title, Note: r.Note, CreatedAt: r.CreatedAt, ResolvedAt: r.ResolvedAt,
	}
}
