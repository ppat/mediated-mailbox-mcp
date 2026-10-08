package registry

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	senderclassification "github.com/ppat/mediated-mailbox-mcp/db/senders/classification"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// The policy history's actions as stored (ADR-0102).
func changeActions() []string { return []string{"added", "edited", "lifted", "confirmed"} }

// A rule's row identity in the row-detail path is its scope and its identifier, base:{id} or
// account:{id}, since both scopes may hold one identifier (docs/UI.md section 5). The scope word holds
// no colon, so the first colon ends it, and an identifier may hold colons of its own.
const (
	baseRow    = rules.Base + ":"
	accountRow = rules.Account + ":"
)

// RuleRowID is the row identity of a rule of scope with identifier id.
func RuleRowID(scope, id string) string { return scope + ":" + id }

// StoredRule is one rule as policy management reads it, from either scope.
type StoredRule struct {
	// Base is set for a base rule, which every account inherits.
	Base      bool
	ID        string
	Class     string
	Suffixes  []string
	Source    string
	CreatedAt time.Time
	CreatedBy string
}

// ComposedRules reads the account's policy, the base rules and the account's own, in identifier order
// with a base rule ahead of the account's rule of one identifier.
func ComposedRules(ctx context.Context, q *manage.Queries, account string) ([]StoredRule, error) {
	read, err := q.ComposedRules(ctx, account)
	if err != nil {
		return nil, err
	}
	out := make([]StoredRule, 0, len(read))
	for _, r := range read {
		out = append(out, StoredRule{
			Base: !r.AccountID.Valid, ID: r.RuleID, Class: r.Class, Suffixes: r.DomainSuffix,
			Source: r.Source, CreatedAt: r.CreatedAt.Time, CreatedBy: r.CreatedBy,
		})
	}
	return out, nil
}

// Composed returns the rules in the order the classifier meets them, the base rules and then the
// account's own, each in identifier order.
func Composed(stored []StoredRule) []rules.Rule {
	var base, own []rules.Rule
	for _, r := range stored {
		rule := rules.Rule{ID: r.ID, Suffixes: r.Suffixes}
		if r.Base {
			base = append(base, rule)
		} else {
			own = append(own, rule)
		}
	}
	return append(base, own...)
}

// RuleRow is one row of the rules dataset, the policy screen's columns (docs/UI.md section 8.7). Its
// counts are the account's alone. Senders counts the senders the rule matches in the index,
// Restricted those whose stored class reads restricted, which "index updated" shows, and Messages
// their messages.
type RuleRow struct {
	Scope      string   `json:"scope"`
	RuleID     string   `json:"rule_id"`
	Class      string   `json:"class"`
	Suffixes   []string `json:"suffixes"`
	Source     string   `json:"source"`
	CreatedAt  string   `json:"created_at"`
	CreatedBy  string   `json:"created_by"`
	Senders    int64    `json:"senders"`
	Restricted int64    `json:"restricted"`
	Messages   int64    `json:"messages"`
}

func ruleFields() []schema.Field {
	return []schema.Field{
		schema.F("scope", schema.Str(rules.Base, rules.Account)),
		schema.F("rule_id", schema.Str()),
		schema.F("class", schema.Str()),
		schema.F("suffixes", schema.ArrayOf(schema.Str())),
		schema.F("source", schema.Str()),
		schema.F("created_at", schema.Time()),
		schema.F("created_by", schema.Str()),
		schema.F("senders", schema.Int()),
		schema.F("restricted", schema.Int()),
		schema.F("messages", schema.Int()),
	}
}

func ruleRowType() schema.Type { return schema.Obj("RuleRow", ruleFields()...) }

func scopeOf(base bool) string {
	if base {
		return rules.Base
	}
	return rules.Account
}

// SenderClasses reads the account's senders as the counts take them.
func SenderClasses(ctx context.Context, q Queries, account string, l classify.Lookups) (rules.Senders, int64, error) {
	read, err := q.Senders.SenderClasses(ctx, account)
	if err != nil {
		return rules.Senders{}, 0, err
	}
	senders := make([]rules.Sender, 0, len(read))
	var restricted int64
	for _, s := range read {
		r := s.SenderClass == "restricted"
		if r {
			restricted++
		}
		senders = append(senders, rules.Sender{Domain: s.Domain, Restricted: r, Messages: s.MessageCount})
	}
	return rules.NewSenders(senders, l), restricted, nil
}

