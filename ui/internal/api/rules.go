package api

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/rules"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
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
func accountWrites(rulesQ *manage.Queries, changesQ *policychanges.Queries, account string) scopeWrites {
	scope := pgtype.Text{String: account, Valid: true}
	return scopeWrites{
		stored: func(ctx context.Context) ([]registry.StoredRule, error) {
			all, err := registry.ComposedRules(ctx, rulesQ, account)
			return slices.DeleteFunc(all, func(r registry.StoredRule) bool { return r.Base }), err
		},
		add: func(ctx context.Context, r rules.Rule, actor string) error {
			return rulesQ.AddRule(ctx, manage.AddRuleParams{AccountID: scope, RuleID: r.ID, Class: policy.Restricted, DomainSuffix: r.Suffixes, Source: sourceOperator, CreatedBy: actor})
		},
		edit: func(ctx context.Context, id string, before, after []string) (int64, error) {
			return rulesQ.EditRule(ctx, manage.EditRuleParams{DomainSuffix: after, AccountID: scope, RuleID: id, SuffixesBefore: before})
		},
		lift: func(ctx context.Context, id string, before []string) (int64, error) {
			return rulesQ.LiftRule(ctx, manage.LiftRuleParams{AccountID: scope, RuleID: id, SuffixesBefore: before})
		},
		record: func(ctx context.Context, action, id string, before, after []string, actor string) error {
			return changesQ.RecordChange(ctx, policychanges.RecordChangeParams{
				AccountID: scope, Actor: actor, Action: action, RuleID: id, SuffixesBefore: nonNil(before), SuffixesAfter: nonNil(after),
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
func (s *Server) inScope(ctx context.Context, account string, isBase bool, fn func(w scopeWrites) error) error {
	if isBase {
		return tx.RunBase(ctx, s.opts.Database, func(t pgx.Tx) error { return fn(baseWrites(base.New(t))) })
	}
	return tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		return fn(accountWrites(manage.New(t), policychanges.New(t), account))
	})
}

// The refusals a policy write answers with, each the client's fault and nothing written (docs/UI.md
// section 17.4).
var (
	errIdentifierTaken      = errors.New("the scope holds a rule of this identifier")
	errUnknownRule          = errors.New("the scope holds no rule of this identifier")
	errStaleRule            = errors.New("the rule's suffixes are not the ones the request was made against")
	errConfirmationRequired = errors.New("the change lifts a restriction every account loses, and its confirmation is not the rule's identifier")
)

// refusedRule is a write the policy snapshot's validation, or the UI's own check, refuses.
type refusedRule struct{ problems []rules.Problem }

func (r *refusedRule) Error() string { return "the rule fails the policy's checks" }

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
		schema.F("kind", schema.Str(string(rules.BlankIdentifier), string(rules.RepeatedIdentifier), string(rules.NoSuffix),
			string(rules.InvalidSuffix), string(rules.OtherClass), string(rules.ReservedIdentifier), kindNotTheForm, kindTooLarge)),
		schema.F("rule_id", schema.Str()),
		schema.F("line", schema.Null(schema.Int())),
		schema.F("suffix", schema.Null(schema.Str())),
	)
}

// refusalAnswer is a refused write's body, the error contract's shape with the problems beside it.
type refusalAnswer struct {
	Error    errorDetail     `json:"error"`
	Problems []ProblemAnswer `json:"problems"`
}

// RefusalType is the declaration of a refused policy write's or file's body.
func RefusalType() schema.Type {
	return schema.Obj("Refusal",
		schema.F("error", schema.Obj("ErrorDetail",
			schema.F("origin", schema.Str(originClient, originUI, originDatabase, originProvider)),
			schema.F("code", schema.Str()),
			schema.F("message", schema.Str()),
			schema.F("request_id", schema.Str()),
		)),
		schema.F("problems", schema.ArrayOf(problemType())),
	)
}

// writeRefusal answers a request whose rules fail the checks with each problem named.
func writeRefusal(w http.ResponseWriter, r *http.Request, code, message string, problems []ProblemAnswer) {
	info := requestInfo(r.Context())
	info.origin, info.code = originClient, code
	writeBody(w, r, http.StatusBadRequest, refusalAnswer{
		Error:    errorDetail{Origin: originClient, Code: code, Message: message, RequestID: info.id},
		Problems: problems,
	})
}

// answerWrite answers a policy write that failed, by what failed it.
func (s *Server) answerWrite(w http.ResponseWriter, r *http.Request, rule string, err error) {
	var refused *refusedRule
	switch {
	case errors.As(err, &refused):
		var out []ProblemAnswer
		for _, p := range refused.problems {
			a := ProblemAnswer{Kind: string(p.Kind), RuleID: rule}
			if p.Suffix != "" {
				suffix := p.Suffix
				a.Suffix = &suffix
			}
			out = append(out, a)
		}
		writeRefusal(w, r, "rule_refused", "the rule fails the policy's checks", out)
	case errors.Is(err, errIdentifierTaken):
		writeFailure(w, r, clientFault(http.StatusConflict, "identifier_taken", "the scope already holds a rule of this identifier"))
	case errors.Is(err, errUnknownRule):
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_rule", "the policy holds no rule of this identifier in this scope"))
	case errors.Is(err, errStaleRule):
		writeFailure(w, r, clientFault(http.StatusConflict, "stale_rule", "the rule changed since the page read it"))
	case errors.Is(err, errConfirmationRequired):
		writeFailure(w, r, clientFault(http.StatusBadRequest, "confirmation_required", "lifting a base rule's restriction needs the rule's identifier typed"))
	default:
		s.databaseFailure(w, r, err)
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

// checked refuses a scope's rules as a write would leave them, when the policy's checks find a problem
// with the rule at index i, the one the write changes.
func checked(scope string, after []rules.Rule, i int) error {
	var mine []rules.Problem
	for _, p := range rules.Check(scope, after) {
		if p.Rule == i {
			mine = append(mine, p)
		}
	}
	if len(mine) > 0 {
		return &refusedRule{problems: mine}
	}
	return nil
}

// addRule adds a rule to a scope with its history row, in one transaction (ADR-0102). An identifier the
// scope holds is refused as taken, and a rule the policy's checks refuse is refused with its problems,
// both before anything is written.
func (s *Server) addRule(ctx context.Context, account string, isBase bool, req RuleRequest, actor string) error {
	scope := scopeOf(isBase)
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(stored, func(r registry.StoredRule) bool { return r.ID == req.RuleID }) {
			return errIdentifierTaken
		}
		rule := rules.Rule{ID: req.RuleID, Suffixes: nonNil(req.Suffixes)}
		after := append(toRules(stored), rule)
		if err := checked(scope, after, len(after)-1); err != nil {
			return err
		}
		if err := w.add(ctx, rule, actor); err != nil {
			if key, ok := violated(err, uniqueViolation); ok && key == ruleKey {
				return errIdentifierTaken
			}
			return err
		}
		if err := s.fault("between a rule's write and its history row"); err != nil {
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
func (s *Server) editRule(ctx context.Context, account string, isBase bool, id string, req EditRequest, actor string) error {
	scope := scopeOf(isBase)
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(stored, func(r registry.StoredRule) bool { return r.ID == id })
		if i < 0 {
			return errUnknownRule
		}
		if !slices.Equal(stored[i].Suffixes, req.SuffixesBefore) {
			return errStaleRule
		}
		after := toRules(stored)
		after[i] = rules.Rule{ID: id, Suffixes: nonNil(req.Suffixes)}
		if err := checked(scope, after, i); err != nil {
			return err
		}
		removes := slices.ContainsFunc(stored[i].Suffixes, func(x string) bool { return !slices.Contains(req.Suffixes, x) })
		if isBase && removes && !confirms(req.Confirmation, id) {
			return errConfirmationRequired
		}
		n, err := w.edit(ctx, id, stored[i].Suffixes, req.Suffixes)
		if err != nil {
			return err
		}
		if n == 0 {
			return errStaleRule
		}
		if err := s.fault("between a rule's write and its history row"); err != nil {
			return err
		}
		return w.record(ctx, actionEdited, id, stored[i].Suffixes, req.Suffixes, actor)
	})
}

// liftRule removes a rule with its history row, in one transaction, read first as an edit is. A base
// rule's lift needs the rule's identifier as its confirmation.
func (s *Server) liftRule(ctx context.Context, account string, isBase bool, id string, req LiftRequest, actor string) error {
	return s.inScope(ctx, account, isBase, func(w scopeWrites) error {
		stored, err := w.stored(ctx)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(stored, func(r registry.StoredRule) bool { return r.ID == id })
		if i < 0 {
			return errUnknownRule
		}
		if !slices.Equal(stored[i].Suffixes, req.SuffixesBefore) {
			return errStaleRule
		}
		if isBase && !confirms(req.Confirmation, id) {
			return errConfirmationRequired
		}
		n, err := w.lift(ctx, id, stored[i].Suffixes)
		if err != nil {
			return err
		}
		if n == 0 {
			return errStaleRule
		}
		if err := s.fault("between a rule's write and its history row"); err != nil {
			return err
		}
		return w.record(ctx, actionLifted, id, stored[i].Suffixes, nil, actor)
	})
}

// confirms reports whether a typed confirmation is want, compared after trimming surrounding space as
// the screen's field compares it (docs/UI.md section 8.7).
func confirms(confirmation *string, want string) bool {
	return confirmation != nil && strings.TrimSpace(*confirmation) == want
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
	writeFailure(w, r, clientFault(http.StatusBadRequest, "malformed_body", "scope is account or base"))
	return false, false
}

// postAccountRule is POST /api/{account}/policy/rules.
func (s *Server) postAccountRule(w http.ResponseWriter, r *http.Request) {
	var body RuleRequest
	if !readBody(w, r, &body) {
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

// postBaseRule is POST /api/setup/policy/rules.
func (s *Server) postBaseRule(w http.ResponseWriter, r *http.Request) {
	var body baseRuleRequest
	if !readBody(w, r, &body) {
		return
	}
	s.writeAdd(w, r, "", true, RuleRequest{Scope: rules.Base, RuleID: body.RuleID, Suffixes: body.Suffixes})
}

func (s *Server) writeAdd(w http.ResponseWriter, r *http.Request, account string, isBase bool, body RuleRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	if err := s.addRule(r.Context(), account, isBase, body, actor); err != nil {
		s.answerWrite(w, r, body.RuleID, err)
		return
	}
	writeJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: body.RuleID, Suffixes: nonNil(body.Suffixes)})
}

// postAccountEdit is POST /api/{account}/policy/rules/{rule}.
func (s *Server) postAccountEdit(w http.ResponseWriter, r *http.Request) {
	var body EditRequest
	if !readBody(w, r, &body) {
		return
	}
	isBase, ok := scopeOfRequest(w, r, body.Scope)
	if !ok {
		return
	}
	s.writeEdit(w, r, r.PathValue("account"), isBase, body)
}

// postBaseEdit is POST /api/setup/policy/rules/{rule}.
func (s *Server) postBaseEdit(w http.ResponseWriter, r *http.Request) {
	var body baseEditRequest
	if !readBody(w, r, &body) {
		return
	}
	s.writeEdit(w, r, "", true, EditRequest{Scope: rules.Base, SuffixesBefore: body.SuffixesBefore, Suffixes: body.Suffixes, Confirmation: body.Confirmation})
}

func (s *Server) writeEdit(w http.ResponseWriter, r *http.Request, account string, isBase bool, body EditRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	id := r.PathValue("rule")
	if err := s.editRule(r.Context(), account, isBase, id, body, actor); err != nil {
		s.answerWrite(w, r, id, err)
		return
	}
	writeJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: id, Suffixes: nonNil(body.Suffixes)})
}

// postAccountLift is POST /api/{account}/policy/rules/{rule}/lift.
func (s *Server) postAccountLift(w http.ResponseWriter, r *http.Request) {
	var body LiftRequest
	if !readBody(w, r, &body) {
		return
	}
	isBase, ok := scopeOfRequest(w, r, body.Scope)
	if !ok {
		return
	}
	s.writeLift(w, r, r.PathValue("account"), isBase, body)
}

// postBaseLift is POST /api/setup/policy/rules/{rule}/lift.
func (s *Server) postBaseLift(w http.ResponseWriter, r *http.Request) {
	var body baseLiftRequest
	if !readBody(w, r, &body) {
		return
	}
	s.writeLift(w, r, "", true, LiftRequest{Scope: rules.Base, SuffixesBefore: body.SuffixesBefore, Confirmation: body.Confirmation})
}

func (s *Server) writeLift(w http.ResponseWriter, r *http.Request, account string, isBase bool, body LiftRequest) {
	actor, ok := s.identity(w, r)
	if !ok {
		return
	}
	id := r.PathValue("rule")
	if err := s.liftRule(r.Context(), account, isBase, id, body, actor); err != nil {
		s.answerWrite(w, r, id, err)
		return
	}
	writeJSON(w, r, RuleAnswer{Scope: scopeOf(isBase), RuleID: id, Suffixes: []string{}})
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
		schema.F("together", schema.Obj("Counts", schema.F("senders", schema.Int()), schema.F("messages", schema.Int()))),
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

// getMatch is GET /api/{account}/policy/match?suffix=…, which the add panel reads as each line is
// typed. A suffix is valid when the policy's checks would take it, and each rule of the account's
// policy one of whose suffixes covers it is named.
func (s *Server) getMatch(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	suffixes := r.URL.Query()["suffix"]
	out := MatchAnswer{Suffixes: []SuffixMatch{}}
	err := s.inAccount(r.Context(), account, func(q registry.Queries) error {
		stored, err := registry.ComposedRules(r.Context(), q.Rules, account)
		if err != nil {
			return err
		}
		senders, _, err := registry.SenderClasses(r.Context(), q, account, s.opts.Lookups)
		if err != nil {
			return err
		}
		out.SendersTotal = senders.Count()
		together := senders.Match(suffixes)
		out.Together = registry.Counts{Senders: together.Senders, Messages: together.Messages}
		for _, suffix := range suffixes {
			c := senders.Match([]string{suffix})
			m := SuffixMatch{
				Suffix: suffix, Valid: len(rules.Check(rules.Account, []rules.Rule{{ID: "x", Suffixes: []string{suffix}}})) == 0,
				PublicSuffix: rules.PublicSuffix(suffix, s.opts.Lookups), Senders: c.Senders, Messages: c.Messages, Rules: []RuleName{},
			}
			for _, rule := range stored {
				if rules.Covers(rule.Suffixes, suffix, s.opts.Lookups) {
					m.Rules = append(m.Rules, RuleName{Scope: scopeOf(rule.Base), RuleID: rule.ID})
				}
			}
			out.Suffixes = append(out.Suffixes, m)
		}
		return nil
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// BaseSuffixMatch is what one suffix typed in Add a base rule is, read with no account named: whether
// the policy's checks take it, whether it is itself a public suffix, and the base rules already
// matching it (docs/UI.md section 8.14). It carries no count, since counts are each account's.
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

// getBaseMatch is GET /api/setup/policy/match?suffix=…, read in a base-policy transaction, which names
// no account (ADR-0112).
func (s *Server) getBaseMatch(w http.ResponseWriter, r *http.Request) {
	suffixes := r.URL.Query()["suffix"]
	out := BaseMatchAnswer{Suffixes: []BaseSuffixMatch{}}
	err := tx.RunBase(r.Context(), s.opts.Database, func(t pgx.Tx) error {
		stored, err := baseStored(r.Context(), base.New(t))
		if err != nil {
			return err
		}
		for _, suffix := range suffixes {
			m := BaseSuffixMatch{
				Suffix: suffix, Valid: len(rules.Check(rules.Base, []rules.Rule{{ID: "x", Suffixes: []string{suffix}}})) == 0,
				PublicSuffix: rules.PublicSuffix(suffix, s.opts.Lookups), Rules: []RuleName{},
			}
			for _, rule := range stored {
				if rules.Covers(rule.Suffixes, suffix, s.opts.Lookups) {
					m.Rules = append(m.Rules, RuleName{Scope: rules.Base, RuleID: rule.ID})
				}
			}
			out.Suffixes = append(out.Suffixes, m)
		}
		return nil
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// getRelease is GET /api/{account}/policy/release?scope=…&rule=…&keep=…, what an edit keeping only
// keep of a rule's suffixes, or a lift keeping none, would release in the account, the senders the
// account's policy restricts now and would not after (docs/UI.md section 8.7). A rule's detail carries
// each suffix's release alone, and an edit removing several reads the set's here, since a sender two
// removed suffixes both match is released only by removing both.
func (s *Server) getRelease(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	q := r.URL.Query()
	isBase, ok := scopeOfRequest(w, r, q.Get("scope"))
	if !ok {
		return
	}
	id := q.Get("rule")
	var out registry.Counts
	err := s.inAccount(r.Context(), account, func(rq registry.Queries) error {
		stored, err := registry.ComposedRules(r.Context(), rq.Rules, account)
		if err != nil {
			return err
		}
		if !slices.ContainsFunc(stored, func(x registry.StoredRule) bool { return x.Base == isBase && x.ID == id }) {
			return errUnknownRule
		}
		senders, _, err := registry.SenderClasses(r.Context(), rq, account, s.opts.Lookups)
		if err != nil {
			return err
		}
		out = registry.RuleRelease(stored, senders, isBase, id, q["keep"])
		return nil
	})
	if err != nil {
		s.answerWrite(w, r, id, err)
		return
	}
	writeJSON(w, r, out)
}

// stampOf is an optional stored time as the read API writes it.
func stampOf(t pgtype.Timestamptz) *string {
	if !t.Valid {
		return nil
	}
	v := t.Time.UTC().Format(time.RFC3339)
	return &v
}
