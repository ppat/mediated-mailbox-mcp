package statements

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// A test runs statements by hand to read what a job kind stored, and is never served.
func read(ctx context.Context, conn *pgx.Conn) {
	_ = conn.QueryRow(ctx, "SELECT credential FROM account_state")
}
