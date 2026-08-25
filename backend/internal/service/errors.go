// Package service sits between the HTTP controllers
// (internal/httpapi/handlers) and the domain ports — orchestration logic
// (validation, scheduler reloads, run-now, aggregating multiple ports
// into one view) that used to live directly in handler bodies. Services
// return domain types or small service-level aggregates, never wire
// DTOs — converting to a specific API version's wire shape stays the
// controller/presenter's job (PLAN.md § API versioning: "one domain
// object, N presenters").
package service

import "errors"

// ErrNotFound marks a lookup failure a controller should render as 404.
// Deliberately coarse — matches this app's existing behavior of treating
// any repository error at a get-by-id call site as "not found" rather
// than distinguishing a missing row from a database hiccup.
var ErrNotFound = errors.New("not found")

// ValidationError marks a failure a controller should render as 400 —
// bad input, not a system failure.
type ValidationError struct{ Err error }

func (e ValidationError) Error() string { return e.Err.Error() }
func (e ValidationError) Unwrap() error { return e.Err }

// RunError marks a pipeline run failure — a controller should render
// this as 502 (this server failed to complete work against an upstream),
// not 400 or 500.
type RunError struct{ Err error }

func (e RunError) Error() string { return e.Err.Error() }
func (e RunError) Unwrap() error { return e.Err }
