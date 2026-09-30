package api

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// runSummary is the run summary endpoint's answer, the run screen's L0 strip and timeline (docs/UI.md
// sections 8.4 and 17.4). Resumer is the latest run whose resumed_from names this one. Failures counts
// the run's item failures in all and per disposition as stored, RecoveredBy the runs that recovered its
// items, and Events the timeline in the order recorded.
type runSummary struct {
	Account     string          `json:"account"`
	AsOf        string          `json:"as_of"`
	Run         run             `json:"run"`
	Resumer     *run            `json:"resumer"`
	Failures    failureCounts   `json:"failures"`
	RecoveredBy []recoveringRun `json:"recovered_by"`
	Events      []timelineEvent `json:"events"`
}

type failureCounts struct {
	Count        int64            `json:"count"`
	Dispositions map[string]int64 `json:"dispositions"`
}

type recoveringRun struct {
	RunID     string `json:"run_id"`
	Recovered int64  `json:"recovered"`
}

// timelineEvent is one event of a run's timeline. detail is provider or scanner text, never a body.
type timelineEvent struct {
	Kind   string  `json:"kind"`
	At     string  `json:"at"`
	Page   *int32  `json:"page"`
	Detail *string `json:"detail"`
}

func runSummaryType() schema.Type {
	return schema.Obj("RunSummary",
		schema.F("account", schema.Str()),
		schema.F("as_of", schema.Time()),
		schema.F("run", runType()),
		schema.F("resumer", schema.Null(runType())),
		schema.F("failures", schema.Obj("RunFailureCounts",
			schema.F("count", schema.Int()),
			schema.F("dispositions", schema.MapOf(schema.Int())),
		)),
		schema.F("recovered_by", schema.ArrayOf(schema.Obj("RecoveringRun",
			schema.F("run_id", schema.Str()),
			schema.F("recovered", schema.Int()),
		))),
		schema.F("events", schema.ArrayOf(schema.Obj("TimelineEvent",
			schema.F("kind", schema.Str()),
			schema.F("at", schema.Time()),
			schema.F("page", schema.Null(schema.Int())),
			schema.F("detail", schema.Null(schema.Str())),
		))),
	)
}

// errUnknownRun is a run the account does not hold.
var errUnknownRun = errors.New("the account holds no such run")

// getRun answers the run summary endpoint in one read transaction for the account.
func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	account, id := r.PathValue("account"), r.PathValue("run")
	var out runSummary
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		q := jobruns.New(t)
		// as_of is the time the read transaction began.
		out = runSummary{Account: account, AsOf: registry.Stamp(s.opts.Clock()), RecoveredBy: []recoveringRun{}, Events: []timelineEvent{}}
		row, err := q.RunByID(r.Context(), jobruns.RunByIDParams{AccountID: account, RunID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return errUnknownRun
		}
		if err != nil {
			return err
		}
		out.Run = *latestRun(jobruns.LatestRunsRow(row))
		resumer, err := q.LatestResumer(r.Context(), jobruns.LatestResumerParams{AccountID: account, RunID: pgtype.Text{String: id, Valid: true}})
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		default:
			out.Resumer = latestRun(jobruns.LatestRunsRow(resumer))
		}
		counts, err := q.FailureDispositions(r.Context(), jobruns.FailureDispositionsParams{AccountID: account, RunID: id})
		if err != nil {
			return err
		}
		out.Failures.Dispositions = map[string]int64{}
		for _, c := range counts {
			out.Failures.Dispositions[c.Disposition] = c.Failures
			out.Failures.Count += c.Failures
		}
		recovering, err := q.RecoveringRuns(r.Context(), jobruns.RecoveringRunsParams{AccountID: account, RunID: id})
		if err != nil {
			return err
		}
		for _, c := range recovering {
			out.RecoveredBy = append(out.RecoveredBy, recoveringRun{RunID: c.RecoveredBy.String, Recovered: c.Recovered})
		}
		events, err := q.RunEvents(r.Context(), jobruns.RunEventsParams{AccountID: account, RunID: id})
		if err != nil {
			return err
		}
		for _, e := range events {
			ev := timelineEvent{Kind: e.Kind, At: registry.Stamp(e.At.Time), Detail: optionalText(e.Detail)}
			if e.Page.Valid {
				p := e.Page.Int32
				ev.Page = &p
			}
			out.Events = append(out.Events, ev)
		}
		return nil
	})
	if errors.Is(err, errUnknownRun) {
		writeFailure(w, r, clientFault(http.StatusNotFound, "unknown_run", "the account holds no such run"))
		return
	}
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}
