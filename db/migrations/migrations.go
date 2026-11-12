// Package migrations holds the migration chain, the SQL files beside this one (ADR-0048), and embeds
// them, so the migrator's binary carries the chain it applies and the integration runs apply the same
// files. The files stay plain SQL, which the code generator reads as its schema and skips this file.
package migrations

import (
	"embed"
	"io/fs"
)

//go:embed *.sql
var chain embed.FS

// Chain is the migration chain this build carries, each file at the top of the returned file system.
func Chain() fs.FS {
	return chain
}
