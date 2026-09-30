package registry

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// planStatuses are the plan statuses as stored (ADR-0020), and planWording their wording (docs/UI.md
// section 11), in the order the status timeline draws them.
func planStatuses() []string {
	return []string{"DRAFT", "APPROVED", "APPLYING", "APPLIED", "ROLLED_BACK", "REJECTED", "APPLY_REFUSED"}
}

func planWording() []string {
	return []string{"Awaiting approval", "Approved, waiting to apply", "Applying", "Applied", "Rolled back", "Rejected", "Apply refused"}
}

// PlanRow is one row of the plans dataset, the plans screen's columns (docs/UI.md section 8.9).
// Messages is the plan's operation count, one per message (ADR-0020).
type PlanRow struct {
	PlanID        string  `json:"plan_id"`
	Description   *string `json:"description"`
	Status        string  `json:"status"`
	Proposer      *string `json:"proposer"`
	CreatedAt     string  `json:"created_at"`
	ApprovedAt    *string `json:"approved_at"`
	ApprovedBy    *string `json:"approved_by"`
	RefusalReason *string `json:"refusal_reason"`
	Messages      int64   `json:"messages"`
	ApplyRun      *RunRef `json:"apply_run"`
	RollbackRun   *RunRef `json:"rollback_run"`
}

// RunRef names a run and its state.
type RunRef struct {
	RunID string  `json:"run_id"`
	State *string `json:"state"`
}

func runRefType() schema.Type {
	return schema.Obj("RunRef",
		schema.F("run_id", schema.Str()),
		schema.F("state", schema.Null(schema.Str("running", "succeeded", "failed"))),
	)
}

func planRowType() schema.Type {
	return schema.Obj("PlanRow",
		schema.F("plan_id", schema.Str()),
		schema.F("description", schema.Null(schema.Str())),
		schema.F("status", schema.Str(planStatuses()...)),
		schema.F("proposer", schema.Null(schema.Str())),
		schema.F("created_at", schema.Time()),
		schema.F("approved_at", schema.Null(schema.Time())),
		schema.F("approved_by", schema.Null(schema.Str())),
		schema.F("refusal_reason", schema.Null(schema.Str())),
		schema.F("messages", schema.Int()),
		schema.F("apply_run", schema.Null(runRefType())),
		schema.F("rollback_run", schema.Null(runRefType())),
	)
}

// plans is the reorg plans, which the plans screen lists and Home's decision inbox filters to DRAFT
// (docs/UI.md sections 8.1 and 8.9). Nothing is groupable, so it has a summary and a rows statement
// only.
func plans() Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:        "plans",
			Ranged:      true,
			RangeColumn: "created_at",
			Dimensions: []lens.Dimension{
				{Name: "status", Storage: "text", Filterable: true, Wording: "status", Values: planStatuses()},
				{Name: "created_at", Storage: "time", Sortable: true, Wording: "created"},
			},
			Default: lens.Defaults{Level: lens.Rows, Range: "all", Sort: lens.Sort{Column: "created_at", Descending: true}},
		},
		Row:     planRowType(),
		Summary: planSummary,
		Rows:    planRows,
	}
}

func planSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	in, out := statusFilter(r.Request)
	rows, err := q.Plans.PlanStatusCounts(ctx, reorgplans.PlanStatusCountsParams{
		AccountID: r.Account, RangeStart: timestamptz(r.Bounds.Start), RangeEnd: timestamptz(r.Bounds.End), StatusIn: in, StatusOut: out,
	})
	if err != nil {
		return nil, Total{}, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Status] = row.Plans
	}
	figures, total := statusFigures(r.Account, "plans", planStatuses(), planWording(), counts)
	return figures, Total{Count: total}, nil
}

func planRows(ctx context.Context, q Queries, r Read) (any, error) {
	out := []PlanRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	in, ex := statusFilter(r.Request)
	rows, err := q.Plans.PlanRows(ctx, reorgplans.PlanRowsParams{
		AccountID: r.Account, RangeStart: timestamptz(r.Bounds.Start), RangeEnd: timestamptz(r.Bounds.End), StatusIn: in, StatusOut: ex,
		Descending: r.Request.Sort.Descending, RowOffset: first,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		p := PlanRow{
			PlanID:        row.PlanID.String(),
			Description:   text(row.Description.Valid, row.Description.String),
			Status:        row.Status,
			Proposer:      text(row.Proposer.Valid, row.Proposer.String),
			CreatedAt:     stamp(row.CreatedAt),
			ApprovedAt:    optionalStamp(row.ApprovedAt),
			ApprovedBy:    text(row.ApprovedBy.Valid, row.ApprovedBy.String),
			RefusalReason: text(row.RefusalReason.Valid, row.RefusalReason.String),
			Messages:      row.Messages,
		}
		if row.ApplyRunID.Valid {
			p.ApplyRun = &RunRef{RunID: row.ApplyRunID.String, State: text(row.ApplyRunState.Valid, row.ApplyRunState.String)}
		}
		if row.RollbackRunID.Valid {
			p.RollbackRun = &RunRef{RunID: row.RollbackRunID.String}
		}
		out = append(out, p)
	}
	return out, nil
}

// timestamptz is a bound as the statements take it, null for no bound.
func timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
