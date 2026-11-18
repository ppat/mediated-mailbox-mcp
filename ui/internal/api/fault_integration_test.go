//go:build integration

package api_test

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errInjected is the failure a fault puts in place of a statement's.
var errInjected = errors.New("a fault injected between the writes")

// fault is the statement a test fails inside a transaction, so a write is proven to land together
// with the statements before it or not at all (ADR-0060). Every server the rig builds runs its
// transactions through it, and it fails nothing until a test arms it. It fails the database's
// answer, so nothing in the shipped code knows a test is there (ADR-0043).
type fault struct {
	mu sync.Mutex
	// name is the generated statement failed, as its sqlc comment names it, empty when unarmed.
	name string
	// nth is which run of the statement fails, counting from 1, and seen how many have run since the
	// fault was armed.
	nth, seen int
}

// arm fails the nth run of the statement named name that a transaction runs from now on.
func (f *fault) arm(name string, nth int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name, f.nth, f.seen = name, nth, 0
}

// disarm fails nothing from now on, and returns how many runs of the armed statement it saw, so a
// test checks the fault was reached and a renamed statement cannot disarm it unseen.
func (f *fault) disarm() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	seen := f.seen
	f.name, f.nth, f.seen = "", 0, 0
	return seen
}

// fails reports whether sql is the run the fault fails, counting it when it is the armed statement.
func (f *fault) fails(sql string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.name == "" || !strings.Contains(sql, "-- name: "+f.name+" ") {
		return false
	}
	f.seen++
	return f.seen == f.nth
}

// faultyDB is the UI's pool with every transaction it begins run through the fault.
type faultyDB struct {
	*pgxpool.Pool
	fault *fault
}

func (d *faultyDB) Begin(ctx context.Context) (pgx.Tx, error) {
	t, err := d.Pool.Begin(ctx)
	return &faultyTx{Tx: t, fault: d.fault}, err
}

func (d *faultyDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	t, err := d.Pool.BeginTx(ctx, opts)
	return &faultyTx{Tx: t, fault: d.fault}, err
}

type faultyTx struct {
	pgx.Tx
	fault *fault
}

func (t *faultyTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if t.fault.fails(sql) {
		return pgconn.CommandTag{}, errInjected
	}
	return t.Tx.Exec(ctx, sql, args...)
}

func (t *faultyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if t.fault.fails(sql) {
		return nil, errInjected
	}
	return t.Tx.Query(ctx, sql, args...)
}

func (t *faultyTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if t.fault.fails(sql) {
		return failedRow{}
	}
	return t.Tx.QueryRow(ctx, sql, args...)
}

// failedRow is a row whose read fails with the injected fault.
type failedRow struct{}

func (failedRow) Scan(...any) error { return errInjected }
