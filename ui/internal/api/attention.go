package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/maskingevents/senders"
	"github.com/ppat/mediated-mailbox-mcp/db/messages"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// maskingWindow and servesWindow are the masking rule's 7 days and the body-serve rule's 24 hours
// (docs/UI.md section 8.1).
const (
	maskingWindow = 7 * 24 * time.Hour
	servesWindow  = 24 * time.Hour
)

// attentionResponse is Home's worth-a-look cards in their order (docs/UI.md sections 8.1 and 17.4).
type attentionResponse struct {
	Account string          `json:"account"`
	AsOf    string          `json:"as_of"`
	Cards   []attentionCard `json:"cards"`
}

type attentionCard struct {
	Rule     string  `json:"rule"`
	What     string  `json:"what"`
	Number   int64   `json:"number"`
	Since    *string `json:"since"`
	Sentence string  `json:"sentence"`
	Link     string  `json:"link"`
}

func attentionType() schema.Type {
	return schema.Obj("Attention",
		schema.F("account", schema.Str()),
		schema.F("as_of", schema.Time()),
		schema.F("cards", schema.ArrayOf(schema.Obj("AttentionCard",
			schema.F("rule", schema.Str(attention.Rules()...)),
			schema.F("what", schema.Str()),
			schema.F("number", schema.Int()),
			schema.F("since", schema.Null(schema.Time())),
			schema.F("sentence", schema.Str()),
			schema.F("link", schema.Str()),
		))),
	)
}

