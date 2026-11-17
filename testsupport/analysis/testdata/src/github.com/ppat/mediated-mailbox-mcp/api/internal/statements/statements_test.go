package statements

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// A test runs statements by hand to set up what the API server reads, and is never served.
func seed(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "INSERT INTO reorg_plans DEFAULT VALUES")
}
