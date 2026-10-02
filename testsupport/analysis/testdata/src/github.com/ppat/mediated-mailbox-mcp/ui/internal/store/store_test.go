package store

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// A test runs statements by hand to set up and read what the server stored, and is never served.
func seed(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "INSERT INTO oauth_clients DEFAULT VALUES")
}
