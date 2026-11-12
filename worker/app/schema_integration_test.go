//go:build integration

package app

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// The worker's start refuses a database that has not applied its chain and one whose compatibility
// floor is above its chain, and starts on its own schema and on a newer one whose floor its chain meets,
// reading both under each job kind's role, since the host checks each job kind's pool (ADR-0140).
func TestTheWorkerRefusesASchemaItCannotRunOn(t *testing.T) {
	postgres.SchemaChecks(t, "../../db/migrations", checkSchema, "mediated_mailbox_backfill", "mediated_mailbox_sync")
}
