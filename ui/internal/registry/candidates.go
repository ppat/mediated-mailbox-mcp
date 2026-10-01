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
// 8.6). Signals is the recorded evidence, one entry per heuristic that fired, typed so no free-form
// JSON reaches a text position (ADR-0063), which the browser words by its templates. MessageCount and
// FirstSeen come from the sender statistics, null when the index holds no sender row for the domain.
type CandidateRow struct {
	Domain       string   `json:"domain"`
	Score        float64  `json:"score"`
	Signals      []Signal `json:"signals"`
	Status       string   `json:"status"`
	CreatedAt    string   `json:"created_at"`
	ReviewedAt   *string  `json:"reviewed_at"`
	ReviewedBy   *string  `json:"reviewed_by"`
	MessageCount *int64   `json:"message_count"`
	FirstSeen    *string  `json:"first_seen"`
}

func candidateRowType() schema.Type {
	return schema.Obj("CandidateRow",
		schema.F("domain", schema.Str()),
		schema.F("score", schema.Num()),
		schema.F("signals", schema.ArrayOf(signalType())),
		schema.F("status", schema.Str(candidateStatuses()...)),
		schema.F("created_at", schema.Time()),
		schema.F("reviewed_at", schema.Null(schema.Time())),
		schema.F("reviewed_by", schema.Null(schema.Str())),
		schema.F("message_count", schema.Null(schema.Int())),
		schema.F("first_seen", schema.Null(schema.Time())),
	)
}

// Signal is one recorded signal as the read API sends it, the heuristic's identifier as stored and its
// evidence keys, each null where the entry records none (ADR-0016, docs/UI.md section 17.1).
type Signal struct {
	Heuristic string   `json:"heuristic"`
	Evidence  Evidence `json:"evidence"`
}

// Evidence is a signal's evidence. Which keys a heuristic records is ADR-0016's.
type Evidence struct {
	Name    *string  `json:"name"`
	Domain  *string  `json:"domain"`
	Keyword *string  `json:"keyword"`
	Score   *float64 `json:"score"`
}

func signalType() schema.Type {
	text := schema.Null(schema.Str())
	return schema.Obj("Signal",
		schema.F("heuristic", schema.Str()),
		schema.F("evidence", schema.Obj("Evidence",
			schema.F("name", text),
			schema.F("domain", text),
			schema.F("keyword", text),
			schema.F("score", schema.Null(schema.Num())),
		)),
	)
}

// Signals reads a candidate's stored signals, each entry on its own, and never refuses one. A key of the
// wrong type is null, an entry with no string identifier has an empty one, and a stored value that is not
// a list is one entry with an empty identifier, so the screen shows that something it cannot word was
// recorded (docs/UI.md section 17.1). The identifier is sent as stored, known or not.
func Signals(stored []byte) []Signal {
	var entries []json.RawMessage
	// A stored JSON null decodes to no list, as anything else that is not one fails to decode.
	if json.Unmarshal(stored, &entries) != nil || entries == nil {
		return []Signal{{}}
	}
	out := make([]Signal, 0, len(entries))
	for _, raw := range entries {
		entry := decoded[map[string]json.RawMessage](raw)
		evidence := decoded[map[string]json.RawMessage](entry["evidence"])
		out = append(out, Signal{
			Heuristic: decoded[string](entry["heuristic"]),
			Evidence: Evidence{
				Name:    optional[string](evidence["name"]),
				Domain:  optional[string](evidence["domain"]),
				Keyword: optional[string](evidence["keyword"]),
				Score:   optional[float64](evidence["score"]),
			},
		})
	}
	return out
}

// optional is a JSON value of type T, nil when it is absent or of another type.
func optional[T any](raw json.RawMessage) *T {
	var v *T
	if raw == nil || json.Unmarshal(raw, &v) != nil {
		return nil
	}
	return v
}

// decoded is a JSON value of type T, its zero value when it is absent or of another type.
func decoded[T any](raw json.RawMessage) T {
	var zero T
	if v := optional[T](raw); v != nil {
		return *v
	}
	return zero
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

func candidateSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	in, out := statusFilter(r.Request)
	rows, err := q.Candidates.CandidateStatusCounts(ctx, policycandidates.CandidateStatusCountsParams{
		AccountID: r.Account, RangeStart: timestamptz(r.Bounds.Start), RangeEnd: timestamptz(r.Bounds.End), StatusIn: in, StatusOut: out,
	})
	if err != nil {
		return nil, Total{}, err
	}
	counts := map[string]int64{}
	for _, row := range rows {
		counts[row.Status] = row.Candidates
	}
	figures, total := statusFigures(r.Account, "candidates", candidateStatuses(), candidateWording(), counts)
	return figures, Total{Count: total}, nil
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
			Signals:    Signals(row.Signals),
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
