package policymanagement

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// The source a rule written by policy management records, and the actions its history rows record
// (ADR-0016, ADR-0102).
const (
	sourceOperator = "operator"
	actionAdded    = "added"
	actionEdited   = "edited"
	actionLifted   = "lifted"
)

// scopeWrites are one scope's statements inside the transaction that runs them, an account's
// transaction for the account's own rules and a base-policy transaction for the base policy's
// (ADR-0112). Each write appends its history row through record in the same transaction (ADR-0102).
type scopeWrites struct {
	stored func(ctx context.Context) ([]registry.StoredRule, error)
	add    func(ctx context.Context, r rules.Rule, actor string) error
	edit   func(ctx context.Context, id string, before, after []string) (int64, error)
	lift   func(ctx context.Context, id string, before []string) (int64, error)
	record func(ctx context.Context, action, id string, before, after []string, actor string) error
}

// accountWrites are the account's own rules' statements, built from the account's transaction.
func accountWrites(rulesQ *manage.Queries, changesQ *policychanges.Queries) scopeWrites {
	return scopeWrites{
		stored: func(ctx context.Context) ([]registry.StoredRule, error) {
			all, err := registry.ComposedRules(ctx, rulesQ)
			return slices.DeleteFunc(all, func(r registry.StoredRule) bool { return r.Base }), err
		},
		add: func(ctx context.Context, r rules.Rule, actor string) error {
			return rulesQ.AddRule(ctx, manage.AddRuleParams{RuleID: r.ID, Class: policy.Restricted, DomainSuffix: r.Suffixes, Source: sourceOperator, CreatedBy: actor})
		},
		edit: func(ctx context.Context, id string, before, after []string) (int64, error) {
			return rulesQ.EditRule(ctx, manage.EditRuleParams{DomainSuffix: after, RuleID: id, SuffixesBefore: before})
		},
		lift: func(ctx context.Context, id string, before []string) (int64, error) {
			return rulesQ.LiftRule(ctx, manage.LiftRuleParams{RuleID: id, SuffixesBefore: before})
		},
		record: func(ctx context.Context, action, id string, before, after []string, actor string) error {
			return changesQ.RecordChange(ctx, policychanges.RecordChangeParams{
				Actor: actor, Action: action, RuleID: id, SuffixesBefore: nonNil(before), SuffixesAfter: nonNil(after),
			})
		},
	}
}

// baseWrites are the base policy's statements, built from the base-policy transaction.
func baseWrites(q *base.Queries) scopeWrites {
	return scopeWrites{
		stored: func(ctx context.Context) ([]registry.StoredRule, error) { return baseStored(ctx, q) },
		add: func(ctx context.Context, r rules.Rule, actor string) error {
			return q.AddBaseRule(ctx, base.AddBaseRuleParams{RuleID: r.ID, Class: policy.Restricted, DomainSuffix: r.Suffixes, Source: sourceOperator, CreatedBy: actor})
		},
		edit: func(ctx context.Context, id string, before, after []string) (int64, error) {
			return q.EditBaseRule(ctx, base.EditBaseRuleParams{DomainSuffix: after, RuleID: id, SuffixesBefore: before})
		},
		lift: func(ctx context.Context, id string, before []string) (int64, error) {
			return q.LiftBaseRule(ctx, base.LiftBaseRuleParams{RuleID: id, SuffixesBefore: before})
		},
		record: func(ctx context.Context, action, id string, before, after []string, actor string) error {
			return q.RecordBaseChange(ctx, base.RecordBaseChangeParams{
				Actor: actor, Action: action, RuleID: id, SuffixesBefore: nonNil(before), SuffixesAfter: nonNil(after),
			})
		},
	}
}