// ruleRow is a stored rule with its counts in the account.
func ruleRow(r StoredRule, senders rules.Senders) RuleRow {
	c := senders.Match(r.Suffixes)
	return RuleRow{
		Scope: scopeOf(r.Base), RuleID: r.ID, Class: r.Class, Suffixes: slices.Clone(r.Suffixes), Source: r.Source,
		CreatedAt: Stamp(r.CreatedAt), CreatedBy: r.CreatedBy, Senders: c.Senders, Restricted: c.Restricted, Messages: c.Messages,
	}
}

// matchesSearch reports whether a rule's identifier or one of its suffixes holds search, compared
// without case (docs/UI.md section 5).
func matchesSearch(r StoredRule, search string) bool {
	s := strings.ToLower(search)
	if strings.Contains(strings.ToLower(r.ID), s) {
		return true
	}
	return slices.ContainsFunc(r.Suffixes, func(x string) bool { return strings.Contains(strings.ToLower(x), s) })
}

// filtered returns the account's rules the read's range and search keep, in the policy screen's
// order, the base rules first and then the account's own, each by identifier, reversed for a
// descending sort.
func filtered(stored []StoredRule, r Read) []StoredRule {
	search, searched := r.Request.Filter("search")
	var out []StoredRule
	for _, rule := range stored {
		if r.Bounds.Start != nil && rule.CreatedAt.Before(*r.Bounds.Start) || r.Bounds.End != nil && !rule.CreatedAt.Before(*r.Bounds.End) {
			continue
		}
		if searched && !matchesSearch(rule, search.Values[0]) {
			continue
		}
		out = append(out, rule)
	}
	slices.SortStableFunc(out, func(a, b StoredRule) int {
		if a.Base != b.Base {
			if a.Base {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	if r.Request.Sort.Descending {
		slices.Reverse(out)
	}
	return out
}

// policyRules is the policy screen's rules, the base rules and the account's own (docs/UI.md sections
// 5 and 8.7). The rules of one account's policy are few, so the screen's search, sort and page and each
// rule's counts are made in the server over the whole policy and the account's senders, read in the
// read's transaction. Matching a sender against a rule is the classifier's, so l is the lookups the
// classifier normalizes with.
func policyRules(l classify.Lookups) Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:        "rules",
			Ranged:      true,
			RangeColumn: "created_at",
			Dimensions: []lens.Dimension{
				{Name: "search", Storage: "text", Filterable: true, Wording: "search", Search: true},
				{Name: "rule_id", Storage: "text", Sortable: true, Wording: "rule"},
			},
			Default:  lens.Defaults{Level: lens.Rows, Range: "all", Sort: lens.Sort{Column: "rule_id"}},
			Identity: &lens.RowIdentity{Name: "rule", Storage: "text"},
		},
		Row: ruleRowType(),
		Summary: func(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
			return rulesSummary(ctx, q, r, l)
		},
		Rows: func(ctx context.Context, q Queries, r Read) (any, error) {
			return rulesRows(ctx, q, r, l)
		},
		Detail: func(ctx context.Context, q Queries, account string, req lens.RowRequest, _ time.Time) (any, error) {
			return ruleDetail(ctx, q, account, req, l)
		},
		DetailType: ruleDetailType(),
	}
}

func rulesSummary(ctx context.Context, q Queries, r Read, l classify.Lookups) ([]Figure, Total, error) {
	stored, err := ComposedRules(ctx, q.Rules, r.Account)
	if err != nil {
		return nil, Total{}, err
	}
	_, restricted, err := SenderClasses(ctx, q, r.Account, l)
	if err != nil {
		return nil, Total{}, err
	}
	latest, err := q.Changes.LatestChange(ctx, r.Account)
	if err != nil {
		return nil, Total{}, err
	}
	var base, own int64
	for _, rule := range stored {
		if rule.Base {
			base++
		} else {
			own++
		}
	}
	figures := []Figure{
		count("base", "base rules", base, link(r.Account, "policy", nil)),
		count("own", "rules for this account", own, link(r.Account, "policy", nil)),
		count("restricted", "senders restricted", restricted, link(r.Account, "policy/pick", nil)),
		{Key: "latest", Wording: "latest change", At: optionalStamp(latest), Link: link(r.Account, "policy/history", nil)},
	}
	return figures, Total{Count: int64(len(filtered(stored, r)))}, nil
}

func rulesRows(ctx context.Context, q Queries, r Read, l classify.Lookups) (any, error) {
	stored, err := ComposedRules(ctx, q.Rules, r.Account)
	if err != nil {
		return nil, err
	}
	senders, _, err := SenderClasses(ctx, q, r.Account, l)
	if err != nil {
		return nil, err
	}
	kept := filtered(stored, r)
	first := (r.Request.Page - 1) * lens.RowsPerPage
	out := []RuleRow{}
	for i := first; i < len(kept) && i < first+lens.RowsPerPage; i++ {
		out = append(out, ruleRow(kept[i], senders))
	}
	return out, nil
}

// SuffixDetail is one suffix of a rule with what it matches in the account and what removing it alone
// would release there.
type SuffixDetail struct {
	Suffix   string `json:"suffix"`
	Senders  int64  `json:"senders"`
	Messages int64  `json:"messages"`
	Released Counts `json:"released"`
}

// Counts are a count of senders and their messages.
type Counts struct {
	Senders  int64 `json:"senders"`
	Messages int64 `json:"messages"`
}

func countsType() schema.Type {
	return schema.Obj("Counts", schema.F("senders", schema.Int()), schema.F("messages", schema.Int()))
}

// SenderMatch is one sender a rule matches, with the sender row's fields the rule screen shows.
type SenderMatch struct {
	Domain      string `json:"domain"`
	SenderClass string `json:"sender_class"`
	Messages    int64  `json:"messages"`
}

// ChangeRow is one row of the policy history, the policy change row of docs/UI.md section 7.2.
// Account is null for a base rule's change. RuleExists says the rule it names is held in its scope
// now, which a link to the rule needs.
type ChangeRow struct {
	ID             int64    `json:"id"`
	TS             string   `json:"ts"`
	Account        *string  `json:"account"`
	Actor          string   `json:"actor"`
	Action         string   `json:"action"`
	RuleID         string   `json:"rule_id"`
	SuffixesBefore []string `json:"suffixes_before"`
	SuffixesAfter  []string `json:"suffixes_after"`
	RuleExists     bool     `json:"rule_exists"`
}

// ChangeRowType is ChangeRow's declaration.
func ChangeRowType() schema.Type {
	return schema.Obj("ChangeRow",
		schema.F("id", schema.Int()),
		schema.F("ts", schema.Time()),
		schema.F("account", schema.Null(schema.Str())),
		schema.F("actor", schema.Str()),
		schema.F("action", schema.Str(changeActions()...)),
		schema.F("rule_id", schema.Str()),
		schema.F("suffixes_before", schema.ArrayOf(schema.Str())),
		schema.F("suffixes_after", schema.ArrayOf(schema.Str())),
		schema.F("rule_exists", schema.Bool()),
	)
}

// RuleDetail is one rule's screen (docs/UI.md section 8.7). Released is what lifting the whole rule
// would release in the account, the senders it restricts and no other rule does, and each suffix's is
// what removing that suffix alone would. Senders are the first fifty senders it matches, most messages
// first, of SendersTotal. Accounts lists every account, which a base rule's lift reaches, and is empty
// for an account's rule.
type RuleDetail struct {
	RuleRow
	SuffixDetails []SuffixDetail  `json:"suffix_details"`
	Released      Counts          `json:"released"`
	Senders       []SenderMatch   `json:"matched"`
	SendersTotal  int64           `json:"matched_total"`
	History       []ChangeRow     `json:"history"`
	Accounts      []string        `json:"accounts"`
	Elsewhere     *ElsewhereScope `json:"elsewhere"`
}

// ElsewhereScope says the account's policy holds a rule of the same identifier in the other scope,
// with its suffixes, so the screen can link to it.
type ElsewhereScope struct {
	Scope    string   `json:"scope"`
	Suffixes []string `json:"suffixes"`
}

func ruleDetailType() schema.Type {
	return schema.Obj("RuleDetail", append(ruleFields(),
		schema.F("suffix_details", schema.ArrayOf(schema.Obj("SuffixDetail",
			schema.F("suffix", schema.Str()),
			schema.F("senders", schema.Int()),
			schema.F("messages", schema.Int()),
			schema.F("released", countsType()),
		))),
		schema.F("released", countsType()),
		schema.F("matched", schema.ArrayOf(schema.Obj("SenderMatch",
			schema.F("domain", schema.Str()),
			schema.F("sender_class", schema.Str()),
			schema.F("messages", schema.Int()),
		))),
		schema.F("matched_total", schema.Int()),
		schema.F("history", schema.ArrayOf(ChangeRowType())),
		schema.F("accounts", schema.ArrayOf(schema.Str())),
		schema.F("elsewhere", schema.Null(schema.Obj("ElsewhereScope",
			schema.F("scope", schema.Str(rules.Base, rules.Account)),
			schema.F("suffixes", schema.ArrayOf(schema.Str())),
		))),
	)...)
}

// without returns the composed policy's suffixes with the rule of scope and identifier id taking only
// keep.
func without(stored []StoredRule, base bool, id string, keep []string) []string {
	var out []string
	for _, r := range stored {
		if r.Base == base && r.ID == id {
			out = append(out, keep...)
			continue
		}
		out = append(out, r.Suffixes...)
	}
	return out
}

// RuleRelease returns what lifting a rule of the account's policy, or removing some of its suffixes,
// would release in the account, the senders the policy restricts now and would not after.
func RuleRelease(stored []StoredRule, senders rules.Senders, base bool, id string, after []string) Counts {
	c := senders.Released(without(stored, base, id, rulesSuffixes(stored, base, id)), without(stored, base, id, after))
	return Counts{Senders: c.Senders, Messages: c.Messages}
}

func rulesSuffixes(stored []StoredRule, base bool, id string) []string {
	for _, r := range stored {
		if r.Base == base && r.ID == id {
			return r.Suffixes
		}
	}
	return nil
}

func ruleDetail(ctx context.Context, q Queries, account string, req lens.RowRequest, l classify.Lookups) (any, error) {
	var base bool
	var id string
	switch {
	case strings.HasPrefix(req.Row, baseRow):
		base, id = true, strings.TrimPrefix(req.Row, baseRow)
	case strings.HasPrefix(req.Row, accountRow):
		id = strings.TrimPrefix(req.Row, accountRow)
	default:
		return nil, ErrNoRow
	}
	stored, err := ComposedRules(ctx, q.Rules, account)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(stored, func(r StoredRule) bool { return r.Base == base && r.ID == id })
	if i < 0 {
		return nil, ErrNoRow
	}
	rule := stored[i]
	senders, _, err := SenderClasses(ctx, q, account, l)
	if err != nil {
		return nil, err
	}
	d := RuleDetail{RuleRow: ruleRow(rule, senders), SuffixDetails: []SuffixDetail{}, Senders: []SenderMatch{}, History: []ChangeRow{}, Accounts: []string{}}
	for _, suffix := range rule.Suffixes {
		c := senders.Match([]string{suffix})
		rest := slices.DeleteFunc(slices.Clone(rule.Suffixes), func(x string) bool { return x == suffix })
		d.SuffixDetails = append(d.SuffixDetails, SuffixDetail{
			Suffix: suffix, Senders: c.Senders, Messages: c.Messages, Released: RuleRelease(stored, senders, base, id, rest),
		})
	}
	d.Released = RuleRelease(stored, senders, base, id, nil)
	matched := senders.Matched(rule.Suffixes)
	slices.SortStableFunc(matched, func(a, b rules.Sender) int {
		if a.Messages != b.Messages {
			if a.Messages > b.Messages {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Domain, b.Domain)
	})
	d.SendersTotal = int64(len(matched))
	for _, s := range matched[:min(len(matched), lens.RowsPerPage)] {
		class := "normal"
		if s.Restricted {
			class = "restricted"
		}
		d.Senders = append(d.Senders, SenderMatch{Domain: s.Domain, SenderClass: class, Messages: s.Messages})
	}
	history, err := q.Changes.RuleHistory(ctx, policychanges.RuleHistoryParams{AccountID: account, RuleID: id, Base: base})
	if err != nil {
		return nil, err
	}
	for _, h := range history {
		d.History = append(d.History, changeRow(h.ID, h.Ts, h.AccountID, h.Actor, h.Action, h.RuleID, h.SuffixesBefore, h.SuffixesAfter, stored))
	}
	if base {
		listed, err := q.Accounts.Accounts(ctx)
		if err != nil {
			return nil, err
		}
		for _, a := range listed {
			d.Accounts = append(d.Accounts, a.AccountID)
		}
	}
	if j := slices.IndexFunc(stored, func(r StoredRule) bool { return r.Base != base && r.ID == id }); j >= 0 {
		d.Elsewhere = &ElsewhereScope{Scope: scopeOf(!base), Suffixes: slices.Clone(stored[j].Suffixes)}
	}
	return d, nil
}

// changeRow is a stored change as the read API sends it. A rule exists when the account's policy holds
// a rule of its identifier in its scope.
func changeRow(id int64, ts pgtype.Timestamptz, account pgtype.Text, actor, action, rule string, before, after []string, stored []StoredRule) ChangeRow {
	base := !account.Valid
	return ChangeRow{
		ID: id, TS: stamp(ts), Account: text(account.Valid, account.String), Actor: actor, Action: action, RuleID: rule,
		SuffixesBefore: nonNil(before), SuffixesAfter: nonNil(after),
		RuleExists: slices.ContainsFunc(stored, func(r StoredRule) bool { return r.Base == base && r.ID == rule }),
	}
}

// BaseChangeRow is a base rule's stored change as the read API sends it, for the base policy's
// screens, with whether the base policy holds the rule now.
func BaseChangeRow(id int64, ts pgtype.Timestamptz, actor, action, rule string, before, after []string, exists bool) ChangeRow {
	return ChangeRow{
		ID: id, TS: stamp(ts), Actor: actor, Action: action, RuleID: rule,
		SuffixesBefore: nonNil(before), SuffixesAfter: nonNil(after), RuleExists: exists,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// changeFilters are a read's range and filters as the policy history's statements take them. The
// scope filter keeps the base policy's changes, its null group written none, and the account's own,
// its identifier, and any other value keeps neither.
type changeFilters struct {
	baseRows, ownRows                                       bool
	rangeStart, rangeEnd                                    pgtype.Timestamptz
	actionIn, actionOut, actorIn, actorOut, ruleIn, ruleOut []string
	dayIn, dayOut                                           []pgtype.Date
}

func changeFiltersOf(r Read) (changeFilters, error) {
	f := changeFilters{baseRows: true, ownRows: true, rangeStart: timestamptz(r.Bounds.Start), rangeEnd: timestamptz(r.Bounds.End)}
	f.actionIn, f.actionOut = split(r.Request, "action")
	f.actorIn, f.actorOut = split(r.Request, "actor")
	f.ruleIn, f.ruleOut = split(r.Request, "rule")
	dayIn, dayOut := split(r.Request, "day")
	var err error
	if f.dayIn, err = dates(dayIn); err != nil {
		return changeFilters{}, err
	}
	if f.dayOut, err = dates(dayOut); err != nil {
		return changeFilters{}, err
	}
	if scope, ok := r.Request.Filter("scope"); ok {
		base := slices.Contains(scope.Values, lens.None)
		own := slices.Contains(scope.Values, r.Account)
		if scope.Exclude {
			f.baseRows, f.ownRows = !base, !own
		} else {
			f.baseRows, f.ownRows = base, own
		}
	}
	return f, nil
}

// policyChanges is the policy history, the account's own changes and the base policy's (docs/UI.md
// sections 8.7 and 17.1, ADR-0102).
func policyChanges() Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:        "policy_changes",
			Ranged:      true,
			RangeColumn: "ts",
			Dimensions: []lens.Dimension{
				{Name: "action", Storage: "text", Groupable: true, Filterable: true, Wording: "action", Values: changeActions()},
				{Name: "scope", Storage: "text", Groupable: true, Filterable: true, Wording: "scope", NullWording: "base"},
				{Name: "actor", Storage: "text", Groupable: true, Filterable: true, Wording: "identity"},
				{Name: "day", Storage: "date", Groupable: true, Filterable: true, Wording: "day"},
				{Name: "rule", Storage: "text", Filterable: true, Wording: "rule"},
				{Name: "ts", Storage: "time", Sortable: true, Wording: "time"},
			},
			Default: lens.Defaults{Level: lens.Rows, Range: "30d", Sort: lens.Sort{Column: "ts", Descending: true}},
		},
		Row:     ChangeRowType(),
		Summary: changesSummary,
		Rows:    changesRows,
		Aggregates: map[string]Aggregate{
			"action": changesByAction,
			"scope":  changesByScope,
			"actor":  changesByActor,
			"day":    changesByDay,
		},
	}
}

func changesSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, Total{}, err
	}
	row, err := q.Changes.ChangeFigures(ctx, policychanges.ChangeFiguresParams{
		AccountID: r.Account, BaseRows: f.baseRows, OwnRows: f.ownRows, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		ActionIn: f.actionIn, ActionOut: f.actionOut, ActorIn: f.actorIn, ActorOut: f.actorOut,
		RuleIn: f.ruleIn, RuleOut: f.ruleOut, DayIn: f.dayIn, DayOut: f.dayOut,
	})
	if err != nil {
		return nil, Total{}, err
	}
	history := func(q url.Values) string { return link(r.Account, "policy/history", q) }
	figures := []Figure{
		count("changes", "changes", row.Changes, history(url.Values{"level": {"3"}})),
		count("added", "added restriction", row.Added, history(url.Values{"level": {"3"}, "action": {"added,confirmed"}})),
		count("lifted", "lifted restriction", row.Lifted, history(url.Values{"level": {"3"}, "action": {"lifted"}})),
	}
	return figures, Total{Count: row.Changes}, nil
}

