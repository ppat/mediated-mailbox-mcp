package registry

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// candidateStatuses are the candidate statuses as stored (ADR-0004), and candidateWording their
// figures' wording (docs/UI.md section 17.1), section 11's apart from confirmed, since a count holds
// candidates in effect and not yet.
func candidateStatuses() []string { return []string{"pending", "confirmed", "dismissed"} }

func candidateWording() []string { return []string{"Awaiting review", "Confirmed", "Dismissed"} }

// CandidateRow is one row of the candidates dataset, the review queue's columns (docs/UI.md section
// 8.6). Signals is the recorded evidence, one entry per heuristic that fired, which the browser words
// by its templates. MessageCount and FirstSeen come from the sender statistics, null when the index
// holds no sender row for the domain.
type CandidateRow struct {
	Domain       string          `json:"domain"`
	Score        float64         `json:"score"`
	Signals      json.RawMessage `json:"signals"`
	Status       string          `json:"status"`
	CreatedAt    string          `json:"created_at"`
	ReviewedAt   *string         `json:"reviewed_at"`
	ReviewedBy   *string         `json:"reviewed_by"`
	MessageCount *int64          `json:"message_count"`
	FirstSeen    *string         `json:"first_seen"`
}

func candidateRowType() schema.Type {
	return schema.Obj("CandidateRow",
		schema.F("domain", schema.Str()),
		schema.F("score", schema.Num()),
		schema.F("signals", schema.Any()),
		schema.F("status", schema.Str(candidateStatuses()...)),
		schema.F("created_at", schema.Time()),
		schema.F("reviewed_at", schema.Null(schema.Time())),
		schema.F("reviewed_by", schema.Null(schema.Str())),
		schema.F("message_count", schema.Null(schema.Int())),
		schema.F("first_seen", schema.Null(schema.Time())),
	)
}

// candidates is the review queue's policy candidates, which the review queue lists and Home's
// decision inbox filters to pending (docs/UI.md sections 8.1 and 8.6). Nothing is groupable, so it has
// a summary and a rows statement only. Its default status filter is pending, which the browser's
// router fills.
func candidates() Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:        "candidates",
			Ranged:      true,
			RangeColumn: "created_at",
			Dimensions: []lens.Dimension{
				{Name: "status", Storage: "text", Filterable: true, Wording: "status", Values: candidateStatuses()},
				{Name: "score", Storage: "number", Sortable: true, Wording: "score"},
				{Name: "created_at", Storage: "time", Sortable: true, Wording: "time in queue"},
			},
			Default: lens.Defaults{
				Level: lens.Rows, Range: "all", Sort: lens.Sort{Column: "score", Descending: true},
				Filters: map[string][]string{"status": {"pending"}},
			},
		},
		Row:     candidateRowType(),
		Summary: candidateSummary,
		Rows:    candidateRows,
	}
}

func candidateSummary(ctx context.Context, q Queries, r Read) ([]Figure, int64, error) {
	in, out := statusFilter(r.Request)
	rows, err := q.Candidates.CandidateStatusCounts(ctx, policycandidates.CandidateStatusCountsParams{
		AccountID: r.Account, RangeStart: timestamptz(r.Bounds.Start), RangeEnd: timestamptz(r.Bounds.End), StatusIn: in, StatusOut: out,
	})
	if err != nil {
		return nil, 0, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Status] = row.Candidates
	}
	figures, total := statusFigures(r.Account, "candidates", candidateStatuses(), candidateWording(), counts)
	return figures, total, nil
}

func candidateRows(ctx context.Context, q Queries, r Read) (any, error) {
	out := []CandidateRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	in, ex := statusFilter(r.Request)
	rows, err := q.Candidates.CandidateRows(ctx, policycandidates.CandidateRowsParams{
		AccountID: r.Account, RangeStart: timestamptz(r.Bounds.Start), RangeEnd: timestamptz(r.Bounds.End), StatusIn: in, StatusOut: ex,
		SortScore: r.Request.Sort.Column == "score", Descending: r.Request.Sort.Descending, RowOffset: first,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		c := CandidateRow{
			Domain:     row.Domain,
			Score:      Widen(row.Score),
			Signals:    json.RawMessage(row.Signals),
			Status:     row.Status,
			CreatedAt:  stamp(row.CreatedAt),
			ReviewedAt: optionalStamp(row.ReviewedAt),
			ReviewedBy: text(row.ReviewedBy.Valid, row.ReviewedBy.String),
			FirstSeen:  optionalStamp(row.FirstSeen),
		}
		if row.MessageCount.Valid {
			n := row.MessageCount.Int64
			c.MessageCount = &n
		}
		out = append(out, c)
	}
	return out, nil
}

// Widen widens a stored real to the float64 JSON writes, through its shortest decimal form, so 0.9
// arrives as 0.9 and not as the binary neighbour a plain conversion prints.
func Widen(f float32) float64 {
	v, err := strconv.ParseFloat(strconv.FormatFloat(float64(f), 'g', -1, 32), 64)
	if err != nil {
		return float64(f)
	}
	return v
}