// baseStored reads the base policy's rules.
func baseStored(ctx context.Context, q *base.Queries) ([]registry.StoredRule, error) {
	read, err := q.BaseRules(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.StoredRule, 0, len(read))
	for _, r := range read {
		out = append(out, registry.StoredRule{Base: true, ID: r.RuleID, Class: r.Class, Suffixes: r.DomainSuffix, Source: r.Source, CreatedAt: r.CreatedAt.Time, CreatedBy: r.CreatedBy})
	}
	return out, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// inScope runs fn in the transaction of a scope, the base policy's when base is set and the account's
// otherwise, with that scope's statements (ADR-0112).
func (s *handlers) inScope(ctx context.Context, account string, isBase bool, fn func(w scopeWrites) error) error {
	if isBase {
		return tx.RunBase(ctx, s.opts.Database, func(t pgx.Tx) error { return fn(baseWrites(base.New(t))) })
	}
	return tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		return fn(accountWrites(manage.New(t), policychanges.New(t)))
	})
}

// ProblemAnswer is one problem a refused write or file names, with the rule's identifier, its line in a
// file, and the suffix an invalid suffix names.
type ProblemAnswer struct {
	Kind   string  `json:"kind"`
	RuleID string  `json:"rule_id"`
	Line   *int    `json:"line"`
	Suffix *string `json:"suffix"`
}

func problemType() schema.Type {
	return schema.Obj("Problem",
		schema.F("kind", schema.Str(kinds()...)),
		schema.F("rule_id", schema.Str()),
		schema.F("line", schema.Null(schema.Int())),
		schema.F("suffix", schema.Null(schema.Str())),
	)
}

// kinds are the problems' kinds as the contract declares them.
func kinds() []string {
	var out []string
	for _, k := range refusal.Kinds() {
		out = append(out, string(k))
	}
	return out
}

// refusalAnswer is a refused write's body, the error contract's shape with the problems beside it.
type refusalAnswer struct {
	Error    server.ErrorDetail `json:"error"`
	Problems []ProblemAnswer    `json:"problems"`
}

// RefusalType is the declaration of a refused policy write's or file's body.
func RefusalType() schema.Type {
	return schema.Obj("Refusal",
		schema.F("error", server.ErrorDetailType()),
		schema.F("problems", schema.ArrayOf(problemType())),
	)
}

// writeRefusal answers a request whose rules or file fail the checks with each problem named.
func writeRefusal(w http.ResponseWriter, r *http.Request, refused *refusal.Refusal) {
	problems := make([]ProblemAnswer, 0, len(refused.Problems))
	for _, p := range refused.Problems {
		a := ProblemAnswer{Kind: string(p.Kind), RuleID: p.Rule}
		if p.Line > 0 {
			line := p.Line
			a.Line = &line
		}
		if p.Suffix != "" {
			suffix := p.Suffix
			a.Suffix = &suffix
		}
		problems = append(problems, a)
	}
	server.WriteBody(w, r, server.Status(refused.Code), refusalAnswer{
		Error:    server.ClientRefusal(r, refused),
		Problems: problems,
	})
}

// answerWrite answers a policy write that failed, by what failed it, a refusal the rules decided
// with its problems beside it when it names any.
func (s *handlers) answerWrite(w http.ResponseWriter, r *http.Request, err error) {
	var refused *refusal.Refusal
	switch {
	case errors.As(err, &refused) && len(refused.Problems) > 0:
		writeRefusal(w, r, refused)
	case errors.As(err, &refused):
		server.WriteFailure(w, r, server.Refused(refused))
	default:
		server.DatabaseFailure(w, r, err)
	}
}

// RuleRequest adds a rule (docs/UI.md section 17.4). Scope is account or base on an account's request,
// and absent on the base policy's.
type RuleRequest struct {
	Scope    string   `json:"scope"`
	RuleID   string   `json:"rule_id"`
	Suffixes []string `json:"suffixes"`
}

// EditRequest edits a rule's suffixes, or with Suffixes absent, on the lift route, lifts it.
type EditRequest struct {
	Scope          string   `json:"scope"`
	SuffixesBefore []string `json:"suffixes_before"`
	Suffixes       []string `json:"suffixes"`
	Confirmation   *string  `json:"confirmation"`
}

// LiftRequest lifts a rule.
type LiftRequest struct {
	Scope          string   `json:"scope"`
	SuffixesBefore []string `json:"suffixes_before"`
	Confirmation   *string  `json:"confirmation"`
}

func scopeField(withScope bool) []schema.Field {
	if withScope {
		return []schema.Field{schema.F("scope", schema.Str(rules.Account, rules.Base))}
	}
	return nil
}

