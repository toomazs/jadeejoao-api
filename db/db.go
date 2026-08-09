// Package db embeds the goose SQL migrations so the compiled binary can run
// them at startup without shipping loose files.
package db

import "embed"

//go:embed migrations/*.sql
var Migrations embed.FS
