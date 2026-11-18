// Package rules is the pure rules of the UI's policy management (docs/UI.md sections 8.7 and 8.14).
// The shell reads the request, the scope's stored rules and the account's senders, and these decide
// whether a write is refused and why, what an import changes and lifts, what a search keeps, and what
// each rule matches and would release, as values (ADR-0040).
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
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
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

// Unreachable reports whether a rule with identifier id could not be reached at its own address,
// because no path segment can hold it, exactly . or .., or holding a /. A rule's identifier follows
// the word rules or base in its address and never sits where a route word does, so no word is
// unreachable (docs/UI.md sections 5 and 8.7, ADR-0137).
func Unreachable(id string) bool {
	return id == "." || id == ".." || strings.Contains(id, "/")
}

// Problem is one problem with the rule at index Rule of the rules checked, with the suffix an invalid
// suffix names. Its kind is the snapshot validation's or the UI's own.
type Problem struct {
	Rule   int
	Kind   refusal.Kind
	Suffix string
}

// Check returns every problem with a scope's rules as a write would leave them, every one of them
// restricted. It is the snapshot's validation and the check that every identifier is reachable.
func Check(rules []Rule) []Problem {
	classes := make([]string, len(rules))
	for i := range classes {
		classes[i] = policy.Restricted
	}
	return CheckClasses(rules, classes)
}

// CheckClasses is Check for rules whose classes were given, as a file states each rule's class.
func CheckClasses(rules []Rule, classes []string) []Problem {
	rows := make([]policy.Row, len(rules))
	for i, r := range rules {
		rows[i] = policy.Row{ID: r.ID, Class: classes[i], DomainSuffixes: r.Suffixes}
	}
	kinds := map[policy.ProblemKind]refusal.Kind{
		policy.BlankIdentifier:    refusal.BlankIdentifier,
		policy.RepeatedIdentifier: refusal.RepeatedIdentifier,
		policy.OtherClass:         refusal.OtherClass,
		policy.NoSuffix:           refusal.NoSuffix,
		policy.InvalidSuffix:      refusal.InvalidSuffix,
	}
	var out []Problem
	for _, p := range policy.Validate(rows) {
		out = append(out, Problem{Rule: p.Row, Kind: kinds[p.Kind], Suffix: p.Suffix})
	}
	for i, r := range rules {
		if Unreachable(r.ID) {
			out = append(out, Problem{Rule: i, Kind: refusal.ReservedIdentifier})
		}
	}
	slices.SortStableFunc(out, func(a, b Problem) int { return a.Rule - b.Rule })
	return out
}

// Adding decides adding r to a scope whose rules are stored, refusing an identifier the scope holds as
// taken, and a rule the policy's checks refuse, over the scope's rules as the write would leave them,
// with its problems (ADR-0041).
func Adding(stored []Rule, r Rule) *refusal.Refusal {
	if slices.ContainsFunc(stored, func(s Rule) bool { return s.ID == r.ID }) {
		return Taken()
	}
	after := append(slices.Clone(stored), r)
	return checked(after, len(after)-1)
}

// Editing decides setting the suffixes of the rule id of a scope whose rules are stored to after, made
// against before, the suffixes the page read. A rule the scope does not hold is unknown and never a
// stale conflict (ADR-0084), and one whose suffixes are not before is stale. A rule the policy's
// checks refuse is refused with its problems, which refuses an edit removing every suffix, and an
// edit removing a suffix of a base rule needs the rule's identifier as its confirmation.
func Editing(stored []Rule, id string, before, after []string, base bool, confirmation *string) *refusal.Refusal {
	i, r := held(stored, id, before)
	if r != nil {
		return r
	}
	next := slices.Clone(stored)
	next[i] = Rule{ID: id, Suffixes: after}
	if r := checked(next, i); r != nil {
		return r
	}
	removes := slices.ContainsFunc(stored[i].Suffixes, func(x string) bool { return !slices.Contains(after, x) })
	if base && removes && !Confirms(confirmation, id) {
		return confirmationRequired()
	}
	return nil
}

// Lifting decides removing the rule id of a scope whose rules are stored, read first as an edit is. A
// base rule's lift needs the rule's identifier as its confirmation.
func Lifting(stored []Rule, id string, before []string, base bool, confirmation *string) *refusal.Refusal {
	if _, r := held(stored, id, before); r != nil {
		return r
	}
	if base && !Confirms(confirmation, id) {
		return confirmationRequired()
	}
	return nil
}

// held returns the index of the rule id among stored, refusing a rule stored does not hold as unknown
// and one whose suffixes are not before as stale.
func held(stored []Rule, id string, before []string) (int, *refusal.Refusal) {
	i := slices.IndexFunc(stored, func(r Rule) bool { return r.ID == id })
	if i < 0 {
		return -1, UnknownRule()
	}
	if !slices.Equal(stored[i].Suffixes, before) {
		return -1, StaleRule()
	}
	return i, nil
}

// checked refuses a scope's rules as a write would leave them, when the policy's checks find a problem
// with the rule at index i, the one the write changes, naming each with the rule's identifier.
func checked(after []Rule, i int) *refusal.Refusal {
	var mine []refusal.Problem
	for _, p := range Check(after) {
		if p.Rule == i {
			mine = append(mine, refusal.Problem{Kind: p.Kind, Rule: after[i].ID, Suffix: p.Suffix})
		}
	}
	if len(mine) == 0 {
		return nil
	}
	return &refusal.Refusal{Code: refusal.RuleRefused, Message: "the rule fails the policy's checks", Problems: mine}
}

