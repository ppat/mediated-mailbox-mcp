//go:build integration

package app

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// The worker's start refuses a database that has not applied its chain and one whose compatibility
// floor is above its chain, and starts on its own schema and on a newer one whose floor its chain meets,
// reading both as the application user, which every job kind's pool connects as (ADR-0140, ADR-0143).
func TestTheWorkerRefusesASchemaItCannotRunOn(t *testing.T) {
	postgres.SchemaChecks(t, "../../db/migrations", checkSchema, postgres.ApplicationUser)
}
