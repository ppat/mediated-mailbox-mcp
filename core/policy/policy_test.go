package policy_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// view is what a test can observe of one account's composed policy.
type view struct {
	RestrictsAll bool
	Rules        map[string][]string
	Order        []string
}

func observe(c policy.Composed) view {
	v := view{RestrictsAll: c.RestrictsAll(), Rules: map[string][]string{}}
	for _, r := range c.Rules() {
		v.Rules[r.ID()] = r.DomainSuffixes()
		v.Order = append(v.Order, r.ID())
	}
	return v
}

func rule(account, id string, suffixes ...string) policy.Row {
	return policy.Row{Account: account, ID: id, Class: policy.Restricted, DomainSuffixes: suffixes}
}

var valid = []policy.Row{
	rule("", "financial.brokerage.fidelity", "fidelity.com", "fmr.com"),
	rule("", "gov.federal.irs", "irs.gov"),
	rule("acct-a", "candidate.acct-a.examplebank.com", "examplebank.com"),
	rule("acct-b", "candidate.acct-b.xn--bcher-kva.example", "xn--bcher-kva.example"),
}

func load(t *testing.T, rows []policy.Row) policy.Snapshot {
	t.Helper()
	s, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Before any valid load there is no policy, and absence denies (ADR-0041).
func TestTheZeroSnapshotRestrictsEverySender(t *testing.T) {
	var s policy.Snapshot
	if s.Loaded() {
		t.Error("the zero Snapshot reports a valid load")
	}
	want := view{RestrictsAll: true, Rules: map[string][]string{}}
	for _, account := range []string{"", "acct-a"} {
		if diff := cmp.Diff(want, observe(s.For(account)), compare.Options); diff != "" {
			t.Errorf("zero Snapshot for %q (-want +got):\n%s", account, diff)
		}
	}
	if diff := cmp.Diff(want, observe(policy.Composed{}), compare.Options); diff != "" {
		t.Errorf("zero Composed (-want +got):\n%s", diff)
	}
}

// An account's policy is the base rules followed by its own overlay, and never another account's
// overlay (ADR-0026).
func TestForComposesTheBaseWithTheAccountsOverlay(t *testing.T) {
	s := load(t, valid)
	base := map[string][]string{
		"financial.brokerage.fidelity": {"fidelity.com", "fmr.com"},
		"gov.federal.irs":              {"irs.gov"},
	}
	cases := []struct {
		account string
		want    view
	}{
		{"acct-a", view{
			Rules: map[string][]string{
				"financial.brokerage.fidelity":     {"fidelity.com", "fmr.com"},
				"gov.federal.irs":                  {"irs.gov"},
				"candidate.acct-a.examplebank.com": {"examplebank.com"},
			},
			Order: []string{"financial.brokerage.fidelity", "gov.federal.irs", "candidate.acct-a.examplebank.com"},
		}},
		{"acct-c", view{Rules: base, Order: []string{"financial.brokerage.fidelity", "gov.federal.irs"}}},
	}
	for _, c := range cases {
		t.Run(c.account, func(t *testing.T) {
			if diff := cmp.Diff(c.want, observe(s.For(c.account)), compare.Options); diff != "" {
				t.Errorf("For(%q) (-want +got):\n%s", c.account, diff)
			}
		})
	}
}

func TestLoadRefusesInvalidRows(t *testing.T) {
	cases := []struct {
		name string
		row  policy.Row
		want string
	}{
		{"empty identifier", rule("", "", "a.example"), "has an empty identifier"},
		{"identifier with surrounding space", rule("", " a ", "a.example"), "has an empty identifier or one with surrounding space"},
		{"repeated identifier", rule("acct-a", "gov.federal.irs", "a.example"), "repeats an identifier"},
		{"class other than restricted", policy.Row{ID: "x", Class: "normal", DomainSuffixes: []string{"a.example"}}, `has the class "normal"`},
		{"no class", policy.Row{ID: "x", DomainSuffixes: []string{"a.example"}}, `has the class ""`},
		{"no domain suffix", rule("", "x"), "has no domain suffix"},
		{"leading dot", rule("", "x", ".example"), `".example", which is not a domain name`},
		{"empty label", rule("", "x", "a..example"), "which is not a domain name"},
		{"trailing dot", rule("", "x", "a.example."), "which is not a domain name"},
		{"space", rule("", "x", "a b.example"), "which is not a domain name"},
		{"control character", rule("", "x", "a\tb.example"), "which is not a domain name"},
		{"at sign", rule("", "x", "@fidelity.com"), "which is not a domain name"},
		{"slash", rule("", "x", "fidelity.com/"), "which is not a domain name"},
		{"wildcard", rule("", "x", "*.fidelity.com"), "which is not a domain name"},
		{"comma", rule("", "x", "fidelity,com"), "which is not a domain name"},
		{"empty suffix", rule("", "x", ""), "which is not a domain name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := policy.Load(append(append([]policy.Row{}, valid...), c.row))
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("Load returned %v, want an error containing %q", err, c.want)
			}
		})
	}
}