func changesRows(ctx context.Context, q Queries, r Read) (any, error) {
	out := []ChangeRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Changes.ChangeRows(ctx, policychanges.ChangeRowsParams{
		AccountID: r.Account, BaseRows: f.baseRows, OwnRows: f.ownRows, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		ActionIn: f.actionIn, ActionOut: f.actionOut, ActorIn: f.actorIn, ActorOut: f.actorOut,
		RuleIn: f.ruleIn, RuleOut: f.ruleOut, DayIn: f.dayIn, DayOut: f.dayOut,
		Descending: r.Request.Sort.Descending, RowOffset: first,
	})
	if err != nil {
		return nil, err
	}
	stored, err := ComposedRules(ctx, q.Rules, r.Account)
	if err != nil {
		return nil, err
	}
	for _, h := range rows {
		out = append(out, changeRow(h.ID, h.Ts, h.AccountID, h.Actor, h.Action, h.RuleID, h.SuffixesBefore, h.SuffixesAfter, stored))
	}
	return out, nil
}

func changesByAction(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Changes.ChangesByAction(ctx, policychanges.ChangesByActionParams(changeParams(r, f)))
	if err != nil {
		return nil, err
	}
	out := []Group{}
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"action": row.Action}, Count: row.Changes})
	}
	return out, nil
}

