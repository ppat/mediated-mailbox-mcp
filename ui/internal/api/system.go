package api

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/auditlog"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/maskingevents"
	"github.com/ppat/mediated-mailbox-mcp/db/messages"
	"github.com/ppat/mediated-mailbox-mcp/db/messages/classification"
	"github.com/ppat/mediated-mailbox-mcp/db/policycandidates"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgplans"
	"github.com/ppat/mediated-mailbox-mcp/db/scangatedecisions"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// auditActions are the audit actions as stored (ADR-0016), which the corpus block counts, each
// present with zero when no row has it.
func auditActions() []string { return []string{"READ_BODY", "DENY_BODY", "MUTATE", "DENY_MUTATE"} }

// systemResponse is the system endpoint's three blocks (docs/UI.md sections 8.1, 8.8 and 17.4).
type systemResponse struct {
	Account     string           `json:"account"`
	Provider    string           `json:"provider"`
	AsOf        string           `json:"as_of"`
	Operational operationalBlock `json:"operational"`
	Corpus      corpusBlock      `json:"corpus"`
	Decisions   decisionsBlock   `json:"decisions"`
}

// operationalBlock carries the values ADR-0034 exposes to clients, one to one, and beside them each
// pass's latest run for its progress, the rate state, and the finish of pass 1's latest succeeded run.
// Connected is false for an account with no state row, whose flags then read false and whose times
// read null (ADR-0091). BackfillPass1SucceededAt tells the partial-index banner a pass 1 a change of
// scanner re-opened from a first one (docs/UI.md section 12, ADR-0096).
type operationalBlock struct {
	Connected                bool       `json:"connected"`
	BackfillPass1Complete    bool       `json:"backfill_pass1_complete"`
	BackfillPass1Run         *run       `json:"backfill_pass1_run"`
	BackfillPass1SucceededAt *string    `json:"backfill_pass1_succeeded_at"`
	BackfillPass2Complete    bool       `json:"backfill_pass2_complete"`
	BackfillPass2Run         *run       `json:"backfill_pass2_run"`
	PendingScan              int64      `json:"pending_scan"`
	SyncCursorAt             *string    `json:"sync_cursor_at"`
	LastSuccessfulTickAt     *string    `json:"last_successful_tick_at"`
	Rate                     *rateBlock `json:"rate"`
	LastAuthAt               *string    `json:"last_auth_at"`
	LastAuthOutcome          *string    `json:"last_auth_outcome"`
}

// corpusBlock is Home's at-a-glance figures. Audit24h counts the audit rows of the last 24 hours by
// action as stored.
type corpusBlock struct {
	Messages        int64            `json:"messages"`
	Threads         int64            `json:"threads"`
	Unfiled         int64            `json:"unfiled"`
	Restricted      int64            `json:"restricted"`
	Audit24h        map[string]int64 `json:"audit_24h"`
	MaskingEvents7d int64            `json:"masking_events_7d"`
	GateSkips7d     int64            `json:"gate_skips_7d"`
}

// decisionsBlock is what the chrome's counters read.
type decisionsBlock struct {
	PlansDraft        int64 `json:"plans_draft"`
	CandidatesPending int64 `json:"candidates_pending"`
	WorkloadsRunning  int64 `json:"workloads_running"`
}

func systemType() schema.Type {
	nullRun := schema.Null(runType())
	nullTime := schema.Null(schema.Time())
	return schema.Obj("System",
		schema.F("account", schema.Str()),
		schema.F("provider", schema.Str()),
		schema.F("as_of", schema.Time()),
		schema.F("operational", schema.Obj("Operational",
			schema.F("connected", schema.Bool()),
			schema.F("backfill_pass1_complete", schema.Bool()),
			schema.F("backfill_pass1_run", nullRun),
			schema.F("backfill_pass1_succeeded_at", nullTime),
			schema.F("backfill_pass2_complete", schema.Bool()),
			schema.F("backfill_pass2_run", nullRun),
			schema.F("pending_scan", schema.Int()),
			schema.F("sync_cursor_at", nullTime),
			schema.F("last_successful_tick_at", nullTime),
			schema.F("rate", schema.Null(rateType())),
			schema.F("last_auth_at", nullTime),
			schema.F("last_auth_outcome", schema.Null(schema.Str())),
		)),
		schema.F("corpus", schema.Obj("Corpus",
			schema.F("messages", schema.Int()),
			schema.F("threads", schema.Int()),
			schema.F("unfiled", schema.Int()),
			schema.F("restricted", schema.Int()),
			schema.F("audit_24h", schema.MapOf(schema.Int())),
			schema.F("masking_events_7d", schema.Int()),
			schema.F("gate_skips_7d", schema.Int()),
		)),
		schema.F("decisions", schema.Obj("Decisions",
			schema.F("plans_draft", schema.Int()),
			schema.F("candidates_pending", schema.Int()),
			schema.F("workloads_running", schema.Int()),
		)),
	)
}

