package domain

import (
	"encoding/json"
	"time"
)

type RequestID string

// RequestKind discriminates what a Request asks for — Payload decodes
// differently per kind, same "opaque JSON, typed by a sibling field"
// pattern as Watch.Params/AssetKind.
type RequestKind string

const (
	// RequestAddLeg's Payload is a JSON-encoded AddLegPayload.
	RequestAddLeg RequestKind = "add_leg"
	// RequestRemoveLeg's Payload is a JSON-encoded string: the WatchID.
	RequestRemoveLeg RequestKind = "remove_leg"
	// RequestCreateTrip's Payload is a JSON-encoded CreateTripPayload.
	RequestCreateTrip RequestKind = "create_trip"
)

// AddLegPayload is RequestAddLeg's decoded Payload — a leg (flight or
// lodging watch) to add to an already-existing trip.
type AddLegPayload struct {
	TripID TripID          `json:"trip_id"`
	Kind   AssetKind       `json:"kind"`
	Params json.RawMessage `json:"params"`
}

// CreateTripPayload is RequestCreateTrip's decoded Payload.
type CreateTripPayload struct {
	Name     string `json:"name"`
	CronExpr string `json:"cron_expr"`
	Timezone string `json:"timezone"`
}

type RequestStatus string

const (
	RequestPending  RequestStatus = "pending"
	RequestApproved RequestStatus = "approved"
	RequestRejected RequestStatus = "rejected"
)

// Request is a Discord-submitted change awaiting admin review — the
// Discord bot only ever creates one of these, it never mutates a watch
// or trip directly. Nothing takes effect until RequestService.Approve
// applies it through the normal WatchService/TripService methods, the
// same ones the HTTP API uses. This isn't access control (there's one
// admin) — it's a confirm-before-committing safety net against a typo'd
// URL or a slip while typing a command on a phone.
type Request struct {
	ID      RequestID
	Kind    RequestKind
	Payload json.RawMessage
	Status  RequestStatus
	// Title and Note are both precomputed at submission time so the
	// panel doesn't need to decode Payload (or look up the trip a leg
	// belongs to) just to render a list row. Title is the short heading
	// — "Delhi to Goa, Dec 4 → 8" — and Note is the supporting detail
	// line, typically the raw URL, which can be long and isn't what a
	// reviewer should read first.
	Title      string
	Note       string
	CreatedAt  time.Time
	ResolvedAt *time.Time
}
