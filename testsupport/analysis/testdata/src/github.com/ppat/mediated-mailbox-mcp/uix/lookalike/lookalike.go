// Package lookalike sits in a directory whose name only starts with ui, so it is out of scope.
package lookalike

import (
	"context"
	"net/http"

	"github.com/jackc/pgx/v5"
)

func mount() {
	http.NewServeMux().Handle("/", http.NotFoundHandler())
}

func statement(ctx context.Context, conn *pgx.Conn) {
	_ = conn.Exec(ctx, "SELECT 1")
}
