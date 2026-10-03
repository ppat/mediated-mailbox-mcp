// Package policy holds the policy snapshot that classification decides against (ADR-0041), and the
// rule that composes the base policy with an account's overlay (ADR-0026).
//
// The shell loads the rows of the policy tables, passes them to Swap with the snapshot active now,
// and makes the returned snapshot the active one. Swap validates the rows first. An update that
// fails validation never takes effect, and Swap returns the active snapshot unchanged with the
// problems for the shell's alarm. A snapshot is immutable, so a request or unit of work that took it
// keeps deciding against the same policy after a later swap. A policy with no rules is valid, so
// telling a failed or empty read of the tables apart from an empty policy is the shell's.
//
// The zero Snapshot is the state before any valid load, and every account's composed policy then
// restricts every sender, because absence denies. A loaded snapshot composes an account's policy as
// the base rules plus that account's own overlay rules. An overlay only adds restrictions, since
// every rule restricts and composing never drops a base rule.
//
// Matching a sender against the rules, and normalizing either side, is the sender classifier's.
package policy

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// Restricted is the only class a rule can carry (ADR-0016).
const Restricted = "restricted"

// Row is one row of the policy tables as the shell reads it. Account is empty for a base rule and
// names the account for an overlay rule.
type Row struct {
	Account        string
	ID             string
	Class          string
	DomainSuffixes []string
}

// Rule is one validated rule. Every rule restricts the senders whose domain it matches.
type Rule struct {
	id       string
	suffixes []string
}

// ID returns the rule's identifier.
func (r Rule) ID() string { return r.id }

// DomainSuffixes returns a copy of the domain suffixes the rule matches.
func (r Rule) DomainSuffixes() []string { return slices.Clone(r.suffixes) }

// Snapshot is one immutable policy. The zero value is no policy at all.
type Snapshot struct {
	loaded   bool
	base     []Rule
	overlays map[string][]Rule
}

// Loaded reports whether the snapshot came from a valid load.
func (s Snapshot) Loaded() bool { return s.loaded }

// Composed is the policy one account's decisions are made against. The zero value restricts every
// sender.
type Composed struct {
	loaded bool
	rules  []Rule
}

// RestrictsAll reports whether every sender is restricted, which holds while no valid policy has
// loaded.
func (c Composed) RestrictsAll() bool { return !c.loaded }

// Rules returns a copy of the account's rules, the base rules followed by its overlay rules.
func (c Composed) Rules() []Rule { return slices.Clone(c.rules) }

// For returns the policy account's decisions are made against. Every operation names its account
// (ADR-0026), so an empty account name gets the policy that restricts every sender.
func (s Snapshot) For(account string) Composed {
	if !s.loaded || account == "" {
		return Composed{}
	}
	return Composed{loaded: true, rules: slices.Concat(s.base, s.overlays[account])}
}

// Load validates rows and returns the snapshot they describe, or every problem found, joined.
func Load(rows []Row) (Snapshot, error) {
	s, problems := load(rows)
	if len(problems) > 0 {
		errs := make([]error, 0, len(problems))
		for _, p := range problems {
			errs = append(errs, errors.New(p))
		}
		return Snapshot{}, errors.Join(errs...)
	}
	return s, nil
}

// ProblemKind names what is wrong with a row.
type ProblemKind uint8

const (
	// BlankIdentifier is an empty identifier or one with surrounding space.
	BlankIdentifier ProblemKind = iota + 1
	// RepeatedIdentifier is an identifier another rule of the same scope holds.
	RepeatedIdentifier
	// OtherClass is a class other than Restricted.
	OtherClass
	// NoSuffix is a rule with no domain suffix.
	NoSuffix
	// InvalidSuffix is a domain suffix not shaped like a domain name.
	InvalidSuffix
)

// Problem is one reason a row fails validation. Row indexes the rows validated, and Suffix is the
// suffix an InvalidSuffix names.
type Problem struct {
	Row    int
	Kind   ProblemKind
	Suffix string
}

