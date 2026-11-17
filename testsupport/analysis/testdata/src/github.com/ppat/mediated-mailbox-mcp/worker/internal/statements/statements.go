// Package statements stands for a job kind's code in the worker that runs statements by hand.
package statements

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func credential(ctx context.Context, conn *pgx.Conn) {
	_ = conn.QueryRow(ctx, "SELECT credential FROM account_state") // want `The method QueryRow runs a statement the data-access library did not generate`
}
