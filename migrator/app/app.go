// Package app is the migrator's entry package, which holds its spec and nothing else (ADR-0142). The
// spec lists one task, built by migrate.Build in migrator/internal/migrate, which applies the
// migration chain the binary carries (ADR-0139). The process starts the migrator as it starts every
// program, refusing the database driver's password variables before the configuration is read,
// reading the configuration, answering --help and --version, logging every effective value, setting
// the level from log_level, running the configuration's checks and writing the start banner, then
// runs the task and exits.
package app

import (
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/migrator/internal/migrate"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// Spec is the migrator's spec.
func Spec() process.Spec[config.Migrator] {
	return process.Spec[config.Migrator]{
		Name: "migrator",
		Task: &process.Task[config.Migrator]{Build: migrate.Build},
	}
}
