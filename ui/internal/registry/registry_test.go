package registry_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// TestTheRegistryHasItsStatements is the statement-set check over the real registry, which must be
// clean (ADR-0066).
func TestTheRegistryHasItsStatements(t *testing.T) {
	if problems := registry.Check(registry.Datasets()); len(problems) > 0 {
		t.Fatalf("the registry's statements do not match its declarations:\n%v", problems)
	}
}

func summary(context.Context, registry.Queries, registry.Read) ([]registry.Figure, int64, error) {
	return nil, 0, nil
}

func rows(context.Context, registry.Queries, registry.Read) (any, error) { return nil, nil }

// TestTheStatementSetCheckReportsEachMismatch hands the check entries that break it each way, a
// dataset added without its statements, a groupable dimension added without its aggregate, an
// aggregate for a dimension that is not groupable, one for a dimension that is not declared, and a
// dataset declared twice, and requires exactly those reported (VERIFICATIONS, the statement-set row).
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
		{Descriptor: lens.Descriptor{Name: "bare"}, Summary: summary, Rows: rows},
	}
	want := []string{
		"bare: no summary statement",
		"bare: no rows statement",
		"grouped: the groupable dimension tier has no aggregate statement",
		"grouped: an aggregate statement for sender, which is not a groupable dimension",
		"grouped: an aggregate statement for status, which is not a groupable dimension",
		"bare: declared twice",
	}
	if d := cmp.Diff(want, registry.Check(violations)); d != "" {
		t.Fatalf("(-want +got):\n%s", d)
	}
}