func ruleRequestType(withScope bool, name string) schema.Type {
	return schema.Obj(name, append(scopeField(withScope),
		schema.F("rule_id", schema.Str()),
		schema.F("suffixes", schema.ArrayOf(schema.Str())),
	)...)
}

func editRequestType(withScope bool, name string) schema.Type {
	return schema.Obj(name, append(scopeField(withScope),
		schema.F("suffixes_before", schema.ArrayOf(schema.Str())),
		schema.F("suffixes", schema.ArrayOf(schema.Str())),
		schema.F("confirmation", schema.Null(schema.Str())),
	)...)
}

func liftRequestType(withScope bool, name string) schema.Type {
	return schema.Obj(name, append(scopeField(withScope),
		schema.F("suffixes_before", schema.ArrayOf(schema.Str())),
		schema.F("confirmation", schema.Null(schema.Str())),
	)...)
}

// RuleAnswer is a write's answer, the rule as the write left it, its suffixes empty once lifted.
type RuleAnswer struct {
	Scope    string   `json:"scope"`
	RuleID   string   `json:"rule_id"`
	Suffixes []string `json:"suffixes"`
}

func ruleAnswerType() schema.Type {
	return schema.Obj("RuleAnswer",
		schema.F("scope", schema.Str(rules.Base, rules.Account)),
		schema.F("rule_id", schema.Str()),
		schema.F("suffixes", schema.ArrayOf(schema.Str())),
	)
}

// toRules returns stored rules as the pure rules take them, in stored order.
func toRules(stored []registry.StoredRule) []rules.Rule {
	out := make([]rules.Rule, len(stored))
	for i, r := range stored {
		out[i] = rules.Rule{ID: r.ID, Suffixes: r.Suffixes}
	}
	return out
}

// addRule adds a rule to a scope with its history row, in one transaction (ADR-0102). An identifier the
// scope holds is refused as taken, and a rule the policy's checks refuse is refused with its problems,
// both before anything is written.
func (s *handlers) addRule(ctx context.Context, account string, isBase bool, req RuleRequest, actor string) error {
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		rule := rules.Rule{ID: req.RuleID, Suffixes: nonNil(req.Suffixes)}
		if refused := rules.Adding(toRules(stored), rule); refused != nil {
			return refused
		}
		if err := w.add(ctx, rule, actor); err != nil {
			if key, ok := server.Violated(err, server.UniqueViolation); ok && key == ruleKey {
				return rules.Taken()
			}
			return err
		}
		return w.record(ctx, actionAdded, rule.ID, nil, rule.Suffixes, actor)
	})
}

// ruleKey is the policy rules' key, which refuses an identifier its scope holds (ADR-0110).
const ruleKey = "policy_rules_scope_rule_id_key"

// editRule sets a rule's suffixes with its history row, in one transaction. The rule is read first, so
// a rule the scope does not hold is reported as unknown and never as a stale conflict (ADR-0084). The
// suffixes stored must be the ones the request was made against. An edit removing a suffix of a base
// rule needs the rule's identifier as its confirmation, an edit removing every suffix is refused, since
// a rule needs one, and an edit the policy's checks refuse is refused.
func (s *handlers) editRule(ctx context.Context, account string, isBase bool, id string, req EditRequest, actor string) error {
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		if refused := rules.Editing(toRules(stored), id, req.SuffixesBefore, nonNil(req.Suffixes), isBase, req.Confirmation); refused != nil {
			return refused
		}
		n, err := w.edit(ctx, id, req.SuffixesBefore, req.Suffixes)
		if err != nil {
			return err
		}
		if n == 0 {
			return rules.StaleRule()
		}
		return w.record(ctx, actionEdited, id, req.SuffixesBefore, req.Suffixes, actor)
	})
}

// liftRule removes a rule with its history row, in one transaction, read first as an edit is. A base
// rule's lift needs the rule's identifier as its confirmation.
func (s *handlers) liftRule(ctx context.Context, account string, isBase bool, id string, req LiftRequest, actor string) error {
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		if refused := rules.Lifting(toRules(stored), id, req.SuffixesBefore, isBase, req.Confirmation); refused != nil {
			return refused
		}
		n, err := w.lift(ctx, id, req.SuffixesBefore)
		if err != nil {
			return err
		}
		if n == 0 {
			return rules.StaleRule()
		}
		return w.record(ctx, actionLifted, id, req.SuffixesBefore, nil, actor)
	})
}

