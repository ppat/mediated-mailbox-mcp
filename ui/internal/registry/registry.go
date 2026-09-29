package registry

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// LensPath is the dataset endpoint's route, the registry's one path in the contract (docs/UI.md
// section 17.1). A bespoke handler claiming it is refused by the contract generator.
const LensPath = "/api/{account}/lens"

// Bounds is a request's range as instants, Start inclusive and End exclusive, nil for no bound.
type Bounds struct {
	Start *time.Time
	End   *time.Time
}

// Read is one dataset read, for one account, inside a transaction that set it.
type Read struct {
	Account string
	Request lens.Request
	Bounds  Bounds
}

// Figure is one L0 figure (docs/UI.md section 17.1).
type Figure struct {
	Key     string  `json:"key"`
	Wording string  `json:"wording"`
	Value   int64   `json:"value"`
	Unit    *string `json:"unit"`
	Link    string  `json:"link"`
}

// FigureType is Figure's declaration.
func FigureType() schema.Type {
	return schema.Obj("Figure",
		schema.F("key", schema.Str()),
		schema.F("wording", schema.Str()),
		schema.F("value", schema.Int()),
		schema.F("unit", schema.Null(schema.Str())),
		schema.F("link", schema.Str()),
	)
}

// Queries are the statements the registry's datasets run. The caller builds them inside the function
// literal it passes the transaction helper, from the transaction that set the account (ADR-0047).
type Queries struct {
	Plans      *reorgplans.Queries
	Candidates *policycandidates.Queries
}

// Summary returns a read's L0 figures and the count of rows the read's filters match, which the row
// pages count from.
type Summary func(ctx context.Context, q Queries, r Read) (figures []Figure, count int64, err error)

// RowPage returns a read's page of rows, a slice of the dataset's row type.
type RowPage func(ctx context.Context, q Queries, r Read) (rows any, err error)

// Aggregate returns a read's groups at levels 1 and 2 for one groupable dimension.
type Aggregate func(ctx context.Context, q Queries, r Read) (rows any, err error)

// Dataset is one registry entry (docs/UI.md section 17.2). The declarative half is the descriptor
// and the row type, from which the contract is generated. The rest are the statements that serve it,
// enumerated rather than composed (ADR-0066), one per groupable dimension plus a summary and a rows
// statement, each pointing at a generated data-access accessor.
type Dataset struct {
	lens.Descriptor
	Row        schema.Type
	Summary    Summary
	Rows       RowPage
	Aggregates map[string]Aggregate
}

// Datasets is the registry, the only source of what a lens can ask for (ADR-0057). Each dataset a
// screen reads is one entry here, and nothing else is a dataset.
func Datasets() []Dataset {
	return []Dataset{plans(), candidates()}
}

// Descriptors returns the declarative half of every entry, which the dataset endpoint's pure core
// checks each request against.
func Descriptors(datasets []Dataset) []lens.Descriptor {
	out := make([]lens.Descriptor, len(datasets))
	for i, d := range datasets {
		out[i] = d.Descriptor
	}
	return out
}

// Check reports every dataset whose statements do not match what it declares, counted rather than
// read: a summary and a rows statement for every dataset, one aggregate statement per groupable
// dimension, and none for a dimension that is not groupable or not declared (ADR-0066). It is the
// statement-set check, and the registry's test requires it clean.
func Check(datasets []Dataset) []string {
	var problems []string
	seen := map[string]bool{}
	for _, d := range datasets {
		if seen[d.Name] {
			problems = append(problems, fmt.Sprintf("%s: declared twice", d.Name))
		}
		seen[d.Name] = true
		if d.Summary == nil {
			problems = append(problems, fmt.Sprintf("%s: no summary statement", d.Name))
		}
		if d.Rows == nil {
			problems = append(problems, fmt.Sprintf("%s: no rows statement", d.Name))
		}
		for _, dim := range d.Dimensions {
			if dim.Groupable && d.Aggregates[dim.Name] == nil {
				problems = append(problems, fmt.Sprintf("%s: the groupable dimension %s has no aggregate statement", d.Name, dim.Name))
			}
		}
		var names []string
		for name := range d.Aggregates {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			if dim, ok := d.Dimension(name); !ok || !dim.Groupable {
				problems = append(problems, fmt.Sprintf("%s: an aggregate statement for %s, which is not a groupable dimension", d.Name, name))
			}
		}
	}
	return problems
}

// Pages is the number of row pages count rows fill, at least one.
func Pages(count int64) int64 {
	return max(1, int64(math.Ceil(float64(count)/lens.RowsPerPage)))
}

// link is a screen URL under the account, with its query.
func link(account, screen string, query url.Values) string {
	u := url.URL{Path: "/" + account + "/" + screen, RawQuery: query.Encode()}
	return u.String()
}

// statusFilter splits a request's status filter into the any-of and exclusion arrays the statements
// take, nil for no filter.
func statusFilter(r lens.Request) (in, out []string) {
	f, ok := r.Filter("status")
	if !ok {
		return nil, nil
	}
	if f.Exclude {
		return nil, f.Values
	}
	return f.Values, nil
}

// statusFigures turns counts per status into one figure per declared status, in declared order, with
// the wording docs/UI.md section 17.1 gives each dataset's figures and a link to the screen filtered by
// that status.
func statusFigures(account, screen string, statuses, wording []string, counts map[string]int64) ([]Figure, int64) {
	var total int64
	figures := make([]Figure, 0, len(statuses))
	for i, s := range statuses {
		n := counts[s]
		total += n
		figures = append(figures, Figure{Key: s, Wording: wording[i], Value: n, Link: link(account, screen, url.Values{"status": {s}})})
	}
	var unknown []string
	for s := range counts {
		if !slices.Contains(statuses, s) {
			unknown = append(unknown, s)
		}
	}
	slices.Sort(unknown)
	for _, s := range unknown {
		// A stored status the design does not know is shown, never dropped (docs/UI.md section 7.1).
		total += counts[s]
		figures = append(figures, Figure{Key: s, Wording: s, Value: counts[s], Link: link(account, screen, url.Values{"status": {s}})})
	}
	return figures, total
}

// offset is the first row of a request's page, and false for a page so far past the end that its
// offset does not fit the statement's parameter, which holds no rows.
func offset(r lens.Request) (int32, bool) {
	first := int64(r.Page-1) * lens.RowsPerPage
	if first > math.MaxInt32 {
		return 0, false
	}
	return int32(first), true //nolint:gosec // first is at most math.MaxInt32, checked above
}

// Stamp is an instant as the read API writes it, RFC 3339 in UTC with the Z suffix, to the second
// (docs/UI.md section 5).
func Stamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// stamp is a stored instant as the read API writes it. A column the schema declares NOT NULL is
// always valid.
func stamp(t pgtype.Timestamptz) string {
	return Stamp(t.Time)
}

// optionalStamp is a nullable stored instant, nil when null.
func optionalStamp(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	s := Stamp(t.Time)
	return &s
}

// text returns s, or nil when it is not valid.
func text(valid bool, s string) *string {
	if !valid {
		return nil
	}
	return &s
}
