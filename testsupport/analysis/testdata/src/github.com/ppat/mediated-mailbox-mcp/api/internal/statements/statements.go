// Package statements stands for a package of the API server that runs statements by hand.
package statements

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func approve(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "UPDATE reorg_plans SET status = 'APPROVED'") // want `The method Exec runs a statement the data-access library did not generate`
}
