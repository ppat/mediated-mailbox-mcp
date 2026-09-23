package policy_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

// rowArgs are the fields of one policy row.
type rowArgs struct {
	Account, ID, Class string
	Suffixes           []string
}

var (
	// rowAccounts are the accounts a drawn row can belong to, the empty one being the base policy.
	rowAccounts = []string{"", "acct-a", "acct-b"}
	// accounts are the accounts whose composed policies are checked, one of them with no overlay.
	accounts = []string{"acct-a", "acct-b", "acct-c"}
	suffixes = []string{"fidelity.com", "irs.gov", "examplebank.com", "bücher.example"}
)

// describe renders one rule as its identifier and suffixes, so a check compares restrictions and
// not identifiers alone.
func describe(id string, suffixes []string) string {
	return id + "=" + strings.Join(suffixes, ",")
}

func rows(args []rowArgs) []policy.Row {
	out := make([]policy.Row, 0, len(args))
	for _, a := range args {
		out = append(out, policy.Row{Account: a.Account, ID: a.ID, Class: a.Class, DomainSuffixes: a.Suffixes})
	}
	return out
}

// composed returns every checked account's composed rules, described, keyed by account.
func composed(s policy.Snapshot) map[string][]string {
	out := map[string][]string{}
	for _, account := range accounts {
		for _, r := range s.For(account).Rules() {
			out[account] = append(out[account], describe(r.ID(), r.DomainSuffixes()))
		}
	}
	return out
}

type swapArgs struct {
	Active, Update []rowArgs
	// UpdateCorrupted records that the draw broke one of the update's rows, so the property knows
	// the update is invalid without asking the code under test.
	UpdateCorrupted bool
}

// drawPolicy draws rows with every field valid and then one corruption, which may be no corruption
// at all, so both valid and invalid updates are common. It reports whether a corruption was
// applied. The rows' fields are drawn independently of the corruption.
func drawPolicy(t *rapid.T, label string) ([]rowArgs, bool) {
	rows := drawValidRows(t)
	corruption := rapid.SampledFrom([]string{"none", "none", "none", "empty id", "repeated id", "class", "suffix", "no suffix"}).Draw(t, label+" corruption")
	if len(rows) == 0 {
		return rows, false
	}
	switch corruption {
	case "empty id":
		rows[0].ID = ""
	case "repeated id":
		rows = append(rows, rows[0])
	case "class":
		rows[0].Class = "normal"
	case "suffix":
		rows[0].Suffixes = append(rows[0].Suffixes, "bad suffix.example")
	case "no suffix":
		rows[0].Suffixes = nil
	default:
		return rows, false
	}
	return rows, true
}

func drawSwap(t *rapid.T) swapArgs {
	active, _ := drawPolicy(t, "active")
	update, corrupted := drawPolicy(t, "update")
	return swapArgs{Active: active, Update: update, UpdateCorrupted: corrupted}
}

// validUpdate states over the drawn arguments, apart from the code, whether the update is valid. It
// is valid exactly when the draw corrupted nothing.
func validUpdate(a swapArgs) bool { return !a.UpdateCorrupted }

// A postcondition over every update to every active policy, a valid one or none. An update is
// accepted exactly when it is valid, a rejected update leaves the active policy exactly as it was,
// restrictions and all (ADR-0041), and an accepted one is exactly the update, so a Swap that keeps
// the active policy regardless fails too.
func TestAnInvalidUpdateNeverDisplacesTheActivePolicy(t *testing.T) {
	property.Check(t, drawSwap, func(t rapid.TB, a swapArgs) {
		active, err := policy.Load(rows(a.Active))
		if err != nil {
			// Drawn rows that do not load stand for a process where no policy has loaded yet.
			active = policy.Snapshot{}
		}
		before := composed(active)
		next, outcome := policy.Swap(active, rows(a.Update))
		if outcome.Accepted() != validUpdate(a) {
			t.Fatalf("%+v: accepted is %v, and the update is valid is %v", a, outcome.Accepted(), validUpdate(a))
		}
		if !outcome.Accepted() {
			if len(outcome.Problems()) == 0 {
				t.Fatalf("%+v was rejected with no problem named", a)
			}
			if next.Loaded() != active.Loaded() {
				t.Fatalf("%+v was rejected but changed whether a policy is loaded", a)
			}
			if diff := cmp.Diff(before, composed(next), compare.Options); diff != "" {
				t.Fatalf("%+v was rejected but changed the active policy (-before +after):\n%s", a, diff)
			}
			return
		}
		want := map[string][]string{}
		for _, account := range accounts {
			for _, r := range a.Update {
				if r.Account == "" {
					want[account] = append(want[account], describe(r.ID, r.Suffixes))
				}
			}
			for _, r := range a.Update {
				if r.Account != "" && r.Account == account {
					want[account] = append(want[account], describe(r.ID, r.Suffixes))
				}
			}
		}
		if !next.Loaded() {
			t.Fatalf("%+v was accepted but left no policy loaded", a)
		}
		if diff := cmp.Diff(want, composed(next), compare.Options); diff != "" {
			t.Fatalf("%+v was accepted but the active policy is not the update (-want +got):\n%s", a, diff)
		}
	})
}