// getSystem answers the system endpoint in one read transaction for the account.
func (s *Server) getSystem(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	listed, err := accounts.New(s.opts.Database).Accounts(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	out := systemResponse{Account: account}
	for _, a := range listed {
		if a.AccountID == account {
			out.Provider = a.AccountProvider
		}
	}
	err = tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		// as_of is the time the read transaction began.
		now := s.opts.Clock()
		out.AsOf = registry.Stamp(now)
		return s.system(r.Context(), systemQueries{
			state: accountstate.New(t), rate: ratestate.New(t), runs: jobruns.New(t), messages: messages.New(t),
			classification: classification.New(t), audit: auditlog.New(t), masking: maskingevents.New(t),
			gate: scangatedecisions.New(t), plans: reorgplans.New(t), candidates: policycandidates.New(t),
		}, account, now, &out)
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// systemQueries are the statements the system endpoint runs, built inside the literal passed the
// transaction helper (ADR-0047).
type systemQueries struct {
	state          *accountstate.Queries
	rate           *ratestate.Queries
	runs           *jobruns.Queries
	messages       *messages.Queries
	classification *classification.Queries
	audit          *auditlog.Queries
	masking        *maskingevents.Queries
	gate           *scangatedecisions.Queries
	plans          *reorgplans.Queries
	candidates     *policycandidates.Queries
}

func (s *Server) system(ctx context.Context, q systemQueries, account string, now time.Time, out *systemResponse) error {
	day := pgtype.Timestamptz{Time: now.Add(-24 * time.Hour), Valid: true}
	week := pgtype.Timestamptz{Time: now.Add(-7 * 24 * time.Hour), Valid: true}

	progress, err := s.progress(ctx, q.state, account)
	if err != nil {
		return err
	}
	op := &out.Operational
	op.Connected = progress.Connected
	op.BackfillPass1Complete = progress.Pass1Complete
	op.BackfillPass2Complete = progress.Pass2Complete
	op.SyncCursorAt = progress.SyncCursorAt
	op.LastAuthAt = progress.LastAuthAt
	op.LastAuthOutcome = progress.LastAuthOutcome
	if op.Rate, err = s.rate(ctx, q.rate, account); err != nil {
		return err
	}

	runs := q.runs
	latest, err := runs.LatestRuns(ctx, account)
	if err != nil {
		return err
	}
	running := map[string]bool{}
	for _, row := range latest {
		if row.State == "running" {
			running[row.Workload] = true
		}
		if row.Workload == "backfill" && row.Pass.Valid && row.Pass.String == "pass1" {
			op.BackfillPass1Run = latestRun(row)
		}
		if row.Workload == "backfill" && row.Pass.Valid && row.Pass.String == "pass2" {
			op.BackfillPass2Run = latestRun(row)
		}
	}
	out.Decisions.WorkloadsRunning = int64(len(running))
	tick, err := finished(ctx, runs, account, "sync", "tick", "succeeded")
	if err != nil {
		return err
	}
	if tick != nil {
		op.LastSuccessfulTickAt = tick.FinishedAt
	}
	pass1, err := finished(ctx, runs, account, "backfill", "pass1", "succeeded")
	if err != nil {
		return err
	}
	if pass1 != nil {
		op.BackfillPass1SucceededAt = pass1.FinishedAt
	}

	if op.PendingScan, err = q.messages.ScanBacklog(ctx, account); err != nil {
		return err
	}
	corpus, err := q.classification.CorpusFigures(ctx, account)
	if err != nil {
		return err
	}
	c := &out.Corpus
	c.Messages, c.Threads, c.Unfiled, c.Restricted = corpus.Messages, corpus.Threads, corpus.Unfiled, corpus.Restricted

	actions, err := q.audit.AuditActionCounts(ctx, auditlog.AuditActionCountsParams{AccountID: account, Since: day})
	if err != nil {
		return err
	}
	c.Audit24h = map[string]int64{}
	for _, a := range auditActions() {
		c.Audit24h[a] = 0
	}
	for _, row := range actions {
		c.Audit24h[row.Action] = row.Entries
	}
	if c.MaskingEvents7d, err = q.masking.MaskingEventCount(ctx, maskingevents.MaskingEventCountParams{AccountID: account, Since: week}); err != nil {
		return err
	}
	if c.GateSkips7d, err = q.gate.GateDecisionCount(ctx, scangatedecisions.GateDecisionCountParams{
		AccountID: account, Decision: "SKIP", Since: week,
	}); err != nil {
		return err
	}

	plans, err := q.plans.PlanStatusCounts(ctx, reorgplans.PlanStatusCountsParams{AccountID: account, StatusIn: []string{"DRAFT"}})
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(plans, func(p reorgplans.PlanStatusCountsRow) bool { return p.Status == "DRAFT" }); i >= 0 {
		out.Decisions.PlansDraft = plans[i].Plans
	}
	candidates, err := q.candidates.CandidateStatusCounts(ctx, policycandidates.CandidateStatusCountsParams{AccountID: account, StatusIn: []string{"pending"}})
	if err != nil {
		return err
	}
	if i := slices.IndexFunc(candidates, func(p policycandidates.CandidateStatusCountsRow) bool { return p.Status == "pending" }); i >= 0 {
		out.Decisions.CandidatesPending = candidates[i].Candidates
	}
	return nil
}
