package registry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/url"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	senderclasses "github.com/ppat/mediated-mailbox-mcp/db/senders/classification"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// LensPath is the dataset endpoint's route, the registry's one path in the contract (docs/UI.md
// section 17.1). A bespoke handler claiming it is refused by the contract generator.
const LensPath = "/api/{account}/lens"

// RowPath is the row-detail endpoint's route for a dataset that declares a provenance query, one per
// such dataset (docs/UI.md section 17.1). The registry claims it, and the contract generator refuses a
// bespoke handler claiming it too.
func RowPath(dataset string) string {
	return "/api/{account}/" + dataset + "/{row}"
}

// Paths are the routes the registry claims, the dataset endpoint's and one row-detail route per dataset
// that declares a provenance query.
func Paths(datasets []Dataset) []string {
	paths := []string{LensPath}
	for _, d := range datasets {
		if d.Detail != nil {
			paths = append(paths, RowPath(d.Name))
		}
	}
	return paths
}

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
	// AsOf is when the read transaction began, which a running run's duration runs to.
	AsOf time.Time
}

// Figure is one L0 figure (docs/UI.md section 17.1), a count in Value or a time in At, the other
// null, and both null for a time with nothing to show.
type Figure struct {
	Key     string  `json:"key"`
	Wording string  `json:"wording"`
	Value   *int64  `json:"value"`
	At      *string `json:"at"`
	Unit    *string `json:"unit"`
	Link    string  `json:"link"`
}

// count is a figure that is a count.
func count(key, wording string, n int64, link string) Figure {
	return Figure{Key: key, Wording: wording, Value: &n, Link: link}
}

// FigureType is Figure's declaration.
func FigureType() schema.Type {
	return schema.Obj("Figure",
		schema.F("key", schema.Str()),
		schema.F("wording", schema.Str()),
		schema.F("value", schema.Null(schema.Int())),
		schema.F("at", schema.Null(schema.Time())),
		schema.F("unit", schema.Null(schema.Str())),
		schema.F("link", schema.Str()),
	)
}

// Queries are the statements the registry's datasets run. The caller builds them inside the function
// literal it passes the transaction helper, from the transaction that set the account (ADR-0047).
type Queries struct {
	Plans      *reorgplans.Queries
	Candidates *policycandidates.Queries
	Runs       *jobruns.Queries
	Failures   *classification.Queries
	Audit      *auditlog.Queries
	// Rules, Changes and Senders are policy management's reads, and Accounts the accounts listing a
	// base rule's screen names (ADR-0091).
	Rules    *manage.Queries
	Changes  *policychanges.Queries
	Senders  *senderclasses.Queries
	Accounts *accounts.Queries
}

// Total is the count of rows a read's filters match, which the row pages count from, and for a
// message-derived dataset how many of them are restricted and how many flagged (docs/UI.md section
// 17.1).
type Total struct {
	Count      int64
	Restricted int64
	Flagged    int64
}

// Summary returns a read's L0 figures and its total.
type Summary func(ctx context.Context, q Queries, r Read) (figures []Figure, total Total, err error)

// RowPage returns a read's page of rows, a slice of the dataset's row type.
type RowPage func(ctx context.Context, q Queries, r Read) (rows any, err error)

// Aggregate returns a read's groups at levels 1 and 2 for one groupable dimension, every group,
// ordered by count descending and then by key with null last (docs/UI.md section 17.1). The groups are
// a []Group, or a []SensitiveGroup for a message-derived dataset.
type Aggregate func(ctx context.Context, q Queries, r Read) (rows any, err error)

// Group is one group of a dataset that is not message-derived. Key holds the grouped dimension's value
// as stored, nil for its null group.
type Group struct {
	Key   map[string]any `json:"key"`
	Count int64          `json:"count"`
}

// SensitiveGroup is one group of a message-derived dataset, with how many of its rows are restricted
// and how many flagged.
type SensitiveGroup struct {
	Key        map[string]any `json:"key"`
	Count      int64          `json:"count"`
	Restricted int64          `json:"restricted"`
	Flagged    int64          `json:"flagged"`
}

// Detail returns one row with its provenance, the row-detail endpoint's answer, or ErrNoRow when the
// account holds no such row under the parent.
type Detail func(ctx context.Context, q Queries, account string, req lens.RowRequest, asOf time.Time) (detail any, err error)

// ErrNoRow is a row detail for a row the account does not hold.
var ErrNoRow = errors.New("the account holds no such row")

// Dataset is one registry entry (docs/UI.md section 17.2). The declarative half is the descriptor
// and the row type, from which the contract is generated. The rest are the statements that serve it,
// enumerated rather than composed (ADR-0066), one per groupable dimension plus a summary and a rows
// statement, each pointing at a generated data-access accessor.
type Dataset struct {
	lens.Descriptor
	// MessageDerived says the dataset's rows carry a message, so its totals and groups also count the
	// restricted and the flagged rows.
	MessageDerived bool
	Row            schema.Type
	Summary        Summary
	Rows           RowPage
	Aggregates     map[string]Aggregate
	// Detail is the provenance query, with DetailType its answer's declaration. A dataset declares one
	// together with its row identity, or neither.
	Detail     Detail
	DetailType schema.Type
}

// Datasets is the registry, the only source of what a lens can ask for (ADR-0057). Each dataset a
// screen reads is one entry here, and nothing else is a dataset. l is the lookups the sender classifier
// normalizes domains with, which the policy datasets match senders against rules with.
func Datasets(l classify.Lookups) []Dataset {
	return []Dataset{plans(), candidates(), runs(), failures(), policyRules(l), policyChanges(), senders(l)}
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
// dimension, none for a dimension that is not groupable or not declared, and a provenance query exactly
// where a row identity is declared (ADR-0066). It also reports a dimension named as a common parameter
// or as the dataset's parent filter, which a URL could not tell apart from it. It is the statement-set
// check, and the registry's test requires it clean.
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
		if d.Identity != nil && d.Detail == nil {
			problems = append(problems, fmt.Sprintf("%s: a row identity with no provenance query", d.Name))
		}
		if d.Identity == nil && d.Detail != nil {
			problems = append(problems, fmt.Sprintf("%s: a provenance query with no row identity", d.Name))
		}
		for _, dim := range d.Dimensions {
			if slices.Contains(lens.Parameters(), dim.Name) || dim.Name == d.Parent {
				problems = append(problems, fmt.Sprintf("%s: the dimension %s is named as a parameter", d.Name, dim.Name))
			}
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
		figures = append(figures, count(s, wording[i], n, link(account, screen, url.Values{"status": {s}})))
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
		figures = append(figures, count(s, s, counts[s], link(account, screen, url.Values{"status": {s}})))
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
