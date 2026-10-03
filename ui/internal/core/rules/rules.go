// Package rules is the pure rules of the UI's policy management (docs/UI.md sections 8.7 and 8.14).
// The shell reads the request, the scope's stored rules and the account's senders, and these decide
// what a write may hold, what an import changes and lifts, and what each rule matches and would
// release, as values (ADR-0040).
//
// A write is checked by the policy snapshot's own validation, core/policy's Validate, over the scope's
// rules as the write would leave them, so no write the UI makes fails a reload's validation (ADR-0041).
// The one check of the UI's own is the identifier a screen's address could not reach. Every match is
// the sender classifier's Name comparison, so a count here counts what classification decides.
package rules

import (
	"slices"
	"strconv"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
)

// The two scopes a rule belongs to, the base policy every account inherits and one account's own
// rules (ADR-0110).
const (
	Base    = "base"
	Account = "account"
)

// Rule is one rule of a scope, its identifier and its domain suffixes. Its class is always restricted.
type Rule struct {
	ID       string
	Suffixes []string
}

// Reserved returns the words a scope's screens use in their addresses after the policy path, which no
// identifier of that scope may be (docs/UI.md sections 8.7 and 8.14).
func Reserved(scope string) []string {
	if scope == Base {
		return []string{"new", "history", "import"}
	}
	return []string{"new", "pick", "history", "import", "base"}
}

// Unreachable reports whether a rule of scope with identifier id could not be reached at its own
// address, because it is one of the scope's route words, exactly . or .., or holds a /.
func Unreachable(id, scope string) bool {
	return slices.Contains(Reserved(scope), id) || id == "." || id == ".." || strings.Contains(id, "/")
}

// Kind names what is wrong with one rule of a write.
type Kind string

// The kinds of problem, the snapshot validation's and the UI's own.
const (
	BlankIdentifier    Kind = "blank_identifier"
	RepeatedIdentifier Kind = "repeated_identifier"
	NoSuffix           Kind = "no_suffix"
	InvalidSuffix      Kind = "invalid_suffix"
	OtherClass         Kind = "other_class"
	ReservedIdentifier Kind = "reserved_identifier"
)

// Problem is one problem with the rule at index Rule of the rules checked, with the suffix an invalid
// suffix names.
type Problem struct {
	Rule   int
	Kind   Kind
	Suffix string
}

// Check returns every problem with a scope's rules as a write would leave them, every one of them
// restricted. It is the snapshot's validation and the check that every identifier is reachable.
func Check(scope string, rules []Rule) []Problem {
	classes := make([]string, len(rules))
	for i := range classes {
		classes[i] = policy.Restricted
	}
	return CheckClasses(scope, rules, classes)
}

// CheckClasses is Check for rules whose classes were given, as a file states each rule's class.
func CheckClasses(scope string, rules []Rule, classes []string) []Problem {
	rows := make([]policy.Row, len(rules))
	for i, r := range rules {
		rows[i] = policy.Row{ID: r.ID, Class: classes[i], DomainSuffixes: r.Suffixes}
	}
	kinds := map[policy.ProblemKind]Kind{
		policy.BlankIdentifier:    BlankIdentifier,
		policy.RepeatedIdentifier: RepeatedIdentifier,
		policy.OtherClass:         OtherClass,
		policy.NoSuffix:           NoSuffix,
		policy.InvalidSuffix:      InvalidSuffix,
	}
	var out []Problem
	for _, p := range policy.Validate(rows) {
		out = append(out, Problem{Rule: p.Row, Kind: kinds[p.Kind], Suffix: p.Suffix})
	}
	for i, r := range rules {
		if Unreachable(r.ID, scope) {
			out = append(out, Problem{Rule: i, Kind: ReservedIdentifier})
		}
	}
	slices.SortStableFunc(out, func(a, b Problem) int { return a.Rule - b.Rule })
	return out
}

// Edit is one rule an import keeps whose suffixes change, with the suffixes it gains and loses.
type Edit struct {
	ID             string
	Before, After  []string
	Added, Removed []string
}

// Diff is what making a scope's stored rules equal to a file changes (ADR-0110). Added are the file's
// rules the scope does not hold, Edited the rules both hold whose suffixes differ, Lifted the stored
// rules the file leaves out, and Unchanged counts the rules both hold alike.
type Diff struct {
	Added     []Rule
	Edited    []Edit
	Lifted    []Rule
	Unchanged int
}

// Lifts counts the restrictions the diff lifts, each rule lifted and each suffix removed from a rule
// counting one (docs/UI.md section 8.7).
func (d Diff) Lifts() int {
	n := len(d.Lifted)
	for _, e := range d.Edited {
		n += len(e.Removed)
	}
	return n
}

// Changes counts the rules the diff adds, edits or lifts.
func (d Diff) Changes() int { return len(d.Added) + len(d.Edited) + len(d.Lifted) }

// Confirmation is what an import that lifts k restrictions must carry, typed for the base scope
// (docs/UI.md section 8.7).
func Confirmation(k int) string { return "lift " + strconv.Itoa(k) }