func scopeOf(isBase bool) string {
	if isBase {
		return rules.Base
	}
	return rules.Account
}

// scopeOfRequest reads an account's request's scope, account or base, and answers the request when it
// is neither.
func scopeOfRequest(w http.ResponseWriter, r *http.Request, scope string) (bool, bool) {
	switch scope {
	case rules.Account:
		return false, true
	case rules.Base:
		return true, true
	}
	server.WriteFailure(w, r, server.ClientFault(refusal.MalformedBody, "scope is account or base"))
	return false, false
}

// postAccountRule is POST /ui/api/accounts/{account}/policy/rules.
func (s *handlers) postAccountRule(w http.ResponseWriter, r *http.Request) {
	var body RuleRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	isBase, ok := scopeOfRequest(w, r, body.Scope)
	if !ok {
		return
	}
	s.writeAdd(w, r, r.PathValue("account"), isBase, body)
}

// The base policy's requests, which name no scope since theirs is always the base policy.
type (
	baseRuleRequest struct {
		RuleID   string   `json:"rule_id"`
		Suffixes []string `json:"suffixes"`
	}
	baseEditRequest struct {
		SuffixesBefore []string `json:"suffixes_before"`
		Suffixes       []string `json:"suffixes"`
		Confirmation   *string  `json:"confirmation"`
	}
	baseLiftRequest struct {
		SuffixesBefore []string `json:"suffixes_before"`
		Confirmation   *string  `json:"confirmation"`
	}
)

// postBaseRule is POST /ui/api/setup/policy/rules.
func (s *handlers) postBaseRule(w http.ResponseWriter, r *http.Request) {
	var body baseRuleRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	s.writeAdd(w, r, "", true, RuleRequest{Scope: rules.Base, RuleID: body.RuleID, Suffixes: body.Suffixes})
}

func (s *handlers) writeAdd(w http.ResponseWriter, r *http.Request, account string, isBase bool, body RuleRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	if err := s.addRule(r.Context(), account, isBase, body, actor); err != nil {
		s.answerWrite(w, r, err)
		return
	}
	server.WriteJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: body.RuleID, Suffixes: nonNil(body.Suffixes)})
}

// postAccountEdit is POST /ui/api/accounts/{account}/policy/rules/{rule}.
func (s *handlers) postAccountEdit(w http.ResponseWriter, r *http.Request) {
	var body EditRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	isBase, ok := scopeOfRequest(w, r, body.Scope)
	if !ok {
		return
	}
	s.writeEdit(w, r, r.PathValue("account"), isBase, body)
}

// postBaseEdit is POST /ui/api/setup/policy/rules/{rule}.
func (s *handlers) postBaseEdit(w http.ResponseWriter, r *http.Request) {
	var body baseEditRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	s.writeEdit(w, r, "", true, EditRequest{Scope: rules.Base, SuffixesBefore: body.SuffixesBefore, Suffixes: body.Suffixes, Confirmation: body.Confirmation})
}

func (s *handlers) writeEdit(w http.ResponseWriter, r *http.Request, account string, isBase bool, body EditRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	id := r.PathValue("rule")
	if err := s.editRule(r.Context(), account, isBase, id, body, actor); err != nil {
		s.answerWrite(w, r, err)
		return
	}
	server.WriteJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: id, Suffixes: nonNil(body.Suffixes)})
}

// postAccountLift is POST /ui/api/accounts/{account}/policy/rules/{rule}/lift.
func (s *handlers) postAccountLift(w http.ResponseWriter, r *http.Request) {
	var body LiftRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	isBase, ok := scopeOfRequest(w, r, body.Scope)
	if !ok {
		return
	}
	s.writeLift(w, r, r.PathValue("account"), isBase, body)
}

