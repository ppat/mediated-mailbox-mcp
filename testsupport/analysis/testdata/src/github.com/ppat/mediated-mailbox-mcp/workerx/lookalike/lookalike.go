// Package lookalike sits in a directory whose name only starts with worker, so it is not the worker's.
package lookalike

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
)

func start(f func()) {
	go f()
	var wg sync.WaitGroup
	wg.Go(f)
}

func statement(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "SELECT 1")
}
