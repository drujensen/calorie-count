// Package migrations embeds SQL migration files for use by the db package.
package migrations

import "embed"

// FS holds all SQL migration files embedded at compile time.
//
//go:embed *.sql
var FS embed.FS
