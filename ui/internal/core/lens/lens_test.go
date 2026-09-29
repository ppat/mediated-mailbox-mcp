package lens_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
)

// catalogue is a registry's declarative half written out for the tests, one ranged dataset with a
// groupable dimension and one nested dataset with a parent, so every rule has something to refuse.
func catalogue() []lens.Descriptor {
	return []lens.Descriptor{
		{
			Name: "events", Ranged: true, RangeColumn: "at",
			Dimensions: []lens.Dimension{
				{Name: "rule", Groupable: true, Filterable: true},
				{Name: "status", Filterable: true},
				{Name: "at", Sortable: true},
				{Name: "secret"},
			},
			Default: lens.Defaults{Group: "rule", Level: lens.Distribution, Range: "7d", Sort: lens.Sort{Column: "at", Descending: true}},
		},
		{
			Name: "items", Parent: "run",
			Dimensions: []lens.Dimension{{Name: "kind", Filterable: true, Groupable: true}, {Name: "seq", Sortable: true}},
			Default:    lens.Defaults{Level: lens.Rows, Sort: lens.Sort{Column: "seq"}},
		},
	}
}

func query(pairs ...string) map[string][]string {
	q := map[string][]string{}
	for i := 0; i+1 < len(pairs); i += 2 {
		q[pairs[i]] = append(q[pairs[i]], pairs[i+1])
	}
	return q
}

// TestParseAdmitsWhatTheEntryDeclares covers the grammar of docs/UI.md section 5 on requests the
// entry allows, each compared with the literal request it must produce.
func TestParseAdmitsWhatTheEntryDeclares(t *testing.T) {
	cases := []struct {
		name  string
		query map[string][]string
		want  lens.Request
	}{
		{
			name:  "defaults fill what is left out, filters excepted",
			query: query("dataset", "events"),
			want:  lens.Request{Dataset: "events", Level: 1, Group: "rule", Range: lens.Range{Preset: "7d"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1},
		},
		{
			name:  "level 0 ignores group, and every filter form parses",
			query: query("dataset", "events", "level", "0", "group", "rule", "rule", "a,b", "status", "!done", "range", "2026-09-01,2026-09-10"),
			want: lens.Request{
				Dataset: "events", Level: 0, Range: lens.Range{From: "2026-09-01", To: "2026-09-10"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1,
				Filters: []lens.Filter{{Dimension: "rule", Values: []string{"a", "b"}}, {Dimension: "status", Values: []string{"done"}, Exclude: true}},
			},
		},
		{
			name:  "level 2 with a group and a filter, a sort and a page",
			query: query("dataset", "events", "level", "2", "group", "rule", "status", "open", "sort", "at,asc", "page", "3", "range", "all"),
			want: lens.Request{
				Dataset: "events", Level: 2, Group: "rule", Range: lens.Range{Preset: "all"}, Sort: lens.Sort{Column: "at"}, Page: 3,
				Filters: []lens.Filter{{Dimension: "status", Values: []string{"open"}}},
			},
		},
		{
			name:  "a nested dataset takes its parent and no range",
			query: query("dataset", "items", "run", "r-1", "kind", "page"),
			want: lens.Request{
				Dataset: "items", Level: 3, ParentID: "r-1", Sort: lens.Sort{Column: "seq"}, Page: 1,
				Filters: []lens.Filter{{Dimension: "kind", Values: []string{"page"}}},
			},
		},
		{
			name:  "a leap day is a date",
			query: query("dataset", "events", "level", "3", "range", "2024-02-29,2024-02-29"),
			want:  lens.Request{Dataset: "events", Level: 3, Range: lens.Range{From: "2024-02-29", To: "2024-02-29"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := lens.Parse(catalogue(), c.query)
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("(-want +got):\n%s", d)
			}
		})
	}
}

// TestParseRefusesWhatTheEntryDoesNotDeclare is the registry's refusal, each case naming the error
// contract's code it must carry.
func TestParseRefusesWhatTheEntryDoesNotDeclare(t *testing.T) {
	cases := []struct {
		name  string
		query map[string][]string
		code  string
	}{
		{"no dataset", query("level", "0"), "missing_dataset"},
		{"an undeclared dataset", query("dataset", "messages"), "unknown_dataset"},
		{"an undeclared filter", query("dataset", "events", "sender", "x"), "unknown_dimension"},
		{"a declared dimension that is not filterable", query("dataset", "events", "secret", "x"), "unknown_dimension"},
		{"a sortable column used as a filter", query("dataset", "events", "at", "x"), "unknown_dimension"},
		{"an undeclared sort column", query("dataset", "events", "sort", "secret,asc"), "unknown_sort"},
		{"a sort without a direction", query("dataset", "events", "sort", "at"), "invalid_sort"},
		{"a sort with another direction", query("dataset", "events", "sort", "at,up"), "invalid_sort"},
		{"an undeclared group", query("dataset", "events", "level", "1", "group", "status"), "unknown_group"},
		{"an undeclared group at level 0", query("dataset", "events", "level", "0", "group", "nothing"), "unknown_group"},
		{"an undeclared group at level 3", query("dataset", "events", "level", "3", "group", "status"), "unknown_group"},
		{"level 1 with no group", query("dataset", "items", "run", "r", "level", "1"), "missing_group"},
		{"level 2 with no filter", query("dataset", "events", "level", "2"), "missing_filter"},
		{"a level off the ladder", query("dataset", "events", "level", "4"), "invalid_level"},
		{"a negative level", query("dataset", "events", "level", "-1"), "invalid_level"},
		{"a level that is not a number", query("dataset", "events", "level", "one"), "invalid_level"},
		{"a page of zero", query("dataset", "events", "page", "0"), "invalid_page"},
		{"a page with a sign", query("dataset", "events", "page", "+2"), "invalid_page"},
		{"a repeated parameter", query("dataset", "events", "rule", "a", "rule", "b"), "repeated_parameter"},
		{"a range on a dataset without one", query("dataset", "items", "run", "r", "range", "7d"), "unknown_dimension"},
		{"an unknown preset", query("dataset", "events", "range", "1y"), "invalid_range"},
		{"a range ending before it starts", query("dataset", "events", "range", "2026-09-10,2026-09-01"), "invalid_range"},
		{"a day past the month's end", query("dataset", "events", "range", "2026-02-29,2026-03-01"), "invalid_range"},
		{"a date with a sign", query("dataset", "events", "range", "+202-09-01,2026-09-02"), "invalid_range"},
		{"a nested dataset without its parent", query("dataset", "items"), "missing_parent"},
		{"an empty any-of member", query("dataset", "events", "rule", "a,"), "invalid_filter"},
		{"an exclusion of several values", query("dataset", "events", "rule", "!a,b"), "invalid_filter"},
		{"an empty exclusion", query("dataset", "events", "rule", "!"), "invalid_filter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := lens.Parse(catalogue(), c.query)
			var refusal *lens.Refusal
			if !errors.As(err, &refusal) {
				t.Fatalf("the request was not refused: %v", err)
			}
			if refusal.Code != c.code {
				t.Fatalf("refused as %s (%s), want %s", refusal.Code, refusal.Message, c.code)
			}
		})
	}
}
