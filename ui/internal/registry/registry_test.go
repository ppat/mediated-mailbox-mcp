package registry_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// lookups are the sender classifier's domain functions, as the composition root passes them.
var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// TestTheRegistryHasItsStatements is the statement-set check over the real registry, which must be
// clean (ADR-0066).
func TestTheRegistryHasItsStatements(t *testing.T) {
	if problems := registry.Check(registry.Datasets(lookups)); len(problems) > 0 {
		t.Fatalf("the registry's statements do not match its declarations:\n%v", problems)
	}
}

func summary(context.Context, registry.Queries, registry.Read) ([]registry.Figure, registry.Total, error) {
	return nil, registry.Total{}, nil
}

func detail(context.Context, registry.Queries, string, lens.RowRequest, time.Time) (any, error) {
	return nil, nil
}

func rows(context.Context, registry.Queries, registry.Read) (any, error) { return nil, nil }

// TestTheStatementSetCheckReportsEachMismatch hands the check entries that break it each way, a
// dataset added without its statements, a groupable dimension added without its aggregate, an
// aggregate for a dimension that is not groupable, one for a dimension that is not declared, a row
// identity without a provenance query and a provenance query without a row identity, a dimension named
// as a common parameter and one named as the parent filter, and a dataset declared twice, and requires
// exactly those reported (VERIFICATIONS, the statement-set row).
// The entries are declared here because each is a registry the build must refuse, which no checked-in
// registry may be.
func TestTheStatementSetCheckReportsEachMismatch(t *testing.T) {
	violations := []registry.Dataset{
		{Descriptor: lens.Descriptor{Name: "bare"}},
		{
			Descriptor: lens.Descriptor{Name: "grouped", Dimensions: []lens.Dimension{
				{Name: "rule", Groupable: true}, {Name: "tier", Groupable: true}, {Name: "status", Filterable: true},
			}},
			Summary: summary, Rows: rows,
			Aggregates: map[string]registry.Aggregate{"rule": rows, "status": rows, "sender": rows},
		},
		{
			Descriptor: lens.Descriptor{Name: "detailless", Identity: &lens.RowIdentity{Name: "seq", Storage: "number"}},
			Summary:    summary, Rows: rows,
		},
		{Descriptor: lens.Descriptor{Name: "identityless"}, Summary: summary, Rows: rows, Detail: detail},
		{
			Descriptor: lens.Descriptor{Name: "clashing", Parent: "run", Dimensions: []lens.Dimension{
				{Name: "page", Filterable: true}, {Name: "run", Filterable: true}, {Name: "level", Sortable: true},
			}},
			Summary: summary, Rows: rows,
		},
		{Descriptor: lens.Descriptor{Name: "bare"}, Summary: summary, Rows: rows},
	}
	want := []string{
		"bare: no summary statement",
		"bare: no rows statement",
		"grouped: the groupable dimension tier has no aggregate statement",
		"grouped: an aggregate statement for sender, which is not a groupable dimension",
		"grouped: an aggregate statement for status, which is not a groupable dimension",
		"detailless: a row identity with no provenance query",
		"identityless: a provenance query with no row identity",
		"clashing: the dimension page is named as a parameter",
		"clashing: the dimension run is named as a parameter",
		"clashing: the dimension level is named as a parameter",
		"bare: declared twice",
	}
	if d := cmp.Diff(want, registry.Check(violations)); d != "" {
		t.Fatalf("(-want +got):\n%s", d)
	}
}

// A candidate's stored signals are read entry by entry, and nothing stored is refused or dropped: a key
// of the wrong type is null, an entry without a string identifier has an empty one, an identifier
// outside the five is sent as stored, and a value that is not a list is one entry (docs/UI.md section
// 17.1).
func TestStoredSignalsAreReadEntryByEntry(t *testing.T) {
	text := func(s string) *string { return &s }
	score := 0.87
	cases := []struct {
		stored string
		want   []registry.Signal
	}{
		{`[]`, []registry.Signal{}},
		{
			`[{"heuristic": "display_name", "evidence": {"name": "Chase", "domain": "chase.com"}}, {"heuristic": "embedding_similarity", "evidence": {"domain": "chase.com", "score": 0.87}}]`,
			[]registry.Signal{
				{Heuristic: "display_name", Evidence: registry.Evidence{Name: text("Chase"), Domain: text("chase.com")}},
				{Heuristic: "embedding_similarity", Evidence: registry.Evidence{Domain: text("chase.com"), Score: &score}},
			},
		},
		{
			`[{"heuristic": "institution_keyword", "evidence": {"keyword": 7, "score": "high"}}, {"heuristic": 3, "evidence": "x"}, "loose", {"heuristic": "a_new_one"}]`,
			[]registry.Signal{{Heuristic: "institution_keyword"}, {}, {}, {Heuristic: "a_new_one"}},
		},
		{`{"heuristic": "display_name"}`, []registry.Signal{{}}},
		{`null`, []registry.Signal{{}}},
	}
	for _, c := range cases {
		if diff := cmp.Diff(c.want, registry.Signals([]byte(c.stored))); diff != "" {
			t.Errorf("Signals(%s) (-want +got):\n%s", c.stored, diff)
		}
	}
}