func TestAnInvalidUpdateNeverDisplacesTheActivePolicyMix(t *testing.T) {
	property.Report(t, drawSwap, func(a swapArgs) string {
		_, activeErr := policy.Load(rows(a.Active))
		_, updateErr := policy.Load(rows(a.Update))
		switch {
		case activeErr == nil && updateErr != nil:
			return "invalid update over a loaded policy"
		case activeErr == nil:
			return "valid update over a loaded policy"
		case updateErr != nil:
			return "invalid update with no policy loaded"
		default:
			return "valid update with no policy loaded"
		}
	}, map[string]float64{
		"invalid update over a loaded policy":  0.01,
		"valid update over a loaded policy":    0.01,
		"invalid update with no policy loaded": 0.01,
	})
}

// drawValidRows draws rows whose every field is valid, with distinct identifiers, for the
// composition rule, which holds only over valid policies. Which account each row belongs to is
// drawn on its own.
func drawValidRows(t *rapid.T) []rowArgs {
	return rapid.SliceOfNDistinct(rapid.Custom(func(t *rapid.T) rowArgs {
		return rowArgs{
			Account:  rapid.SampledFrom(rowAccounts).Draw(t, "account"),
			ID:       rapid.StringMatching(`[a-z]{1,6}`).Draw(t, "id"),
			Class:    policy.Restricted,
			Suffixes: rapid.SliceOfN(rapid.SampledFrom(suffixes), 1, 2).Draw(t, "suffixes"),
		}
	}), 0, 5, func(r rowArgs) string { return r.ID }).Draw(t, "rows")
}

// A postcondition over every valid policy. Each account's composed policy holds every base rule, so
// an overlay never removes a restriction, and holds no rule of another account's overlay
// (ADR-0026).
func TestComposingAnOverlayNeverDropsABaseRule(t *testing.T) {
	property.Check(t, drawValidRows, func(t rapid.TB, a []rowArgs) {
		s, err := policy.Load(rows(a))
		if err != nil {
			return
		}
		got := composed(s)
		for _, account := range accounts {
			for _, r := range a {
				mine := r.Account == "" || r.Account == account
				rule := describe(r.ID, r.Suffixes)
				if mine && !slices.Contains(got[account], rule) {
					t.Fatalf("%+v: account %q lost rule %q", a, account, r.ID)
				}
				if !mine && slices.Contains(got[account], rule) {
					t.Fatalf("%+v: account %q holds rule %q of account %q", a, account, r.ID, r.Account)
				}
			}
		}
	})
}

func TestComposingAnOverlayNeverDropsABaseRuleMix(t *testing.T) {
	property.Report(t, drawValidRows, func(a []rowArgs) string {
		if _, err := policy.Load(rows(a)); err != nil {
			return "invalid"
		}
		base := slices.ContainsFunc(a, func(r rowArgs) bool { return r.Account == "" })
		overlay := slices.ContainsFunc(a, func(r rowArgs) bool { return r.Account != "" })
		switch {
		case base && overlay:
			return "base and overlay rules"
		case base:
			return "base rules only"
		case overlay:
			return "overlay rules only"
		default:
			return "no rules"
		}
	}, map[string]float64{
		"base and overlay rules": 0.01,
		"base rules only":        0.01,
		"overlay rules only":     0.01,
	})
}