// Taken is the refusal of a rule whose identifier its scope holds, which the table's key also refuses
// when another write added one meanwhile (ADR-0110).
func Taken() *refusal.Refusal {
	return refusal.Of(refusal.IdentifierTaken, "the scope already holds a rule of this identifier")
}

// UnknownRule is the refusal of a rule its scope does not hold.
func UnknownRule() *refusal.Refusal {
	return refusal.Of(refusal.UnknownRule, "the policy holds no rule of this identifier in this scope")
}

// StaleRule is the refusal of a write whose rule changed since the page read it, which the write's
// own statement also finds when another write changed it meanwhile.
func StaleRule() *refusal.Refusal {
	return refusal.Of(refusal.StaleRule, "the rule changed since the page read it")
}

func confirmationRequired() *refusal.Refusal {
	return refusal.Of(refusal.ConfirmationRequired, "lifting a base rule's restriction needs the rule's identifier typed")
}

// Confirms reports whether a typed confirmation is want, compared after trimming surrounding space as
// the screen's field compares it (docs/UI.md section 8.7).
func Confirms(confirmation *string, want string) bool {
	return confirmation != nil && strings.TrimSpace(*confirmation) == want
}

// Matches reports whether a rule's identifier or one of its suffixes holds search, compared without
// case, the policy screens' search (docs/UI.md sections 5 and 8.14).
func Matches(r Rule, search string) bool {
	s := strings.ToLower(search)
	if strings.Contains(strings.ToLower(r.ID), s) {
		return true
	}
	return slices.ContainsFunc(r.Suffixes, func(x string) bool { return strings.Contains(strings.ToLower(x), s) })
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

// Stored is one of the account's senders as the index stores it, its domain and its message count.
// The index stores no class (ADR-0136).
type Stored struct {
	Domain   string
	Messages int64
}

// Sender is one of the account's senders with its class under the policy the request read.
type Sender struct {
	Domain   string
	Messages int64
	// Restricted is set when the composed policy restricts the sender, a rule matching its domain or
	// the classifier unable to read the domain.
	Restricted bool
}

// Senders are an account's senders with each domain in the form the classifier compares and its class
// under the composed policy, built once for every count a screen makes.
type Senders struct {
	senders []Sender
	names   []classify.Name
	ok      []bool
	lookups classify.Lookups
}

// NewSenders returns stored with their names and each one's class as Classify decides it under p, the
// composed policy read in the same transaction as stored, prepared with l. A domain the classifier
// cannot classify matches no rule and is restricted (ADR-0004, ADR-0136).
func NewSenders(stored []Stored, p classify.Policy, l classify.Lookups) Senders {
	s := Senders{senders: make([]Sender, len(stored)), names: make([]classify.Name, len(stored)), ok: make([]bool, len(stored)), lookups: l}
	for i, x := range stored {
		s.names[i], s.ok[i] = classify.SenderName(x.Domain, l)
		s.senders[i] = Sender{Domain: x.Domain, Messages: x.Messages, Restricted: Classify(p, x.Domain).Class().Restricted()}
	}
	return s
}

// Classify returns the class of the sender at a stored domain under p, as the classifier decides it
// for an address at that domain. A stored domain is the part of an address after its last @, so it
// holds no @ of its own (ADR-0016).
func Classify(p classify.Policy, domain string) classify.Verdict {
	return classify.Classify(p, "@"+domain)
}

// Count is what a set of rules or suffixes matches among the account's senders.
type Count struct {
	Senders, Messages int64
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
	return SuffixNames(suffixes, s.lookups)
}

// SuffixNames returns suffixes in the form the classifier compares, converted once for every
// comparison a request makes with them.
func SuffixNames(suffixes []string, l classify.Lookups) []classify.Name {
	out := make([]classify.Name, len(suffixes))
	for i, x := range suffixes {
		out[i] = classify.SuffixName(x, l)
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
		}
	}
	return c
}

// Normal returns the domains of the senders the composed policy does not restrict, in the order
// given, never nil, which a statement counts every other domain against as restricted, so a domain
// the read did not classify counts as restricted (ADR-0108, ADR-0136).
func (s Senders) Normal() []string {
	out := []string{}
	for _, x := range s.senders {
		if !x.Restricted {
			out = append(out, x.Domain)
		}
	}
	return out
}

// Restricted counts the senders the composed policy restricts and their messages, which Home's
// restricted messages and the policy screen's senders restricted show (docs/UI.md sections 8.1 and
// 8.7). The statistics hold each stored domain once with its exact count, so the sum is exact
// (ADR-0109).
func (s Senders) Restricted() Count {
	var c Count
	for _, x := range s.senders {
		if x.Restricted {
			c.Senders++
			c.Messages += x.Messages
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

// Covers reports whether a rule whose suffixes are names already matches suffix, the suffix being one
// of them or a subdomain of one, in the form the classifier compares (docs/UI.md section 8.7). The
// names are SuffixNames', converted once for every suffix a request checks.
func Covers(names []classify.Name, suffix string, l classify.Lookups) bool {
	return slices.ContainsFunc(names, classify.SuffixName(suffix, l).Under)
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
