package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/database/core"
)

// statementName is the series timing every statement, labelled by the statement's name.
const statementName = "mediated_mailbox_db_statement_duration_seconds"

// statementTracer times every statement run through a connection it is set on, as a pgx.QueryTracer,
// and writes the outcome of each statement and each connection attempt to the pool's record, as a
// pgx.ConnectTracer too.
type statementTracer struct {
	duration *prometheus.HistogramVec
	outcomes *record
}

var (
	_ pgx.QueryTracer   = (*statementTracer)(nil)
	_ pgx.ConnectTracer = (*statementTracer)(nil)
)

// newTracer registers the statement series on reg and returns the tracer that observes it and writes
// outcomes.
func newTracer(reg prometheus.Registerer, outcomes *record) (*statementTracer, error) {
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    statementName,
		Help:    "How long each statement took, from its start until its result was read or its rows were closed, by the name its SQL carries.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"statement"})
	if err := reg.Register(unchecked{h}); err != nil {
		return nil, err
	}
	return &statementTracer{duration: h, outcomes: outcomes}, nil
}

// started is what TraceQueryStart hands TraceQueryEnd through the context.
type started struct {
	name string
	at   time.Time
}

type startedKey struct{}

// TraceQueryStart implements pgx.QueryTracer.
func (t *statementTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, startedKey{}, started{name: core.Label(data.SQL), at: time.Now()})
}

// TraceQueryEnd implements pgx.QueryTracer. pgx calls it once the statement's result is read, which
// for a query is when its rows are closed.
func (t *statementTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.outcomes.observe(ctx, outcome(ctx, false, conn, data.Err), data.Err)
	s, ok := ctx.Value(startedKey{}).(started)
	if !ok {
		return
	}
	t.duration.WithLabelValues(s.name).Observe(time.Since(s.at).Seconds())
}

// TraceConnectStart implements pgx.ConnectTracer.
func (t *statementTracer) TraceConnectStart(ctx context.Context, _ pgx.TraceConnectStartData) context.Context {
	return ctx
}

// TraceConnectEnd implements pgx.ConnectTracer. pgx calls it once a connection attempt has connected
// or failed.
func (t *statementTracer) TraceConnectEnd(ctx context.Context, data pgx.TraceConnectEndData) {
	t.outcomes.observe(ctx, outcome(ctx, true, nil, data.Err), data.Err)
}