func changesByScope(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Changes.ChangesByScope(ctx, policychanges.ChangesByScopeParams(changeParams(r, f)))
	if err != nil {
		return nil, err
	}
	out := []Group{}
	for _, row := range rows {
		var key any
		if row.AccountID.Valid {
			key = row.AccountID.String
		}
		out = append(out, Group{Key: map[string]any{"scope": key}, Count: row.Changes})
	}
	return out, nil
}

func changesByActor(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Changes.ChangesByActor(ctx, policychanges.ChangesByActorParams(changeParams(r, f)))
	if err != nil {
		return nil, err
	}
	out := []Group{}
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"actor": row.Actor}, Count: row.Changes})
	}
	return out, nil
}

func changesByDay(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := changeFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Changes.ChangesByDay(ctx, policychanges.ChangesByDayParams(changeParams(r, f)))
	if err != nil {
		return nil, err
	}
	out := []Group{}
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"day": row.Day.Time.Format(dateLayout)}, Count: row.Changes})
	}
	return out, nil
}

// changeParams is the aggregate statements' shared parameters, which each statement's own parameter
// type converts from, since the four take the same fields.
type changeParamsOf struct {
	AccountID  string
	BaseRows   bool
	OwnRows    bool
	RangeStart pgtype.Timestamptz
	RangeEnd   pgtype.Timestamptz
	ActionIn   []string
	ActionOut  []string
	ActorIn    []string
	ActorOut   []string
	RuleIn     []string
	RuleOut    []string
	DayIn      []pgtype.Date
	DayOut     []pgtype.Date
}