// postBaseLift is POST /ui/api/setup/policy/rules/{rule}/lift.
func (s *handlers) postBaseLift(w http.ResponseWriter, r *http.Request) {
	var body baseLiftRequest
	if !server.ReadBody(w, r, &body) {
		return
	}
	s.writeLift(w, r, "", true, LiftRequest{Scope: rules.Base, SuffixesBefore: body.SuffixesBefore, Confirmation: body.Confirmation})
}

func (s *handlers) writeLift(w http.ResponseWriter, r *http.Request, account string, isBase bool, body LiftRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	id := r.PathValue("rule")
	if err := s.liftRule(r.Context(), account, isBase, id, body, actor); err != nil {
		s.answerWrite(w, r, err)
		return
	}
	server.WriteJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: id, Suffixes: []string{}})
}

// SuffixMatch is what one suffix typed in Add a rule matches in the account (docs/UI.md section 8.7).
type SuffixMatch struct {
	Suffix       string     `json:"suffix"`
	Valid        bool       `json:"valid"`
	PublicSuffix bool       `json:"public_suffix"`
	Senders      int64      `json:"senders"`
	Messages     int64      `json:"messages"`
	Rules        []RuleName `json:"rules"`
}

// RuleName names a rule of the account's policy by its scope and identifier.
type RuleName struct {
	Scope  string `json:"scope"`
	RuleID string `json:"rule_id"`
}

// MatchAnswer is the add panel's read, each suffix's match, what every suffix together matches, a
// sender two of them match counting once, and the account's count of senders, which a share is taken
// of.
type MatchAnswer struct {
	SendersTotal int64           `json:"senders_total"`
	Together     registry.Counts `json:"together"`
	Suffixes     []SuffixMatch   `json:"suffixes"`
}

func matchAnswerType() schema.Type {
	return schema.Obj("MatchAnswer",
		schema.F("senders_total", schema.Int()),
		schema.F("together", registry.CountsType()),
		schema.F("suffixes", schema.ArrayOf(schema.Obj("SuffixMatch",
			schema.F("suffix", schema.Str()),
			schema.F("valid", schema.Bool()),
			schema.F("public_suffix", schema.Bool()),
			schema.F("senders", schema.Int()),
			schema.F("messages", schema.Int()),
			schema.F("rules", schema.ArrayOf(ruleNameType())),
		))),
	)
}

func ruleNameType() schema.Type {
	return schema.Obj("RuleName", schema.F("scope", schema.Str(rules.Base, rules.Account)), schema.F("rule_id", schema.Str()))
}

// getMatch is GET /ui/api/accounts/{account}/policy/match?suffix=…, which the add panel reads as each line is
// typed. A suffix is valid when the policy's checks would take it, and each rule of the account's
// policy one of whose suffixes covers it is named.
func (s *handlers) getMatch(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	suffixes := r.URL.Query()["suffix"]
	out := MatchAnswer{Suffixes: []SuffixMatch{}}
	err := s.inAccount(r.Context(), account, func(q registry.Queries) error {
		stored, err := registry.ComposedRules(r.Context(), q.Rules)
		if err != nil {
			return err
		}
		senders, err := registry.AccountSenders(r.Context(), q.Senders, stored, s.opts.Lookups)
		if err != nil {
			return err
		}
		names := make([][]classify.Name, len(stored))
		for i, rule := range stored {
			names[i] = rules.SuffixNames(rule.Suffixes, s.opts.Lookups)
		}
		out.SendersTotal = senders.Count()
		together := senders.Match(suffixes)
		out.Together = registry.Counts{Senders: together.Senders, Messages: together.Messages}
		for _, suffix := range suffixes {
			c := senders.Match([]string{suffix})
			m := SuffixMatch{
				Suffix: suffix, Valid: len(rules.Check([]rules.Rule{{ID: "x", Suffixes: []string{suffix}}})) == 0,
				PublicSuffix: rules.PublicSuffix(suffix, s.opts.Lookups), Senders: c.Senders, Messages: c.Messages, Rules: []RuleName{},
			}
			for i, rule := range stored {
				if rules.Covers(names[i], suffix, s.opts.Lookups) {
					m.Rules = append(m.Rules, RuleName{Scope: scopeOf(rule.Base), RuleID: rule.ID})
				}
			}
			out.Suffixes = append(out.Suffixes, m)
		}
		return nil
	})
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	server.WriteJSON(w, r, out)
}

