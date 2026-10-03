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
				{Name: "day", Storage: "date", Groupable: true, Filterable: true},
				{Name: "page_number", Storage: "number", Groupable: true, Filterable: true, NullWording: "no page"},
				{Name: "tier", Storage: "number", Filterable: true},
				{Name: "domain", Storage: "text", Groupable: true, Filterable: true, NullWording: "no domain", Empty: true},
				{Name: "search", Storage: "text", Filterable: true, Search: true},
			},
			Default: lens.Defaults{Group: "rule", Level: lens.Distribution, Range: "7d", Sort: lens.Sort{Column: "at", Descending: true}},
		},
		{
			Name: "items", Parent: "run",
			Dimensions: []lens.Dimension{{Name: "kind", Filterable: true, Groupable: true}, {Name: "seq", Sortable: true}},
			Default:    lens.Defaults{Level: lens.Rows, Sort: lens.Sort{Column: "seq"}},
			Identity:   &lens.RowIdentity{Name: "seq", Storage: "number"},
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
			name:  "a number, a date and the null group of a dimension that has one",
			query: query("dataset", "events", "level", "3", "page_number", "3,none", "day", "!2026-09-10", "tier", "0"),
			want: lens.Request{
				Dataset: "events", Level: 3, Range: lens.Range{Preset: "7d"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1,
				Filters: []lens.Filter{
					{Dimension: "day", Values: []string{"2026-09-10"}, Exclude: true},
					{Dimension: "page_number", Values: []string{"3", "none"}},
					{Dimension: "tier", Values: []string{"0"}},
				},
			},
		},
		{
			name:  "the group of a value stored empty and the null group of a dimension that holds both",
			query: query("dataset", "events", "level", "3", "domain", "example.test,empty,none"),
			want: lens.Request{
				Dataset: "events", Level: 3, Range: lens.Range{Preset: "7d"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1,
				Filters: []lens.Filter{{Dimension: "domain", Values: []string{"example.test", "empty", "none"}}},
			},
		},
		{
			name:  "the group of a value stored empty excluded",
			query: query("dataset", "events", "level", "3", "domain", "!empty"),
			want: lens.Request{
				Dataset: "events", Level: 3, Range: lens.Range{Preset: "7d"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1,
				Filters: []lens.Filter{{Dimension: "domain", Values: []string{"empty"}, Exclude: true}},
			},
		},
		{
			name:  "a search is taken whole, a comma, a leading ! and the null and empty words included",
			query: query("dataset", "events", "level", "3", "search", "!none,empty"),
			want: lens.Request{
				Dataset: "events", Level: 3, Range: lens.Range{Preset: "7d"}, Sort: lens.Sort{Column: "at", Descending: true}, Page: 1,
				Filters: []lens.Filter{{Dimension: "search", Values: []string{"!none,empty"}}},
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
		{"a number filter holding a word", query("dataset", "events", "page_number", "three"), "invalid_filter"},
		{"a number filter holding a sign", query("dataset", "events", "page_number", "-3"), "invalid_filter"},
		{"a number filter with a leading zero", query("dataset", "events", "page_number", "03"), "invalid_filter"},
		{"a number filter past a stored integer", query("dataset", "events", "page_number", "2147483648"), "invalid_filter"},
		{"a number exclusion holding a word", query("dataset", "events", "page_number", "!x"), "invalid_filter"},
		{"none on a dimension with no null group", query("dataset", "events", "tier", "none"), "invalid_filter"},
		{"none on a text dimension with no null group", query("dataset", "events", "status", "none"), "invalid_filter"},
		{"none excluded on a text dimension with no null group", query("dataset", "events", "rule", "!none"), "invalid_filter"},
		{"empty on a text dimension that holds no value stored empty", query("dataset", "events", "status", "empty"), "invalid_filter"},
		{"empty excluded on a text dimension that holds no value stored empty", query("dataset", "events", "rule", "!empty"), "invalid_filter"},
		{"empty on a number dimension", query("dataset", "events", "tier", "empty"), "invalid_filter"},
		{"a date filter holding a time", query("dataset", "events", "day", "2026-09-10T00:00:00Z"), "invalid_filter"},
		{"a date filter past the month's end", query("dataset", "events", "day", "2026-02-30"), "invalid_filter"},
		{"an empty search", query("dataset", "events", "search", ""), "invalid_filter"},
		{"a date any-of with one bad member", query("dataset", "events", "day", "2026-09-10,yesterday"), "invalid_filter"},
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

// TestParseRowAdmitsTheRowAndItsParent covers the row-detail request a nested dataset allows, a row as
// large as a big serial holds among them.
func TestParseRowAdmitsTheRowAndItsParent(t *testing.T) {
	for _, row := range []string{"42", "0", "9223372036854775807"} {
		got, err := lens.ParseRow(catalogue(), "items", row, query("run", "r-1"))
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(lens.RowRequest{Dataset: "items", Row: row, ParentID: "r-1"}, got); d != "" {
			t.Fatalf("(-want +got):\n%s", d)
		}
	}
}

// TestParseRowRefusesWhatTheEntryDoesNotDeclare is the row-detail endpoint's refusal, each case naming
// the error contract's code it must carry.
func TestParseRowRefusesWhatTheEntryDoesNotDeclare(t *testing.T) {
	cases := []struct {
		name    string
		dataset string
		row     string
		query   map[string][]string
		code    string
	}{
		{"a dataset with no row detail", "events", "1", query(), "unknown_dataset"},
		{"an undeclared dataset", "messages", "1", query("run", "r"), "unknown_dataset"},
		{"a row that is not a number", "items", "one", query("run", "r"), "invalid_row"},
		{"a row with a sign", "items", "-1", query("run", "r"), "invalid_row"},
		{"an empty row", "items", "", query("run", "r"), "invalid_row"},
		{"a row past a big serial", "items", "9223372036854775808", query("run", "r"), "invalid_row"},
		{"a row with a leading zero", "items", "042", query("run", "r"), "invalid_row"},
		{"no parent", "items", "1", query(), "missing_parent"},
		{"an empty parent", "items", "1", query("run", ""), "invalid_filter"},
		{"a repeated parent", "items", "1", query("run", "a", "run", "b"), "repeated_parameter"},
		{"a filter beside the parent", "items", "1", query("run", "r", "kind", "page"), "unknown_parameter"},
		{"a level beside the parent", "items", "1", query("run", "r", "level", "4"), "unknown_parameter"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := lens.ParseRow(catalogue(), c.dataset, c.row, c.query)
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
