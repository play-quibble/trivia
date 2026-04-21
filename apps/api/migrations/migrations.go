// Package migrations exposes the schema migration files as an embedded filesystem
// so the API binary can run them on startup without needing the raw SQL files
// present on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
