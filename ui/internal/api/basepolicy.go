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

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/policychanges"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"
	"github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	senderclasses "github.com/ppat/mediated-mailbox-mcp/db/senders/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// inAccount runs fn in the account's transaction with every statement the registry and the policy
// reads run, built from that transaction.
func (s *Server) inAccount(ctx context.Context, account string, fn func(q registry.Queries) error) error {
	return tx.Run(ctx, s.opts.Database, account, func(t pgx.Tx) error {
		// The statements are built here, from the transaction that set the account (ADR-0047).
		return fn(registry.Queries{
			Plans: reorgplans.New(t), Candidates: policycandidates.New(t),
			Runs: jobruns.New(t), Failures: classification.New(t), Audit: auditlog.New(t),
			Rules: manage.New(t), Changes: policychanges.New(t), Senders: senderclasses.New(t), Accounts: accounts.New(t),
		})
	})
}

// listAccountIDs reads every account's identifier, the accounts listing, which belongs to no account
// (ADR-0091).
func (s *Server) listAccountIDs(ctx context.Context) ([]string, error) {
	listed, err := accounts.New(s.opts.Database).Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(listed))
	for _, a := range listed {
		out = append(out, a.AccountID)
	}
	slices.Sort(out)
	return out, nil
}

// BaseRule is one base rule as the base policy's screens show it (docs/UI.md section 8.14).
type BaseRule struct {
	RuleID    string   `json:"rule_id"`
	Class     string   `json:"class"`
	Suffixes  []string `json:"suffixes"`
	Source    string   `json:"source"`
	CreatedAt string   `json:"created_at"`
	CreatedBy string   `json:"created_by"`
}

// BasePolicy is the base policy screen's read, the base rules the search keeps, every account's
// identifier, how many base rules there are, and when the base policy last changed. It holds no count
// of any account's senders or messages (docs/UI.md section 8.14).
type BasePolicy struct {
	Rules    []BaseRule `json:"rules"`
	Total    int        `json:"total"`
	Accounts []string   `json:"accounts"`
	Latest   *string    `json:"latest"`
}

func basePolicyType() schema.Type {
	return schema.Obj("BasePolicy",
		schema.F("rules", schema.ArrayOf(schema.Obj("BaseRule",
			schema.F("rule_id", schema.Str()),
			schema.F("class", schema.Str()),
			schema.F("suffixes", schema.ArrayOf(schema.Str())),
			schema.F("source", schema.Str()),
			schema.F("created_at", schema.Time()),
			schema.F("created_by", schema.Str()),
		))),
		schema.F("total", schema.Int()),
		schema.F("accounts", schema.ArrayOf(schema.Str())),
		schema.F("latest", schema.Null(schema.Time())),
	)
}

// getBasePolicy is GET /api/setup/policy?search=…, read in a base-policy transaction, which names no
// account and reads no account's rows (ADR-0112). The search is a case-insensitive substring of a
// rule's identifier or one of its suffixes.
func (s *Server) getBasePolicy(w http.ResponseWriter, r *http.Request) {
	search := strings.ToLower(r.URL.Query().Get("search"))
	out := BasePolicy{Rules: []BaseRule{}}
	err := tx.RunBase(r.Context(), s.opts.Database, func(t pgx.Tx) error {
		q := base.New(t)
		stored, err := baseStored(r.Context(), q)
		if err != nil {
			return err
		}
		out.Total = len(stored)
		for _, rule := range stored {
			if search != "" && !strings.Contains(strings.ToLower(rule.ID), search) &&
				!slices.ContainsFunc(rule.Suffixes, func(x string) bool { return strings.Contains(strings.ToLower(x), search) }) {
				continue
			}
			out.Rules = append(out.Rules, BaseRule{
				RuleID: rule.ID, Class: rule.Class, Suffixes: nonNil(rule.Suffixes), Source: rule.Source,
				CreatedAt: registry.Stamp(rule.CreatedAt), CreatedBy: rule.CreatedBy,
			})
		}
		latest, err := q.BaseLatestChange(r.Context())
		if err != nil {
			return err
		}
		out.Latest = stampOf(latest)
		return nil
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	if out.Accounts, err = s.listAccountIDs(r.Context()); err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// BaseHistory is the base policy's history screen's read, newest first (docs/UI.md section 8.14).
type BaseHistory struct {
	Range string               `json:"range"`
	Rows  []registry.ChangeRow `json:"rows"`
}

func baseHistoryType() schema.Type {
	return schema.Obj("BaseHistory", schema.F("range", schema.Str()), schema.F("rows", schema.ArrayOf(registry.ChangeRowType())))
}

// getBaseHistory is GET /api/setup/policy/history?range=…&rule=…, the base policy's changes alone, read
// in a base-policy transaction. The range is the dataset endpoint's grammar, the last 30 days by
// default, and rule narrows the rows to one rule's.
func (s *Server) getBaseHistory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	for name, values := range query {
		if name != "range" && name != "rule" || len(values) != 1 {
			writeFailure(w, r, clientFault(http.StatusBadRequest, "unknown_parameter", "the base policy's history takes range and rule, each once"))
			return
		}
	}
	rangeText := query.Get("range")
	if rangeText == "" {
		rangeText = "30d"
	}
	rng, err := lens.ParseRange(rangeText)
	var refusal *lens.Refusal
	if errors.As(err, &refusal) {
		writeFailure(w, r, clientFault(http.StatusBadRequest, refusal.Code, refusal.Message))
		return
	}
	if err != nil {
		s.uiFailure(w, r, err)
		return
	}
	b := bounds(rng, s.opts.Clock())
	out := BaseHistory{Range: rng.String(), Rows: []registry.ChangeRow{}}
	err = tx.RunBase(r.Context(), s.opts.Database, func(t pgx.Tx) error {
		q := base.New(t)
		stored, err := baseStored(r.Context(), q)
		if err != nil {
			return err
		}
		rows, err := q.BaseHistory(r.Context(), base.BaseHistoryParams{RangeStart: timestamptz(b.Start), RangeEnd: timestamptz(b.End), RuleID: query.Get("rule")})
		if err != nil {
			return err
		}
		for _, h := range rows {
			exists := slices.ContainsFunc(stored, func(x registry.StoredRule) bool { return x.ID == h.RuleID })
			out.Rows = append(out.Rows, registry.BaseChangeRow(h.ID, h.Ts, h.Actor, h.Action, h.RuleID, h.SuffixesBefore, h.SuffixesAfter, exists))
		}
		return nil
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// timestamptz is an optional instant as the generated statements take it.
func timestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// baseRuleCount reads how many base rules there are, in a base-policy transaction, for the
// installation endpoint (docs/UI.md section 8.10).
func (s *Server) baseRuleCount(ctx context.Context) (int, error) {
	n := 0
	err := tx.RunBase(ctx, s.opts.Database, func(t pgx.Tx) error {
		stored, err := baseStored(ctx, base.New(t))
		n = len(stored)
		return err
	})
	return n, err
}
