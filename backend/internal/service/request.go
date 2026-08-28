package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/SamPariat/pricewatch/internal/domain"
)

// RequestService owns the Discord approval queue: a Request records a
// change a Discord command asked for, and does nothing on its own until
// Approve applies it — through the same WatchService/TripService methods
// the HTTP API and Discord buttons use, never by touching Repo directly
// for the mutation itself. This isn't access control (there's one
// admin) — it's a confirm-before-committing safety net.
type RequestService struct {
	Repo    domain.Repository
	Watches *WatchService
	Trips   *TripService
}

// Submit records a pending request. payload is marshaled to JSON here so
// callers (the Discord command handlers) can pass a typed struct
// (domain.AddLegPayload, domain.CreateTripPayload) or a bare string
// (a WatchID for RequestRemoveLeg) without doing the marshaling
// themselves.
func (s *RequestService) Submit(ctx context.Context, kind domain.RequestKind, payload any, title, note string) (domain.Request, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.Request{}, fmt.Errorf("service: marshal request payload: %w", err)
	}
	return s.Repo.CreateRequest(ctx, domain.Request{
		Kind: kind, Payload: raw, Status: domain.RequestPending, Title: title, Note: note,
	})
}

func (s *RequestService) List(ctx context.Context, status domain.RequestStatus) ([]domain.Request, error) {
	return s.Repo.ListRequests(ctx, status)
}

// Approve decodes the request's Payload by Kind and applies it through
// WatchService/TripService. A failure applying it (e.g. the trip named
// in an add-leg request was deleted since the request was submitted)
// leaves the request pending with the error returned to the caller —
// it is never auto-rejected, since the admin should see why and decide.
func (s *RequestService) Approve(ctx context.Context, id domain.RequestID) error {
	req, err := s.Repo.GetRequest(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if req.Status != domain.RequestPending {
		return ValidationError{fmt.Errorf("service: request %s is already %s", id, req.Status)}
	}

	switch req.Kind {
	case domain.RequestAddLeg:
		var p domain.AddLegPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return fmt.Errorf("service: decode add_leg payload: %w", err)
		}
		params, err := json.Marshal(p.Params)
		if err != nil {
			return fmt.Errorf("service: marshal leg params: %w", err)
		}
		if _, err := s.Watches.Create(ctx, WatchInput{
			Kind: string(p.Kind), Params: params, TripID: string(p.TripID),
		}); err != nil {
			return err
		}
	case domain.RequestRemoveLeg:
		var watchID string
		if err := json.Unmarshal(req.Payload, &watchID); err != nil {
			return fmt.Errorf("service: decode remove_leg payload: %w", err)
		}
		if err := s.Watches.Delete(ctx, domain.WatchID(watchID)); err != nil {
			return err
		}
	case domain.RequestCreateTrip:
		var p domain.CreateTripPayload
		if err := json.Unmarshal(req.Payload, &p); err != nil {
			return fmt.Errorf("service: decode create_trip payload: %w", err)
		}
		if _, err := s.Trips.Create(ctx, p.Name, p.CronExpr, p.Timezone, true); err != nil {
			return err
		}
	default:
		return fmt.Errorf("service: unknown request kind %q", req.Kind)
	}

	return s.Repo.ResolveRequest(ctx, id, domain.RequestApproved)
}

func (s *RequestService) Reject(ctx context.Context, id domain.RequestID) error {
	if _, err := s.Repo.GetRequest(ctx, id); err != nil {
		return ErrNotFound
	}
	return s.Repo.ResolveRequest(ctx, id, domain.RequestRejected)
}