func changeParams(r Read, f changeFilters) changeParamsOf {
	return changeParamsOf{
		AccountID: r.Account, BaseRows: f.baseRows, OwnRows: f.ownRows, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		ActionIn: f.actionIn, ActionOut: f.actionOut, ActorIn: f.actorIn, ActorOut: f.actorOut,
		RuleIn: f.ruleIn, RuleOut: f.ruleOut, DayIn: f.dayIn, DayOut: f.dayOut,
	}
}

// SenderRow is one row of the senders dataset as the sender picker reads it, the sender row's fields
// of docs/UI.md section 7.2 and, for a sender the account's policy restricts, the rule restricting it.
type SenderRow struct {
	Domain       string   `json:"domain"`
	SenderClass  string   `json:"sender_class"`
	MessageCount int64    `json:"message_count"`
	FirstSeen    *string  `json:"first_seen"`
	LastSeen     *string  `json:"last_seen"`
	ListIDRatio  *float64 `json:"list_id_ratio"`
	ScanHits     int64    `json:"scan_hits"`
	RestrictedBy *string  `json:"restricted_by"`
}

func senderRowType() schema.Type {
	return schema.Obj("SenderRow",
		schema.F("domain", schema.Str()),
		schema.F("sender_class", schema.Str()),
		schema.F("message_count", schema.Int()),
		schema.F("first_seen", schema.Null(schema.Time())),
		schema.F("last_seen", schema.Null(schema.Time())),
		schema.F("list_id_ratio", schema.Null(schema.Num())),
		schema.F("scan_hits", schema.Int()),
		schema.F("restricted_by", schema.Null(schema.Str())),
	)
}

