package migrations

import "embed"

// Embedded SQL migrations for single-binary deploys.
//
//go:embed *.sql
var EmbeddedFS embed.FS

