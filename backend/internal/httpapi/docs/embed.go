// Package docs embeds the generated OpenAPI/Swagger spec so it ships
// inside the binary — no filesystem lookup at runtime, works the same
// way in the distroless production image as it does locally.
//
// swagger.json/swagger.yaml below are generated, not hand-written — see
// the regeneration command in internal/httpapi/doc.go. This file is the
// one hand-written exception: swag's own generated docs.go additionally
// registers the spec with github.com/swaggo/swag's runtime lookup
// machinery (for host/scheme templating across multiple environments),
// which internal/httpapi/handlers/docs.go doesn't need — it serves this
// embedded JSON directly, so pulling in that extra dependency isn't
// worth it for one static byte slice.
package docs

import _ "embed"

//go:embed swagger.json
var SwaggerJSON []byte