// getAttention answers the attention endpoint in one read transaction for the account. A rule whose
// threshold is 0 is disabled, and its statements are not run.
func (s *Server) getAttention(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	out := attentionResponse{Account: account, Cards: []attentionCard{}}
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		// as_of is the time the read transaction began.
		now := s.opts.Clock()
		out.AsOf = registry.Stamp(now)
		in, err := s.attentionInputs(r.Context(), attentionQueries{
			messages: messages.New(t), classification: classification.New(t), masking: senders.New(t),
			audit: auditlog.New(t), runs: jobruns.New(t),
		}, account, now)
		if err != nil {
			return err
		}
		for _, c := range attention.Cards(s.opts.Attention, in) {
			card := attentionCard{Rule: c.Rule, What: c.What, Number: c.Number, Sentence: c.Sentence, Link: s.attentionLink(account, c, now)}
			if c.Since != "" {
				since := c.Since
				card.Since = &since
			}
			out.Cards = append(out.Cards, card)
		}
		return nil
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// attentionQueries are the statements the attention endpoint runs, built inside the literal passed the
// transaction helper (ADR-0047).
type attentionQueries struct {
	messages       *messages.Queries
	classification *classification.Queries
	masking        *senders.Queries
	audit          *auditlog.Queries
	runs           *jobruns.Queries
}

// attentionInputs reads the recorded state each enabled rule decides on.
func (s *Server) attentionInputs(ctx context.Context, q attentionQueries, account string, now time.Time) (attention.Inputs, error) {
	th := s.opts.Attention
	var in attention.Inputs
	at := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	if th.BacklogShare > 0 {
		pending, err := q.messages.ScanBacklog(ctx, account)
		if err != nil {
			return in, err
		}
		corpus, err := q.classification.CorpusFigures(ctx, account)
		if err != nil {
			return in, err
		}
		in.Pending, in.Messages = pending, corpus.Messages
	}
	if th.MaskCount > 0 {
		pairs, err := q.masking.MaskingPairsAbove(ctx, senders.MaskingPairsAboveParams{
			AccountID: account, Since: at(now.Add(-maskingWindow)), Above: th.MaskCount,
		})
		if err != nil {
			return in, err
		}
		for _, p := range pairs {
			in.Pairs = append(in.Pairs, attention.Pair{Sender: p.Sender, Rule: p.RuleID, Events: p.Events, FirstAt: registry.Stamp(p.FirstAt.Time)})
		}
	}
	if th.ServeFactor > 0 {
		start := now.Add(-servesWindow)
		served, err := q.audit.BodyServesSince(ctx, auditlog.BodyServesSinceParams{AccountID: account, Since: at(start)})
		if err != nil {
			return in, err
		}
		in.Serves.Count = served.Serves
		if served.FirstAt.Valid {
			in.Serves.FirstAt = registry.Stamp(served.FirstAt.Time)
		}
		// The baseline is the seven whole UTC days before the 24 hours start, the days the audit lens's
		// median reads (docs/UI.md sections 8.1 and 8.5).
		end := start.UTC().Truncate(24 * time.Hour)
		days, err := q.audit.BodyServesByDay(ctx, auditlog.BodyServesByDayParams{
			AccountID: account, RangeStart: at(end.AddDate(0, 0, -attention.BaselineDays)), RangeEnd: at(end),
		})
		if err != nil {
			return in, err
		}
		for _, d := range days {
			in.Serves.DayCounts = append(in.Serves.DayCounts, d.Serves)
		}
	}
	if th.GapDays > 0 {
		runs, err := q.runs.GapRecoveriesSince(ctx, jobruns.GapRecoveriesSinceParams{
			AccountID: account, Since: at(now.Add(-time.Duration(th.GapDays) * 24 * time.Hour)),
		})
		if err != nil {
			return in, err
		}
		for _, row := range runs {
			in.Recoveries = append(in.Recoveries, recovery(row))
		}
	}
	return in, nil
}

// recovery reads a gap recovery's window and reconciled count from its counters (ADR-0016). A key
// the counters lack, or hold as something else, is left unrecorded, and the card's sentence leaves
// its clause out.
func recovery(row jobruns.GapRecoveriesSinceRow) attention.Recovery {
	r := attention.Recovery{StartedAt: registry.Stamp(row.StartedAt.Time)}
	if row.FinishedAt.Valid {
		r.FinishedAt = registry.Stamp(row.FinishedAt.Time)
	}
	var counters struct {
		WindowStart *time.Time `json:"window_start"`
		WindowEnd   *time.Time `json:"window_end"`
		Reconciled  *int64     `json:"reconciled"`
	}
	if json.Unmarshal(row.Counters, &counters) != nil {
		return r
	}
	if counters.WindowStart != nil && counters.WindowEnd != nil {
		seconds := int64(counters.WindowEnd.Sub(*counters.WindowStart).Seconds())
		r.WindowSeconds = &seconds
	}
	r.Reconciled = counters.Reconciled
	return r
}

// attentionLink is the screen that explains a card with its value applied as a filter, as section
// 8.1's table names it. The browser links it only when that screen exists.
func (s *Server) attentionLink(account string, c attention.Card, now time.Time) string {
	base := "/" + url.PathEscape(account) + "/"
	switch c.Rule {
	case attention.Backlog:
		return base + "messages?level=3&scan_state=pending"
	case attention.Masking:
		q := "level=3&range=7d"
		if nameable(c.MaskRule) {
			q += "&rule=" + url.QueryEscape(c.MaskRule)
		}
		if nameable(c.Sender) {
			q += "&sender=" + url.QueryEscape(c.Sender)
		}
		return base + "masking?" + q
	case attention.BodyServes:
		return base + "audit?level=3&range=24h&action=READ_BODY"
	case attention.SyncGap:
		return base + "jobs?range=" + gapRange(s.opts.Attention.GapDays, now) + "&pass=gap_recovery"
	}
	return base
}

// nameable reports whether the filter grammar can name a value as itself, which a value holding a comma,
// starting with an exclusion's mark, spelled as the null or the empty group's word, or empty cannot
// (docs/UI.md section 5).
func nameable(v string) bool {
	return v != "" && v != "none" && v != "empty" && !strings.Contains(v, ",") && !strings.HasPrefix(v, "!")
}

// gapRange is the range covering the sync-gap rule's days, a preset where one matches and otherwise the
// whole UTC days from the first of them to today.
func gapRange(days int64, now time.Time) string {
	switch days {
	case 1:
		return "24h"
	case 7:
		return "7d"
	case 30:
		return "30d"
	case 90:
		return "90d"
	}
	from := now.UTC().AddDate(0, 0, -int(days))
	return from.Format(time.DateOnly) + "," + now.UTC().Format(time.DateOnly)
}
