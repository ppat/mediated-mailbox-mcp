package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/reorgoplog"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// The derived workload states (docs/UI.md section 11). A workload with no run is not started, one
// with a run in running is running, and one whose runs have all finished is idle.
const (
	workloadNotStarted = "not_started"
	workloadRunning    = "running"
	workloadIdle       = "idle"
)

// etaWindow is how much of a run's progress history the estimated time left reads, and how much
// history a run needs before it has one (docs/UI.md section 8.1).
const etaWindow = 10 * time.Minute

// jobsResponse is one block per workload, the rate block and the cadences (docs/UI.md sections 8.3 and
// 17.4). The card fields the design lists are read from each run's checkpoint and counters.
type jobsResponse struct {
	Account    string          `json:"account"`
	AsOf       string          `json:"as_of"`
	Backfill   backfillBlock   `json:"backfill"`
	Sync       syncBlock       `json:"sync"`
	Apply      applyBlock      `json:"apply"`
	Heuristics heuristicsBlock `json:"heuristics"`
	Rate       *rateBlock      `json:"rate"`
}

type backfillBlock struct {
	State string    `json:"state"`
	Pass1 passBlock `json:"pass1"`
	Pass2 passBlock `json:"pass2"`
}

// passBlock is one backfill pass. Complete is the account's flag for the pass, and Run the pass's
// latest run. EtaSeconds is set only while the run runs and has ten minutes of progress history.
type passBlock struct {
	Complete   bool   `json:"complete"`
	Run        *run   `json:"run"`
	EtaSeconds *int64 `json:"eta_seconds"`
}

// syncBlock is the delta sync card. CursorAt is when delta sync last wrote the account's cursor, from
// which the card shows the cursor's age.
type syncBlock struct {
	State           string  `json:"state"`
	CadenceSeconds  int64   `json:"cadence_seconds"`
	CursorAt        *string `json:"cursor_at"`
	LastTick        *run    `json:"last_tick"`
	GapRecoveries7d int64   `json:"gap_recoveries_7d"`
	LastGapRecovery *run    `json:"last_gap_recovery"`
}

// applyBlock is the reorg apply card. Running is the apply run in progress, Last the latest finished
// apply run, and RollbackAvailable says Last's plan is APPLIED, whose rollback replays OpLogRows
// operations (ADR-0020).
type applyBlock struct {
	State             string `json:"state"`
	Running           *run   `json:"running"`
	Last              *run   `json:"last"`
	RollbackAvailable bool   `json:"rollback_available"`
	OpLogRows         *int64 `json:"op_log_rows"`
}

type heuristicsBlock struct {
	State          string  `json:"state"`
	CadenceSeconds int64   `json:"cadence_seconds"`
	Last           *run    `json:"last"`
	NextRunAt      *string `json:"next_run_at"`
}

func workloadStateType() schema.Type {
	return schema.Str(workloadNotStarted, workloadRunning, workloadIdle)
}

func jobsType() schema.Type {
	nullRun := schema.Null(runType())
	pass := schema.Obj("BackfillPass",
		schema.F("complete", schema.Bool()),
		schema.F("run", nullRun),
		schema.F("eta_seconds", schema.Null(schema.Int())),
	)
	return schema.Obj("Jobs",
		schema.F("account", schema.Str()),
		schema.F("as_of", schema.Time()),
		schema.F("backfill", schema.Obj("BackfillBlock",
			schema.F("state", workloadStateType()),
			schema.F("pass1", pass),
			schema.F("pass2", pass),
		)),
		schema.F("sync", schema.Obj("SyncBlock",
			schema.F("state", workloadStateType()),
			schema.F("cadence_seconds", schema.Int()),
			schema.F("cursor_at", schema.Null(schema.Time())),
			schema.F("last_tick", nullRun),
			schema.F("gap_recoveries_7d", schema.Int()),
			schema.F("last_gap_recovery", nullRun),
		)),
		schema.F("apply", schema.Obj("ApplyBlock",
			schema.F("state", workloadStateType()),
			schema.F("running", nullRun),
			schema.F("last", nullRun),
			schema.F("rollback_available", schema.Bool()),
			schema.F("op_log_rows", schema.Null(schema.Int())),
		)),
		schema.F("heuristics", schema.Obj("HeuristicsBlock",
			schema.F("state", workloadStateType()),
			schema.F("cadence_seconds", schema.Int()),
			schema.F("last", nullRun),
			schema.F("next_run_at", schema.Null(schema.Time())),
		)),
		schema.F("rate", schema.Null(rateType())),
	)
}

