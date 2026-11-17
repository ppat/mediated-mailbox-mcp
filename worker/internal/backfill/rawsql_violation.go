//go:build banproof

package backfill

import "context"

// statements is a handle a job kind's code could declare over the pool it is given, which the
// driver's pool satisfies.
type statements interface {
	QueryRow(ctx context.Context, sql string, arguments ...any) any
}

// This file reads a sealed credential by a statement written by hand, on purpose. Every job kind
// connects as the application user, which reads the sealed credentials for the connections, so what
// keeps backfill's code from reading one is its import list and that every statement it runs is one
// the data-access library generated (ADR-0071, ADR-0117, ADR-0143).
func credentialByHand(ctx context.Context, db statements) {
	_ = db.QueryRow(ctx, "SELECT credential FROM account_state") // want vetcheck "The method QueryRow runs a statement the data-access library did not generate"
}
