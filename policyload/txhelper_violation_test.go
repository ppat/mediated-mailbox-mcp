//go:build banproof

package policyload

import (
	"context"

	"github.com/ppat/mediated-mailbox-mcp/db/policyrules"
)

// This file calls a generated data-access function outside the transaction helper from a test file
// on purpose, since the txhelper analyser holds test files to the rule too, and banproof requires
// the finding annotated below from go vet.
func rulesInATest(ctx context.Context, pool policyrules.DBTX) error {
	_, err := policyrules.New(pool).PolicyRules(ctx, "") // want vetcheck "calls policyrules.New outside a function literal passed to tx.Run"
	return err
}