// getJobs answers the jobs endpoint in one read transaction for the account.
func (s *Server) getJobs(w http.ResponseWriter, r *http.Request) {
	account := r.PathValue("account")
	var out jobsResponse
	err := tx.Run(r.Context(), s.opts.Database, account, func(t pgx.Tx) error {
		var err error
		// as_of is the time the read transaction began.
		out, err = s.jobs(r.Context(), jobsQueries{
			runs: jobruns.New(t), oplog: reorgoplog.New(t), state: accountstate.New(t), rate: ratestate.New(t),
		}, account, s.opts.Clock())
		return err
	})
	if err != nil {
		s.databaseFailure(w, r, err)
		return
	}
	writeJSON(w, r, out)
}

// jobsQueries are the statements the jobs endpoint runs, built inside the literal passed the
// transaction helper (ADR-0047).
type jobsQueries struct {
	runs  *jobruns.Queries
	oplog *reorgoplog.Queries
	state *accountstate.Queries
	rate  *ratestate.Queries
}

func (s *Server) jobs(ctx context.Context, jq jobsQueries, account string, now time.Time) (jobsResponse, error) {
	q := jq.runs
	latest, err := latestRuns(ctx, q, account)
	if err != nil {
		return jobsResponse{}, err
	}
	find := func(workload, pass string) *jobruns.LatestRunsRow {
		i := slices.IndexFunc(latest, func(r jobruns.LatestRunsRow) bool {
			return r.Workload == workload && r.Pass == pass
		})
		if i < 0 {
			return nil
		}
		return &latest[i]
	}
	state := func(workload string) string {
		result := workloadNotStarted
		for _, r := range latest {
			if r.Workload != workload {
				continue
			}
			if r.State == "running" {
				return workloadRunning
			}
			result = workloadIdle
		}
		return result
	}
	progress, err := s.progress(ctx, jq.state, account)
	if err != nil {
		return jobsResponse{}, err
	}
	rate, err := s.rate(ctx, jq.rate, account)
	if err != nil {
		return jobsResponse{}, err
	}
	out := jobsResponse{Account: account, AsOf: registry.Stamp(now), Rate: rate}

	out.Backfill.State = state("backfill")
	for _, p := range []struct {
		pass     string
		complete bool
		block    *passBlock
	}{{"pass1", progress.Pass1Complete, &out.Backfill.Pass1}, {"pass2", progress.Pass2Complete, &out.Backfill.Pass2}} {
		p.block.Complete = p.complete
		row := find("backfill", p.pass)
		if row == nil {
			continue
		}
		p.block.Run = latestRun(*row)
		if p.block.EtaSeconds, err = eta(ctx, q, account, *row, now); err != nil {
			return jobsResponse{}, err
		}
	}

	out.Sync = syncBlock{State: state("sync"), CadenceSeconds: int64(s.opts.Cadences.Sync.Seconds()), CursorAt: progress.SyncCursorAt}
	if row := find("sync", "tick"); row != nil {
		out.Sync.LastTick = latestRun(*row)
	}
	if row := find("sync", "gap_recovery"); row != nil {
		out.Sync.LastGapRecovery = latestRun(*row)
	}
	since := pgtype.Timestamptz{Time: now.Add(-7 * 24 * time.Hour), Valid: true}
	if out.Sync.GapRecoveries7d, err = q.RunsStartedSince(ctx, jobruns.RunsStartedSinceParams{
		AccountID: account, Workload: "sync", Pass: "gap_recovery", Since: since,
	}); err != nil {
		return jobsResponse{}, err
	}

	out.Apply.State = state("apply")
	if row := find("apply", "apply"); row != nil && row.State == "running" {
		out.Apply.Running = latestRun(*row)
	}
	last, err := finished(ctx, q, account, "apply", "apply", "succeeded", "failed")
	if err != nil {
		return jobsResponse{}, err
	}
	if last != nil {
		out.Apply.Last = last
		if last.PlanStatus != nil && *last.PlanStatus == "APPLIED" && last.PlanID != nil {
			var plan pgtype.UUID
			if err := plan.Scan(*last.PlanID); err != nil {
				return jobsResponse{}, err
			}
			rows, err := jq.oplog.OpLogCount(ctx, reorgoplog.OpLogCountParams{AccountID: account, PlanID: plan})
			if err != nil {
				return jobsResponse{}, err
			}
			out.Apply.RollbackAvailable = true
			out.Apply.OpLogRows = &rows
		}
	}

	out.Heuristics = heuristicsBlock{State: state("heuristics"), CadenceSeconds: int64(s.opts.Cadences.Heuristics.Seconds())}
	// No heuristics pair is in the closed set the schema's check holds, and every recorded pass is
	// named, so the empty pass finds no run. The job kind that adds its pair names its pass here
	// (ADR-0016).
	if out.Heuristics.Last, err = finished(ctx, q, account, "heuristics", "", "succeeded", "failed"); err != nil {
		return jobsResponse{}, err
	}
	if out.Heuristics.Last != nil {
		started, err := time.Parse(time.RFC3339, out.Heuristics.Last.StartedAt)
		if err != nil {
			return jobsResponse{}, err
		}
		next := registry.Stamp(started.Add(s.opts.Cadences.Heuristics))
		out.Heuristics.NextRunAt = &next
	}
	return out, nil
}

