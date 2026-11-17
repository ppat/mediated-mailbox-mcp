// Package lookalike sits in a directory whose name only starts with api, so it is not the API server's.
package lookalike

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func start(f func()) {
	go f()
}

func statement(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "SELECT 1")
}
