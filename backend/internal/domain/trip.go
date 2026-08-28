package domain

import "time"

type TripID string

// Trip is the scheduled, watched entity — one cron entry, one combined
// digest. Watch (a flight or lodging leg) has no schedule of its own;
// every watch belongs to exactly one trip and fires on that trip's cron.
type Trip struct {
	ID       TripID
	Name     string
	CronExpr string
	Timezone string
	// Enabled pauses the whole trip (and every leg in it). A single leg
	// can also be paused independently via Watch.Enabled.
	Enabled   bool
	CreatedAt time.Time
}
