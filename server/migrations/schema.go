// Package migrations embeds the SQL applied at every startup.
package migrations

import _ "embed"

// Files are applied in order. Every statement is idempotent so old and fresh
// databases converge on the same schema.
var Files = []string{schema001, schema002}

//go:embed 001_init.sql
var schema001 string

//go:embed 002_social.sql
var schema002 string
