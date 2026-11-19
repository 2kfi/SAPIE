// Package migrations embeds the SQL applied at every startup.
package migrations

import _ "embed"

// Schema is the idempotent initial migration.
//
//go:embed 001_init.sql
var Schema string