// finished is the latest run of a workload and pass in one of the given states, nil for none.
func finished(ctx context.Context, q *jobruns.Queries, account, workload, pass string, states ...string) (*run, error) {
	row, err := q.LatestRunInStates(ctx, jobruns.LatestRunInStatesParams{
		AccountID: account, Workload: workload, Pass: pass, States: states,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run{
		RunID: row.RunID, Workload: row.Workload, Pass: &row.Pass, State: row.State,
		PlanID: optionalUUID(row.PlanID), PlanDescription: optionalText(row.PlanDescription), PlanStatus: optionalText(row.PlanStatus),
		ResumedFrom: optionalText(row.ResumedFrom), StartedAt: registry.Stamp(row.StartedAt.Time),
		FinishedAt: optionalStamp(row.FinishedAt), HeartbeatAt: optionalStamp(row.HeartbeatAt),
		Checkpoint: rawJSON(row.Checkpoint), Counters: rawJSON(row.Counters), LastError: optionalText(row.LastError),
	}, nil
}

// latestRuns is the latest run of each workload and pass the built job kinds record.
func latestRuns(ctx context.Context, q *jobruns.Queries, account string) ([]jobruns.LatestRunsRow, error) {
	workloads, passes := registry.RecordedRuns()
	return q.LatestRuns(ctx, jobruns.LatestRunsParams{Workloads: workloads, Passes: passes, AccountID: account})
}

func latestRun(row jobruns.LatestRunsRow) *run {
	return &run{
		RunID: row.RunID, Workload: row.Workload, Pass: &row.Pass, State: row.State,
		PlanID: optionalUUID(row.PlanID), PlanDescription: optionalText(row.PlanDescription), PlanStatus: optionalText(row.PlanStatus),
		ResumedFrom: optionalText(row.ResumedFrom), StartedAt: registry.Stamp(row.StartedAt.Time),
		FinishedAt: optionalStamp(row.FinishedAt), HeartbeatAt: optionalStamp(row.HeartbeatAt),
		Checkpoint: rawJSON(row.Checkpoint), Counters: rawJSON(row.Counters), LastError: optionalText(row.LastError),
	}
}

// eta is a running pass's estimated time left, the pages remaining divided by the pages completed in
// the last ten minutes of the run, read from its progress events (docs/UI.md section 8.1). It is nil
// for a run that is not running, has less than ten minutes of history or completed no page in the
// window, or whose checkpoint is not a page of pages.
func eta(ctx context.Context, q *jobruns.Queries, account string, row jobruns.LatestRunsRow, now time.Time) (*int64, error) {
	windowStart := now.Add(-etaWindow)
	if row.State != "running" || row.StartedAt.Time.After(windowStart) {
		return nil, nil
	}
	var checkpoint struct {
		Page *int64 `json:"page"`
		Of   *int64 `json:"of"`
	}
	if json.Unmarshal(row.Checkpoint, &checkpoint) != nil || checkpoint.Page == nil || checkpoint.Of == nil {
		return nil, nil
	}
	p, err := q.ProgressSince(ctx, jobruns.ProgressSinceParams{
		AccountID: account, RunID: row.RunID, Since: pgtype.Timestamptz{Time: windowStart, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	done := int64(p.LastPage - p.FirstPage)
	if p.Events == 0 || done <= 0 {
		return nil, nil
	}
	remaining := max(0, *checkpoint.Of-*checkpoint.Page)
	seconds := remaining * int64(etaWindow.Seconds()) / done
	return &seconds, nil
}