// Compare returns the diff between a scope's stored rules and a file's, each list in identifier
// order. Suffixes compare as sets, so a rule whose suffixes are only reordered is unchanged.
func Compare(stored, file []Rule) Diff {
	byID := map[string]Rule{}
	for _, r := range stored {
		byID[r.ID] = r
	}
	inFile := map[string]bool{}
	var d Diff
	for _, r := range sorted(file) {
		inFile[r.ID] = true
		s, held := byID[r.ID]
		if !held {
			d.Added = append(d.Added, r)
			continue
		}
		added, removed := minus(r.Suffixes, s.Suffixes), minus(s.Suffixes, r.Suffixes)
		if len(added) == 0 && len(removed) == 0 {
			d.Unchanged++
			continue
		}
		d.Edited = append(d.Edited, Edit{ID: r.ID, Before: slices.Clone(s.Suffixes), After: slices.Clone(r.Suffixes), Added: added, Removed: removed})
	}
	for _, r := range sorted(stored) {
		if !inFile[r.ID] {
			d.Lifted = append(d.Lifted, r)
		}
	}
	return d
}

// sorted returns rules in identifier order.
func sorted(rules []Rule) []Rule {
	return slices.SortedFunc(slices.Values(rules), func(a, b Rule) int { return strings.Compare(a.ID, b.ID) })
}

// minus returns the suffixes of a not in b, in a's order.
func minus(a, b []string) []string {
	var out []string
	for _, s := range a {
		if !slices.Contains(b, s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// Sender is one of the account's senders as the index stores it.
type Sender struct {
	Domain string
	// Restricted is the class the index stores for it, which "index updated" counts.
	Restricted bool
	Messages   int64
}

// Senders are an account's senders with each domain in the form the classifier compares, built once
// for every count a screen makes.
type Senders struct {
	senders []Sender
	names   []classify.Name
	ok      []bool
	lookups classify.Lookups
}

// NewSenders returns senders with their names. A domain the classifier cannot classify matches no
// rule, as Classify restricts it with no rule.
func NewSenders(senders []Sender, l classify.Lookups) Senders {
	s := Senders{senders: senders, names: make([]classify.Name, len(senders)), ok: make([]bool, len(senders)), lookups: l}
	for i, x := range senders {
		s.names[i], s.ok[i] = classify.SenderName(x.Domain, l)
	}
	return s
}

// Count is what a set of rules or suffixes matches among the account's senders.
type Count struct {
	Senders, Messages int64
	// Restricted counts the matched senders whose stored class reads restricted.
	Restricted int64
}

// under reports whether sender i is under any of suffixes.
func (s Senders) under(i int, suffixes []classify.Name) bool {
	if !s.ok[i] {
		return false
	}
	return slices.ContainsFunc(suffixes, func(n classify.Name) bool { return s.names[i].Under(n) })
}

// suffixNames returns suffixes in the form the classifier compares.
func (s Senders) suffixNames(suffixes []string) []classify.Name {
	out := make([]classify.Name, len(suffixes))
	for i, x := range suffixes {
		out[i] = classify.SuffixName(x, s.lookups)
	}
	return out
}

// Count is how many senders the account has.
func (s Senders) Count() int64 { return int64(len(s.senders)) }

// Match counts the senders any of suffixes matches, a sender counting once.
func (s Senders) Match(suffixes []string) Count {
	names := s.suffixNames(suffixes)
	var c Count
	for i, x := range s.senders {
		if s.under(i, names) {
			c.Senders++
			c.Messages += x.Messages
			if x.Restricted {
				c.Restricted++
			}
		}
	}
	return c
}

// Matched returns the senders suffixes match, in the order given.
func (s Senders) Matched(suffixes []string) []Sender {
	names := s.suffixNames(suffixes)
	var out []Sender
	for i, x := range s.senders {
		if s.under(i, names) {
			out = append(out, x)
		}
	}
	return out
}

// Released counts the senders some rule of before restricts and no rule of after does, and their
// messages, which a change from before to after releases (docs/UI.md section 8.7). Each argument is
// every suffix of the account's policy, its base rules' and its own.
func (s Senders) Released(before, after []string) Count {
	b, a := s.suffixNames(before), s.suffixNames(after)
	var c Count
	for i, x := range s.senders {
		if s.under(i, b) && !s.under(i, a) {
			c.Senders++
			c.Messages += x.Messages
		}
	}
	return c
}

// RestrictedBy returns the identifier of the first rule of rules that matches domain, in the order
// the classifier meets them, the base rules then the account's own, or the empty string for none.
func RestrictedBy(domain string, rules []Rule, l classify.Lookups) string {
	name, ok := classify.SenderName(domain, l)
	if !ok {
		return ""
	}
	for _, r := range rules {
		for _, suffix := range r.Suffixes {
			if name.Under(classify.SuffixName(suffix, l)) {
				return r.ID
			}
		}
	}
	return ""
}

// Covers reports whether a rule of suffixes already matches suffix, the suffix being one of them or a
// subdomain of one, in the form the classifier compares (docs/UI.md section 8.7).
func Covers(suffixes []string, suffix string, l classify.Lookups) bool {
	n := classify.SuffixName(suffix, l)
	return slices.ContainsFunc(suffixes, func(x string) bool { return n.Under(classify.SuffixName(x, l)) })
}

// PublicSuffix reports whether suffix is itself a public suffix, so it has no registrable domain and
// restricts every sender under it (docs/UI.md sections 8.7 and 8.14). A suffix the lookups cannot read
// is not one.
func PublicSuffix(suffix string, l classify.Lookups) bool {
	ascii, err := l.ToASCII(strings.TrimSuffix(suffix, "."))
	if err != nil || ascii == "" {
		return false
	}
	_, err = l.Registrable(ascii)
	return err != nil
}

// Suffixes returns every suffix of rules, in order.
func Suffixes(rules []Rule) []string {
	var out []string
	for _, r := range rules {
		out = append(out, r.Suffixes...)
	}
	return out
}