// Validate returns every problem with rows, in row order. Every rule needs an identifier no other rule
// of its scope has, the restricted class, and at least one domain suffix shaped like a domain name
// (validSuffix). A scope is the base policy or one account's overlay, so the base policy and an
// account, or two accounts, may each hold a rule of one identifier, as the policy table's key allows
// (ADR-0110). The form a suffix is matched in, and how either side is normalized, are the
// classifier's. Validate is the snapshot's validation, so a writer that checks rows with it before it
// writes them writes nothing a reload refuses.
func Validate(rows []Row) []Problem {
	var problems []Problem
	type scoped struct{ account, id string }
	seen := map[scoped]bool{}
	for i, row := range rows {
		if row.ID == "" || strings.TrimSpace(row.ID) != row.ID {
			problems = append(problems, Problem{Row: i, Kind: BlankIdentifier})
		}
		key := scoped{account: row.Account, id: row.ID}
		if seen[key] {
			problems = append(problems, Problem{Row: i, Kind: RepeatedIdentifier})
		}
		seen[key] = true
		if row.Class != Restricted {
			problems = append(problems, Problem{Row: i, Kind: OtherClass})
		}
		if len(row.DomainSuffixes) == 0 {
			problems = append(problems, Problem{Row: i, Kind: NoSuffix})
		}
		for _, suffix := range row.DomainSuffixes {
			if !validSuffix(suffix) {
				problems = append(problems, Problem{Row: i, Kind: InvalidSuffix, Suffix: suffix})
			}
		}
	}
	return problems
}

// describe words a problem with the row it is about.
func describe(p Problem, row Row) string {
	where := "rule " + strconv.Itoa(p.Row) + " (" + strconv.Quote(row.ID) + ")"
	switch p.Kind {
	case BlankIdentifier:
		return where + " has an empty identifier or one with surrounding space"
	case RepeatedIdentifier:
		return where + " repeats an identifier within its scope"
	case OtherClass:
		return where + " has the class " + strconv.Quote(row.Class) + ", and the only class is " + strconv.Quote(Restricted)
	case NoSuffix:
		return where + " has no domain suffix"
	case InvalidSuffix:
		return where + " has the domain suffix " + strconv.Quote(p.Suffix) + ", which is not a domain name"
	}
	return where + " is not a valid rule"
}

// load validates rows with Validate and builds the snapshot they describe.
func load(rows []Row) (Snapshot, []string) {
	var problems []string
	for _, p := range Validate(rows) {
		problems = append(problems, describe(p, rows[p.Row]))
	}
	s := Snapshot{loaded: true, overlays: map[string][]Rule{}}
	for _, row := range rows {
		rule := Rule{id: row.ID, suffixes: slices.Clone(row.DomainSuffixes)}
		if row.Account == "" {
			s.base = append(s.base, rule)
		} else {
			s.overlays[row.Account] = append(s.overlays[row.Account], rule)
		}
	}
	return s, problems
}

// validSuffix reports whether suffix has the shape of a domain name, dot-separated labels that are
// not empty and hold only letters of any script, digits, combining marks and hyphens. A suffix with
// any other character, such as `@`, `/`, `*` or a space, can never match a sender, so it is refused
// rather than loaded as a rule that restricts nothing.
func validSuffix(suffix string) bool {
	if suffix == "" {
		return false
	}
	for label := range strings.SplitSeq(suffix, ".") {
		if label == "" {
			return false
		}
		for _, r := range label {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsMark(r) && r != '-' {
				return false
			}
		}
	}
	return true
}

// Outcome is the verdict on one update. The zero value is a rejection.
type Outcome struct {
	accepted bool
	problems []string
}

// Accepted reports whether the update took effect.
func (o Outcome) Accepted() bool { return o.accepted }

// Problems returns why the update was rejected, one entry per problem.
func (o Outcome) Problems() []string { return slices.Clone(o.problems) }

// Swap returns the snapshot that is active after an update to rows, and the verdict on the update. A
// valid update becomes the active snapshot. An invalid one leaves active in place, whether or not
// active was ever loaded.
func Swap(active Snapshot, rows []Row) (Snapshot, Outcome) {
	next, problems := load(rows)
	if len(problems) > 0 {
		return active, Outcome{problems: problems}
	}
	return next, Outcome{accepted: true}
}
