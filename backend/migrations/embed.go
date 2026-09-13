// Package migrations exposes the SQL migrations to the API binary.
package migrations

import "embed"

// FS contains the numbered SQL migration files in this directory.
//
//go:embed *.sql
var FS embed.FS
