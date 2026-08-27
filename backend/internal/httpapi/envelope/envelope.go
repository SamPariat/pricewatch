// Package envelope wraps every /api/v1 JSON response in one consistent
// shape, success or failure. Two exemptions by design: the two Swagger
// doc routes (their body IS the payload — a spec document or an HTML
// page — wrapping it would break Swagger UI), and 204 responses (Login,
// Logout, DeleteWatch), which stay genuinely bodyless per RFC 9110.
package envelope

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/i18n"
)

// Envelope is the wire shape of every enveloped response. Data is `any`
// rather than a generic type parameter because callers wrap slices,
// single DTOs, and small ad hoc structs interchangeably, and c.JSON needs
// no generic plumbing to marshal any of them the same way.
type Envelope struct {
	Data       any    `json:"data"`
	Message    string `json:"message"`
	Error      any    `json:"error"` // string on failure, JSON null on success
	StatusCode int    `json:"status_code"`
}

// Ok writes a success envelope. status is echoed into both the HTTP
// status line and the body's status_code field, so a caller reading only
// the parsed body (e.g. a future audit log) never loses it.
func Ok(c fiber.Ctx, status int, data any, message string) error {
	return c.Status(status).JSON(Envelope{Data: data, Message: message, Error: nil, StatusCode: status})
}

// ErrorHandler renders every failure — every existing fiber.NewError call
// site across the API, plus Fiber's own 404 for unmatched routes — as an
// envelope, without any of those call sites needing to change. Wired once
// via fiber.Config{ErrorHandler: envelope.ErrorHandler}.
func ErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	msg := i18n.T(i18n.From(c), "common.internal_error")
	if fe, ok := err.(*fiber.Error); ok {
		code, msg = fe.Code, fe.Message
	}
	return c.Status(code).JSON(Envelope{Data: nil, Message: msg, Error: msg, StatusCode: code})
}
