//go:build banproof

package service

import "context"

// statements is a handle the service layer could declare over the pool it is given, which the
// driver's pool satisfies.
type statements interface {
	Exec(ctx context.Context, sql string, arguments ...any) (any, error)
}

// This file approves a plan by a statement written by hand, on purpose. The API server connects as
// the application user, whose grants hold the UI's approving write, so what keeps the API server's code
// from approving is its import lists and that every statement it runs is one the data-access library
// generated (ADR-0020, ADR-0071, ADR-0143).
func approveByHand(ctx context.Context, db statements) {
	_, err := db.Exec(ctx, "UPDATE reorg_plans SET status = 'APPROVED'") // want vetcheck "The method Exec runs a statement the data-access library did not generate"
	_ = err
}
