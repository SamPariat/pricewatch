package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/i18n"
)

// Locale resolves the request's Accept-Language header into an
// internal/i18n.Locale and stores it under i18n.CtxKey via c.Locals.
// Since fiber.Ctx satisfies context.Context by reading that same Locals
// store in its own Value method, i18n.From(ctx) finds it anywhere a
// handler passes ctx (the fiber.Ctx itself, per this app's existing
// convention) straight into a service call — no per-handler plumbing
// needed beyond registering this once in router.go.
//
// Registered ahead of auth so even pre-login responses (/auth/login,
// /meta) are localized — the panel's ApiClient sets Accept-Language from
// its own cookie on every call, login included (web/lib/api.ts).
func Locale() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals(i18n.CtxKey, i18n.Parse(c.Get(fiber.HeaderAcceptLanguage)))
		return c.Next()
	}
}
