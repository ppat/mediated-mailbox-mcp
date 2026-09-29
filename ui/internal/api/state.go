package api

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// accountProgress is an account's state as the UI may read it, never its credential (ADR-0084,
// ADR-0091). An account with no state row is not connected, and its flags read false.
type accountProgress struct {
	Connected       bool
	Pass1Complete   bool
	Pass2Complete   bool
	SyncCursorAt    *string
	LastAuthAt      *string
	LastAuthOutcome *string
}

// progress reads the account's progress.
func (s *Server) progress(ctx context.Context, q *accountstate.Queries, account string) (accountProgress, error) {
	row, err := q.AccountProgress(ctx, account)
	if errors.Is(err, pgx.ErrNoRows) {
		return accountProgress{}, nil
	}
	if err != nil {
		return accountProgress{}, err
	}
	return accountProgress{
		Connected:       true,
		Pass1Complete:   row.BackfillPass1Complete,
		Pass2Complete:   row.BackfillPass2Complete,
		SyncCursorAt:    optionalStamp(row.SyncCursorAt),
		LastAuthAt:      optionalStamp(row.LastAuthAt),
		LastAuthOutcome: optionalText(row.LastAuthOutcome),
	}, nil
}

// rateBlock is the rate state the jobs screen shows, in units per second, with the reservation and
// use of each priority class the limiter records (ADR-0024, ADR-0025, docs/UI.md section 8.3).
type rateBlock struct {
	Current        float64         `json:"current"`
	Target         float64         `json:"target"`
	Cap            float64         `json:"cap"`
	BackoffUntil   *string         `json:"backoff_until"`
	LastThrottleAt *string         `json:"last_throttle_at"`
	Classes        json.RawMessage `json:"classes"`
}

func rateType() schema.Type {
	return schema.Obj("Rate",
		schema.F("current", schema.Num()),
		schema.F("target", schema.Num()),
		schema.F("cap", schema.Num()),
		schema.F("backoff_until", schema.Null(schema.Time())),
		schema.F("last_throttle_at", schema.Null(schema.Time())),
		schema.F("classes", schema.Null(schema.MapOf(schema.Obj("RateClass",
			schema.F("reserved", schema.Num()),
			schema.F("used", schema.Num()),
		)))),
	)
}

// rate reads the account's rate state, nil for an account that has never spent.
func (s *Server) rate(ctx context.Context, q *ratestate.Queries, account string) (*rateBlock, error) {
	row, err := q.RateStatus(ctx, account)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &rateBlock{
		Current:        widen(row.CurrentRate),
		Target:         widen(row.TargetRate),
		Cap:            widen(row.HardCap),
		BackoffUntil:   optionalStamp(row.BackoffUntil),
		LastThrottleAt: optionalStamp(row.LastThrottleAt),
		Classes:        rawJSON(row.Classes),
	}, nil
}

// widen is a stored real as JSON writes it, through its shortest decimal form.
func widen(f float32) float64 { return registry.Widen(f) }
