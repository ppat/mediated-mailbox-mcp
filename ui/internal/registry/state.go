package registry

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// AccountProgress is an account's state as the UI may read it, never its credential (ADR-0084,
// ADR-0091). An account with no state row is not connected, and its flags read false. The jobs, system
// and stream reads and account settings' read share it.
type AccountProgress struct {
	Connected       bool
	Pass1Complete   bool
	Pass2Complete   bool
	SyncCursorAt    *string
	LastAuthAt      *string
	LastAuthOutcome *string
}

// Progress reads the account's progress.
func Progress(ctx context.Context, q *accountstate.Queries) (AccountProgress, error) {
	row, err := q.AccountProgress(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountProgress{}, nil
	}
	if err != nil {
		return AccountProgress{}, err
	}
	return AccountProgress{
		Connected:       true,
		Pass1Complete:   row.BackfillPass1Complete,
		Pass2Complete:   row.BackfillPass2Complete,
		SyncCursorAt:    optionalStamp(row.SyncCursorAt),
		LastAuthAt:      optionalStamp(row.LastAuthAt),
		LastAuthOutcome: OptionalText(row.LastAuthOutcome),
	}, nil
}

// RateBlock is the rate state the jobs screen shows, in units per second, with the reservation and
// use of each priority class the limiter records (ADR-0024, ADR-0025, docs/UI.md section 8.3). The
// jobs and stream reads and account settings' read share it.
type RateBlock struct {
	Current        float64         `json:"current"`
	Target         float64         `json:"target"`
	Cap            float64         `json:"cap"`
	BackoffUntil   *string         `json:"backoff_until"`
	LastThrottleAt *string         `json:"last_throttle_at"`
	Classes        json.RawMessage `json:"classes"`
}

// RateType is the rate block's declaration.
func RateType() schema.Type {
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

// Rate reads the account's rate state, nil for an account that has never spent.
func Rate(ctx context.Context, q *ratestate.Queries) (*RateBlock, error) {
	row, err := q.RateStatus(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &RateBlock{
		Current:        Widen(row.CurrentRate),
		Target:         Widen(row.TargetRate),
		Cap:            Widen(row.HardCap),
		BackoffUntil:   optionalStamp(row.BackoffUntil),
		LastThrottleAt: optionalStamp(row.LastThrottleAt),
		Classes:        RawJSON(row.Classes),
	}, nil
}

// OptionalText is a nullable stored text, nil when null.
func OptionalText(t pgtype.Text) *string {
	if !t.Valid {
		return nil
	}
	return &t.String
}
