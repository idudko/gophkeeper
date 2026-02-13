// Package migrations contains database migration files for GophKeeper.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var migrationFiles embed.FS

// FS is the filesystem containing migration files.
var FS, _ = fs.Sub(migrationFiles, ".")
