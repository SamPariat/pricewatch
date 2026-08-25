// @title                       Pricewatch API
// @version                     1.0
// @description                 Backend for the Pricewatch admin panel — watches, price history, run logs, settings, and Telegram channel status. Single-admin: every route except /auth/login and /meta requires a valid session.
// @description                 X-API-Version defaults to the oldest supported version when absent (see internal/httpapi/middleware.APIVersion) — this spec documents version 1.
// @BasePath                    /api
// @securityDefinitions.apikey  CookieAuth
// @in                          header
// @name                        Cookie
// @description                 httpOnly session cookie issued by POST /auth/login (see internal/httpapi/middleware.Session). Paste as "Cookie: pw_session=<value>" — this is the real wire format, not an approximation of the cookie mechanism.
package httpapi

// Regenerate the spec after changing any route or handler doc comment:
//
//	make swagger
//
// (--outputTypes json,yaml deliberately skips swag's generated docs.go —
// see internal/httpapi/docs/embed.go for why.) Served at GET /api/docs
// (Swagger UI) and GET /api/docs/openapi.json (raw spec) — see
// internal/httpapi/handlers/docs.go.
