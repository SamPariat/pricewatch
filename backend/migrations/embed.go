// Package migrations embeds the goose SQL migration files into the compiled
// binary, so a distroless production image needs no migrations/ directory
// mounted alongside it — the binary carries its own schema history.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
