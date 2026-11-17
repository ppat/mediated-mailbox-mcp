package database_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Only the migrator's configuration type can ask for the migrator's pool, which is opened with no
// schema check, so the compiler refuses a deployable's Build that hands ForMigration its own
// configuration. Left to review are a Build that makes a config.Migrator value of its own, by a
// literal, by DefaultMigrator or as a zero value, and, until process/host is deleted, Open and
// OpenPool, which run no schema check, and the driver's own pool (ADR-0140, ADR-0142). A
// configuration type with no database part cannot ask for a pool at all.
func TestOnlyTheMigratorCanAskForThePoolWithNoSchemaCheck(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"formigrationapi", "config.API does not implement config.HasMigratorDatabase (missing method migrator)"},
		{"formigrationui", "config.UI does not implement config.HasMigratorDatabase (missing method migrator)"},
		{"formigrationworker", "config.Worker does not implement config.HasMigratorDatabase (missing method migrator)"},
		{"getwithoutdatabase", "noDatabase does not implement config.HasDatabase (missing method DatabasePart)"},
	} {
		t.Run(c.name, func(t *testing.T) {
			mustnotcompile.Require(t, "./testdata/mustnotcompile/"+c.name, c.want)
		})
	}
}
