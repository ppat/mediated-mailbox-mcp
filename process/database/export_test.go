package database

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process"
)

// The names the package's external tests read, which the package does not export.
var PoolConfig = poolConfig

type Tracer = statementTracer

// OpenPool opens a pool as open does, with a record that reports nothing.
func OpenPool(ctx context.Context, config *pgxpool.Config, reg prometheus.Registerer) (*pgxpool.Pool, error) {
	return openPool(ctx, config, reg, &record{})
}

// NewRecordingTracer returns a tracer whose record reports under reporter, as a pool Get keeps does,
// with holding standing for whether the pool holds a connection.
func NewRecordingTracer(reg prometheus.Registerer, reporter process.Capabilities, holding func() bool) (*Tracer, error) {
	outcomes := &record{}
	outcomes.attach(nil, holding)
	outcomes.start(reporter)
	return newTracer(reg, outcomes)
}

// The bounds the record's tests read.
const (
	NoConnectionBound = noConnectionBound
	ConnectTimeout    = connectTimeout
)
