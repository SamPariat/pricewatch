// Package middleware holds Fiber v3 middleware specific to this API —
// version negotiation and session auth. Generic middleware (recover,
// requestid, logger, limiter, etag, cache) comes straight from Fiber's own
// middleware packages and is wired directly in httpapi.Router, not
// duplicated here.
package middleware

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

const versionLocalsKey = "apiVersion"

// APIVersion enforces the X-API-Version contract described in PLAN.md:
// absent header defaults to the OLDEST supported version, never the
// latest — defaulting to latest would silently break every client that
// predates the header, which is the exact failure versioning exists to
// prevent. A request below min or above max is rejected outright; a
// request below max gets a Deprecation/Sunset warning but still succeeds.
func APIVersion(min, max int) fiber.Handler {
	return func(c fiber.Ctx) error {
		v := min
		if raw := c.Get("X-API-Version"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "X-API-Version must be an integer")
			}
			v = parsed
		}
		if v < min || v > max {
			return fiber.NewError(fiber.StatusBadRequest, "unsupported API version")
		}

		c.Locals(versionLocalsKey, v)
		c.Set("X-API-Version", strconv.Itoa(v))
		if v < max {
			c.Set("Deprecation", "true")
			c.Set("Sunset", sunsetFor(v))
		}
		return c.Next()
	}
}

// Version reads the negotiated version set by APIVersion. Handlers use
// this to pick a presenter, e.g. presenter/v1 vs a future presenter/v2.
func Version(c fiber.Ctx) int {
	v, _ := c.Locals(versionLocalsKey).(int)
	return v
}

// sunsetFor is a placeholder until a v2 actually exists and a real
// deprecation date is chosen for v1 — RFC 8594 wants an HTTP-date here.
func sunsetFor(version int) string {
	return ""
}
