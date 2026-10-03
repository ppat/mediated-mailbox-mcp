// Package statements stands for a generated data-access subsection's database handle.
package statements

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// DBTX is any database handle.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Secret reads a client's secret through the handle, which is outside the rule's scope.
func Secret(ctx context.Context, db DBTX) pgx.Row {
	return db.QueryRow(ctx, "SELECT client_secret FROM oauth_clients")
}
