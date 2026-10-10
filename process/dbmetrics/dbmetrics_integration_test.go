//go:build integration

package dbmetrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// measured returns a pool of at most one connection to the test database, its statements traced and
// its statistics collected on reg, as a composition root builds one.
func measured(t *testing.T, reg prometheus.Registerer) *pgxpool.Pool {
	t.Helper()
	config, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	tracer, err := dbmetrics.NewTracer(reg)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := dbmetrics.RegisterPool(reg, pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

// Every statement a traced pool runs against PostgreSQL is observed once under its label, a named
// statement under its name, a unit of data access under its transaction's keywords and db/tx's own
// statement names, a query once its rows are closed, and a statement with no name as unnamed.
func TestEveryStatementIsTimedByItsName(t *testing.T) {
	ctx := t.Context()
	reg := prometheus.NewRegistry()
	pool := measured(t, reg)
	var one int
	if err := pool.QueryRow(ctx, "-- name: CountProbe :one\nSELECT 1").Scan(&one); err != nil {
		t.Fatal(err)
	}
	rows, err := pool.Query(ctx, "-- name: ListProbe :many\nSELECT generate_series(1, 3)")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "SELECT 2"); err != nil {
		t.Fatal(err)
	}
	err = tx.Run(ctx, pool, "personal", func(q pgx.Tx) error {
		_, err := q.Exec(ctx, "-- name: TouchProbe :exec\nSELECT 3")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("the unit fails")
	if err := tx.RunBase(ctx, pool, func(pgx.Tx) error { return failed }); !errors.Is(err, failed) {
		t.Fatalf("RunBase returned %v, want %v", err, failed)
	}
	want := []sample{
		{Labels: "statement=CountProbe", Value: 1},
		{Labels: "statement=ListProbe", Value: 1},
		{Labels: "statement=ReadBaseScope", Value: 1},
		{Labels: "statement=ReadTransactionAccount", Value: 1},
		{Labels: "statement=SetBaseScope", Value: 1},
		{Labels: "statement=SetTransactionAccount", Value: 1},
		{Labels: "statement=TouchProbe", Value: 1},
		{Labels: "statement=begin", Value: 2},
		{Labels: "statement=commit", Value: 1},
		{Labels: "statement=rollback", Value: 1},
		{Labels: "statement=unnamed", Value: 1},
	}
	if diff := cmp.Diff(want, gathered(t, reg, "mediated_mailbox_db_statement_duration_seconds"), compare.Options); diff != "" {
		t.Errorf("the statements observed (-want +got):\n%s", diff)
	}
}

// A pool's acquires are counted, an acquire that waited for the one connection with the time it
// waited, and an acquire whose caller gave up waiting as cancelled.
func TestThePoolsWaitsAreCounted(t *testing.T) {
	ctx := t.Context()
	reg := prometheus.NewRegistry()
	pool := measured(t, reg)
	held, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gaveUp, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := pool.Acquire(gaveUp); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an acquire while the one connection is held returned %v, want the deadline", err)
	}
	const hold = 100 * time.Millisecond
	released := make(chan struct{})
	go func() {
		time.Sleep(hold)
		held.Release()
		close(released)
	}()
	waited, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	<-released
	got := map[string][]sample{}
	for _, name := range []string{
		"mediated_mailbox_db_pool_acquires_total", "mediated_mailbox_db_pool_waited_acquires_total",
		"mediated_mailbox_db_pool_canceled_acquires_total", "mediated_mailbox_db_pool_acquired_connections",
		"mediated_mailbox_db_pool_max_connections",
	} {
		got[name] = gathered(t, reg, name)
	}
	want := map[string][]sample{
		"mediated_mailbox_db_pool_acquires_total":          {{Value: 2}},
		"mediated_mailbox_db_pool_waited_acquires_total":   {{Value: 2}},
		"mediated_mailbox_db_pool_canceled_acquires_total": {{Value: 1}},
		"mediated_mailbox_db_pool_acquired_connections":    {{Value: 1}},
		"mediated_mailbox_db_pool_max_connections":         {{Value: 1}},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the pool's series (-want +got):\n%s", diff)
	}
	if wait := gathered(t, reg, "mediated_mailbox_db_pool_acquire_wait_seconds_total"); len(wait) != 1 || wait[0].Value < hold.Seconds()/2 {
		t.Errorf("the acquires waited %v seconds in all, want at least %v", wait, hold.Seconds()/2)
	}
	waited.Release()
}