// BaseSuffixMatch is what one suffix typed in Add a base rule is, read with no account named. It
// says whether the policy's checks take it, whether it is itself a public suffix, and the base
// rules already matching it (docs/UI.md section 8.14). It carries no count, since counts are each
// account's.
type BaseSuffixMatch struct {
	Suffix       string     `json:"suffix"`
	Valid        bool       `json:"valid"`
	PublicSuffix bool       `json:"public_suffix"`
	Rules        []RuleName `json:"rules"`
}

// BaseMatchAnswer is the base add panel's read.
type BaseMatchAnswer struct {
	Suffixes []BaseSuffixMatch `json:"suffixes"`
}

func baseMatchAnswerType() schema.Type {
	return schema.Obj("BaseMatchAnswer", schema.F("suffixes", schema.ArrayOf(schema.Obj("BaseSuffixMatch",
		schema.F("suffix", schema.Str()),
		schema.F("valid", schema.Bool()),
		schema.F("public_suffix", schema.Bool()),
		schema.F("rules", schema.ArrayOf(ruleNameType())),
	))))
}

// getBaseMatch is GET /ui/api/setup/policy/match?suffix=…, read in a base-policy transaction, which names
// no account (ADR-0112).
func (s *handlers) getBaseMatch(w http.ResponseWriter, r *http.Request) {
	suffixes := r.URL.Query()["suffix"]
	out := BaseMatchAnswer{Suffixes: []BaseSuffixMatch{}}
	err := tx.RunBase(r.Context(), s.opts.Database, func(t pgx.Tx) error {
		stored, err := baseStored(r.Context(), base.New(t))
		if err != nil {
			return err
		}
		names := make([][]classify.Name, len(stored))
		for i, rule := range stored {
			names[i] = rules.SuffixNames(rule.Suffixes, s.opts.Lookups)
		}
		for _, suffix := range suffixes {
			m := BaseSuffixMatch{
				Suffix: suffix, Valid: len(rules.Check([]rules.Rule{{ID: "x", Suffixes: []string{suffix}}})) == 0,
				PublicSuffix: rules.PublicSuffix(suffix, s.opts.Lookups), Rules: []RuleName{},
			}
			for i, rule := range stored {
				if rules.Covers(names[i], suffix, s.opts.Lookups) {
					m.Rules = append(m.Rules, RuleName{Scope: rules.Base, RuleID: rule.ID})
				}
			}
			out.Suffixes = append(out.Suffixes, m)
		}
		return nil
	})
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return
	}
	server.WriteJSON(w, r, out)
}

// getRelease is GET /ui/api/accounts/{account}/policy/release?scope=…&rule=…&keep=…, what an edit keeping only
// keep of a rule's suffixes, or a lift keeping none, would release in the account, the senders the
// account's policy restricts now and would not after (docs/UI.md section 8.7). A rule's detail carries
// each suffix's release alone, and an edit removing several reads the set's here, since a sender two
// removed suffixes both match is released only by removing both.
func (s *handlers) getRelease(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	q := r.URL.Query()
	isBase, ok := scopeOfRequest(w, r, q.Get("scope"))
	if !ok {
		return
	}
	id := q.Get("rule")
	var out registry.Counts
	err := s.inAccount(r.Context(), account, func(rq registry.Queries) error {
		stored, err := registry.ComposedRules(r.Context(), rq.Rules)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(stored, func(x registry.StoredRule) bool { return x.Base == isBase && x.ID == id }) {
			return rules.UnknownRule()
		}
		senders, err := registry.AccountSenders(r.Context(), rq.Senders, stored, s.opts.Lookups)
		if err != nil {
			return err
		}
		out = registry.RuleRelease(stored, senders, isBase, id, q["keep"])
		return nil
	})
	if err != nil {
		s.answerWrite(w, r, err)
		return
	}
	server.WriteJSON(w, r, out)
}

// stampOf is an optional stored time as the read API writes it.
func stampOf(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC().Format(time.RFC3339)
	return &v
}
