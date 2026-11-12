//go:build integration

package app

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// The UI's start refuses a database that has not applied its chain and one whose compatibility floor is
// above its chain, and starts on its own schema and on a newer one whose floor its chain meets, reading
// both under its own role (ADR-0140).
func TestTheUIRefusesASchemaItCannotRunOn(t *testing.T) {
	postgres.SchemaChecks(t, "../../db/migrations", checkSchema, "mediated_mailbox_ui")
}
