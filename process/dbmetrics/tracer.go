// Package dbmetrics measures a deployable's database connection, the latency of every statement it
// runs by the statement's name and the statistics of each connection pool (ADR-0125). A composition
// root sets the tracer on a pool's connection configuration and registers the pool's collector, both
// on the registerer it passes in. Its case is argued in process/README.md.
package dbmetrics

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
)

// statementName is the series timing every statement, labelled by the statement's name.
const statementName = "mediated_mailbox_db_statement_duration_seconds"

// unnamed is the label of a statement whose SQL carries no name and starts with no transaction
// keyword, so the label's values stay a closed set.
const unnamed = "unnamed"

// Tracer times every statement run through a connection it is set on, as a pgx.QueryTracer.
type Tracer struct {
	duration *prometheus.HistogramVec
}

var _ pgx.QueryTracer = (*Tracer)(nil)

// NewTracer registers the statement series on reg and returns the tracer that observes it.
func NewTracer(reg prometheus.Registerer) (*Tracer, error) {
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    statementName,
		Help:    "How long each statement took, from its start until its result was read or its rows were closed, by the name its SQL carries.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"statement"})
	if err := reg.Register(h); err != nil {
		return nil, err
	}
	return &Tracer{duration: h}, nil
}

// started is what TraceQueryStart hands TraceQueryEnd through the context.
type started struct {
	name string
	at   time.Time
}

type startedKey struct{}

// TraceQueryStart implements pgx.QueryTracer.
func (t *Tracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, startedKey{}, started{name: Label(data.SQL), at: time.Now()})
}

// TraceQueryEnd implements pgx.QueryTracer. pgx calls it once the statement's result is read, which
// for a query is when its rows are closed.
func (t *Tracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	s, ok := ctx.Value(startedKey{}).(started)
	if !ok {
		return
	}
	t.duration.WithLabelValues(s.name).Observe(time.Since(s.at).Seconds())
}

// Label returns a statement's label. It is the name in the `-- name: <Name>` comment the statement's
// SQL starts with, as sqlc writes it, begin, commit or rollback for a statement without one that
// starts with that keyword, and unnamed for every other, so no label is taken from the SQL's text
// beyond a name the code chose.
func Label(sql string) string {
	text := strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(text, "-- name:"); ok {
		line, _, _ := strings.Cut(rest, "\n")
		if fields := strings.Fields(line); len(fields) > 0 && identifier(fields[0]) {
			return fields[0]
		}
		return unnamed
	}
	first, _, _ := strings.Cut(text, " ")
	switch keyword := strings.ToLower(strings.TrimSuffix(first, ";")); keyword {
	case "begin", "commit", "rollback":
		return keyword
	default:
		return unnamed
	}
}

// identifier reports whether name is letters, digits and underscores, the form sqlc requires of a
// statement's name, so a comment holding anything else is no name.
func identifier(name string) bool {
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return name != ""
}