// searchOf is the read's search, empty for none.
func searchOf(r Read) string {
	if f, ok := r.Request.Filter("search"); ok {
		return f.Values[0]
	}
	return ""
}

func sendersSearchFigures(r Read) senderclassification.SenderSearchFiguresParams {
	return senderclassification.SenderSearchFiguresParams{AccountID: r.Account, Search: searchOf(r)}
}

func sendersSearchRows(r Read, first int32) senderclassification.SenderSearchRowsParams {
	return senderclassification.SenderSearchRowsParams{AccountID: r.Account, Search: searchOf(r), Descending: r.Request.Sort.Descending, RowOffset: first}
}

// senders is the account's stored senders as the sender picker of docs/UI.md section 8.7 reads them,
// at L0 and L3 under the search filter. The analysis lens over them, with its groupable dimensions and
// its range, is ROADMAP's unit M6's.
func senders(l classify.Lookups) Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name: "senders",
			Dimensions: []lens.Dimension{
				{Name: "search", Storage: "text", Filterable: true, Wording: "search", Search: true},
				{Name: "message_count", Storage: "number", Sortable: true, Wording: "messages"},
			},
			Default: lens.Defaults{Level: lens.Rows, Sort: lens.Sort{Column: "message_count", Descending: true}},
		},
		Row:     senderRowType(),
		Summary: sendersSummary,
		Rows: func(ctx context.Context, q Queries, r Read) (any, error) {
			return senderRows(ctx, q, r, l)
		},
	}
}

func sendersSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	row, err := q.Senders.SenderSearchFigures(ctx, sendersSearchFigures(r))
	if err != nil {
		return nil, Total{}, err
	}
	pick := link(r.Account, "policy/pick", nil)
	return []Figure{
		count("senders", "senders", row.Senders, pick),
		count("messages", "messages", row.Messages, pick),
	}, Total{Count: row.Senders}, nil
}

func senderRows(ctx context.Context, q Queries, r Read, l classify.Lookups) (any, error) {
	out := []SenderRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	rows, err := q.Senders.SenderSearchRows(ctx, sendersSearchRows(r, first))
	if err != nil {
		return nil, err
	}
	stored, err := ComposedRules(ctx, q.Rules, r.Account)
	if err != nil {
		return nil, err
	}
	policy := Composed(stored)
	for _, row := range rows {
		s := SenderRow{
			Domain: row.Domain, SenderClass: row.SenderClass, MessageCount: row.MessageCount,
			FirstSeen: optionalStamp(row.FirstSeen), LastSeen: optionalStamp(row.LastSeen), ScanHits: row.ScanHitCount,
		}
		if row.HasListIDRatio.Valid {
			ratio := Widen(row.HasListIDRatio.Float32)
			s.ListIDRatio = &ratio
		}
		if by := rules.RestrictedBy(row.Domain, policy, l); by != "" {
			s.RestrictedBy = &by
		}
		out = append(out, s)
	}
	return out, nil
}
