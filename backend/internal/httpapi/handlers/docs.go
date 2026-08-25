package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/SamPariat/pricewatch/internal/httpapi/docs"
)

// OpenAPISpec serves the generated Swagger 2.0 spec (see
// internal/httpapi/doc.go for the regeneration command) — not a route
// worth its own swag annotation, since documenting the documentation
// endpoint in itself is circular.
func (a *API) OpenAPISpec(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	return c.Send(docs.SwaggerJSON)
}

// swaggerUIPage loads swagger-ui from a CDN rather than vendoring the
// bundle into the binary — this is an internal ops page behind the same
// session auth as the rest of /api (see router.go), not the product UI,
// so a CDN script tag here doesn't carry the same "never ship external
// assets to end users" weight the actual panel does.
const swaggerUIPage = `<!DOCTYPE html>
<html>
<head>
	<title>Pricewatch API docs</title>
	<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
	<div id="swagger-ui"></div>
	<script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	<script>
		window.onload = () => SwaggerUIBundle({
			url: "/api/` + APIVersion + `/docs/openapi.json",
			dom_id: "#swagger-ui",
		});
	</script>
</body>
</html>`

// SwaggerUI serves an HTML shell that renders the spec from
// /api/{APIVersion}/docs/openapi.json via Swagger UI.
func (a *API) SwaggerUI(c fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.SendString(swaggerUIPage)
}