// Neither a policy of overlays alone nor one with no rules is invalid, and each loads as written.
// Rule suffixes are kept in whatever form the operator wrote, since normalizing is the classifier's.
func TestLoadAcceptsOverlaysOnlyAnEmptyPolicyAndAnyScript(t *testing.T) {
	for name, rows := range map[string][]policy.Row{
		"no rows":       nil,
		"overlays only": valid[2:],
		"any script":    {rule("", "x", "Bücher.example")},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := policy.Load(rows); err != nil {
				t.Fatalf("Load refused a valid policy: %v", err)
			}
		})
	}
}

// Every operation names its account (ADR-0026), so an empty name gets the policy that restricts
// every sender rather than the base policy alone.
func TestAnEmptyAccountNameRestrictsEverySender(t *testing.T) {
	want := view{RestrictsAll: true, Rules: map[string][]string{}}
	if diff := cmp.Diff(want, observe(load(t, valid).For("")), compare.Options); diff != "" {
		t.Errorf("For(\"\") (-want +got):\n%s", diff)
	}
}

func TestSwap(t *testing.T) {
	active := load(t, valid[:2])
	next := []policy.Row{rule("", "infra.vendor.cloudflare", "cloudflare.com")}
	invalid := []policy.Row{rule("", "infra.vendor.cloudflare", "cloud flare.com")}

	type outcome struct {
		Accepted bool
		Problems []string
		Policy   view
	}
	cases := []struct {
		name   string
		active policy.Snapshot
		rows   []policy.Row
		want   outcome
	}{
		{"a valid update takes effect", active, next, outcome{Accepted: true, Policy: view{
			Rules: map[string][]string{"infra.vendor.cloudflare": {"cloudflare.com"}},
			Order: []string{"infra.vendor.cloudflare"},
		}}},
		{"an invalid update leaves the active policy in place", active, invalid, outcome{
			Problems: []string{`rule 0 ("infra.vendor.cloudflare") has the domain suffix "cloud flare.com", which is not a domain name`},
			Policy: view{
				Rules: map[string][]string{"financial.brokerage.fidelity": {"fidelity.com", "fmr.com"}, "gov.federal.irs": {"irs.gov"}},
				Order: []string{"financial.brokerage.fidelity", "gov.federal.irs"},
			},
		}},
		{"an invalid first update leaves no policy", policy.Snapshot{}, invalid, outcome{
			Problems: []string{`rule 0 ("infra.vendor.cloudflare") has the domain suffix "cloud flare.com", which is not a domain name`},
			Policy:   view{RestrictsAll: true, Rules: map[string][]string{}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, o := policy.Swap(c.active, c.rows)
			got := outcome{Accepted: o.Accepted(), Problems: o.Problems(), Policy: observe(s.For("acct-a"))}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("Swap (-want +got):\n%s", diff)
			}
		})
	}
}

// A snapshot is immutable. Neither the rows it was loaded from nor the slices it hands out can
// change it afterwards.
func TestASnapshotCannotBeChangedAfterLoading(t *testing.T) {
	rows := []policy.Row{rule("", "gov.federal.irs", "irs.gov")}
	s := load(t, rows)
	rows[0].DomainSuffixes[0] = "attacker.example"
	c := s.For("acct-a")
	c.Rules()[0] = policy.Rule{}
	c.Rules()[0].DomainSuffixes()[0] = "attacker.example"
	want := view{Rules: map[string][]string{"gov.federal.irs": {"irs.gov"}}, Order: []string{"gov.federal.irs"}}
	if diff := cmp.Diff(want, observe(s.For("acct-a")), compare.Options); diff != "" {
		t.Errorf("snapshot after outside writes (-want +got):\n%s", diff)
	}
}

// No policy value exposes a field, so none can be built or changed outside the package (ADR-0042).
func TestNoPolicyValueExposesAField(t *testing.T) {
	mustnotcompile.RequireNoExportedFields(t, "github.com/ppat/mediated-mailbox-mcp/core/policy",
		"Rule", "Snapshot", "Composed", "Outcome")
}
