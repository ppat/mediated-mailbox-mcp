package database_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
)

// reports is a part's capabilities that keep every report it is given, in order.
type reports struct {
	mu   sync.Mutex
	seen []string
}

func (r *reports) Logger() *slog.Logger                   { return slog.New(slog.DiscardHandler) }
func (r *reports) Metrics() prometheus.Registerer         { return prometheus.NewRegistry() }
func (r *reports) Go(string, func(context.Context)) bool  { return false }
func (r *reports) Draining() context.Context              { return context.Background() }
func (r *reports) Report(state process.State, why string) { r.add(fmt.Sprintf("%d %s", state, why)) }

func (r *reports) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seen = append(r.seen, s)
}

func (r *reports) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	seen := r.seen
	r.seen = nil
	return seen
}

// sqlState is an error the server returned, carrying its SQLSTATE as the driver's errors do.
type sqlState string

func (s sqlState) Error() string    { return "the server refused the statement: " + string(s) }
func (s sqlState) SQLState() string { return string(s) }

// A pool's record reports its database from the outcomes of the statements and connection attempts
// that end on it. It reports ready once the pool is kept. A statement the server refused for its own
// reasons and a statement or attempt its caller stopped waiting for change nothing. A failed attempt
// or a statement the server could not serve reports not ready, which the next success clears. Failed
// attempts with nothing reachable between them for the bound report failed, which a statement the
// database answered clears, and the next failed attempt starts a new bound. Attempts that fail while
// the pool holds a connection change nothing however long they go on, since the pool still serves on
// what it holds, and once it holds none a failed attempt reports not ready (ADR-0142).
func TestThePoolsRecordFollowsItsOutcomes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		got := &reports{}
		var holding atomic.Bool
		tracer, err := database.NewRecordingTracer(prometheus.NewRegistry(), got, holding.Load)
		if err != nil {
			t.Fatal(err)
		}
		ctx := t.Context()
		ended, cancel := context.WithCancel(ctx)
		cancel()
		refused := errors.New("connection refused")
		statement := func(ctx context.Context, err error) {
			tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: err})
		}
		connect := func(ctx context.Context, err error) {
			tracer.TraceConnectEnd(ctx, pgx.TraceConnectEndData{Err: err})
		}
		const (
			ready    = "1 "
			notReady = "0 the database could not serve the latest statement or connection attempt"
		)
		failed := fmt.Sprintf("2 no connection to the database has been made for more than %s", database.NoConnectionBound)
		steps := []struct {
			name string
			do   func()
			want []string
		}{
			{"kept", func() {}, []string{ready}},
			{"a unique violation", func() { statement(ctx, sqlState("23505")) }, nil},
			{"a statement timeout", func() { statement(ctx, sqlState("57014")) }, nil},
			{"a statement its caller cancelled", func() { statement(ended, refused) }, nil},
			{"an attempt its caller cancelled", func() { connect(ended, refused) }, nil},
			{"a failed attempt", func() { connect(ctx, refused) }, []string{notReady}},
			{"a statement that succeeded", func() { statement(ctx, nil) }, []string{ready}},
			{"an administrator's shutdown", func() { statement(ctx, sqlState("57P01")) }, []string{notReady}},
			{"an attempt that succeeded", func() { connect(ctx, nil) }, []string{ready}},
			{"a refused password", func() { connect(ctx, sqlState("28P01")) }, []string{notReady}},
			{"attempts failing short of the bound", func() {
				for range 14 {
					time.Sleep(time.Minute)
					connect(ctx, refused)
				}
			}, nil},
			{"an attempt failing at the bound", func() {
				time.Sleep(time.Minute)
				connect(ctx, refused)
			}, []string{failed}},
			{"a statement the server refused for its own reasons", func() { statement(ctx, sqlState("23505")) }, []string{ready}},
			{"a failed attempt after it", func() {
				time.Sleep(time.Hour)
				connect(ctx, refused)
			}, []string{notReady}},
			{"a statement that succeeded on a connection the pool holds", func() {
				holding.Store(true)
				statement(ctx, nil)
			}, []string{ready}},
			{"attempts failing for an hour while the pool holds a connection", func() {
				for range 60 {
					time.Sleep(time.Minute)
					connect(ctx, refused)
				}
			}, nil},
			{"an attempt failing once the pool holds none", func() {
				holding.Store(false)
				connect(ctx, refused)
			}, []string{notReady}},
		}
		for _, s := range steps {
			s.do()
			if g := got.take(); fmt.Sprint(g) != fmt.Sprint(s.want) {
				t.Errorf("after %s the record reported %q, want %q", s.name, g, s.want)
			}
		}
	})
}
