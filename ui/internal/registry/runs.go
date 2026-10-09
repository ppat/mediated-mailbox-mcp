package registry

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/ppat/mediated-mailbox-mcp/db/jobruns"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// The job vocabularies as stored (ADR-0016).
func workloads() []string { return []string{"backfill", "sync", "apply", "heuristics"} }

func passes() []string {
	return []string{"pass1", "pass2", "tick", "gap_recovery", "apply", "rollback"}
}

func runStates() []string { return []string{"running", "succeeded", "failed"} }

// RecordedRuns returns the workload and pass pairs the built job kinds record, in step, the closed set
// the schema's check on job_runs holds (ADR-0016). The jobs cards read the latest run of each. A job
// kind's pairs join it in the release whose migration adds them to the check.
func RecordedRuns() (workloads, passes []string) {
	return []string{"backfill", "backfill", "sync", "sync"}, []string{"pass1", "pass2", "tick", "gap_recovery"}
}

// Run is a job run's whole recorded state, the same shape on the jobs endpoint, in the stream's run
// event and in the runs dataset's rows (docs/UI.md sections 8.3 and 17.5). The checkpoint and the
// counters are the workload's own JSON (ADR-0016), which the cards read their fields from. last_error
// is provider or scanner text, never a body.
type Run struct {
	RunID           string          `json:"run_id"`
	Workload        string          `json:"workload"`
	Pass            *string         `json:"pass"`
	State           string          `json:"state"`
	PlanID          *string         `json:"plan_id"`
	PlanDescription *string         `json:"plan_description"`
	PlanStatus      *string         `json:"plan_status"`
	ResumedFrom     *string         `json:"resumed_from"`
	StartedAt       string          `json:"started_at"`
	FinishedAt      *string         `json:"finished_at"`
	HeartbeatAt     *string         `json:"heartbeat_at"`
	Checkpoint      json.RawMessage `json:"checkpoint"`
	Counters        json.RawMessage `json:"counters"`
	LastError       *string         `json:"last_error"`
}

// RunType is Run's declaration.
func RunType() schema.Type {
	return schema.Obj("Run", runFields()...)
}

func runFields() []schema.Field {
	return []schema.Field{
		schema.F("run_id", schema.Str()),
		schema.F("workload", schema.Str(workloads()...)),
		schema.F("pass", schema.Null(schema.Str(passes()...))),
		schema.F("state", schema.Str(runStates()...)),
		schema.F("plan_id", schema.Null(schema.Str())),
		schema.F("plan_description", schema.Null(schema.Str())),
		schema.F("plan_status", schema.Null(schema.Str())),
		schema.F("resumed_from", schema.Null(schema.Str())),
		schema.F("started_at", schema.Time()),
		schema.F("finished_at", schema.Null(schema.Time())),
		schema.F("heartbeat_at", schema.Null(schema.Time())),
		schema.F("checkpoint", schema.Any()),
		schema.F("counters", schema.Any()),
		schema.F("last_error", schema.Null(schema.Str())),
	}
}

// RunRow is one row of the runs dataset, a run and its count of item failures (docs/UI.md section
// 8.3).
type RunRow struct {
	Run
	Failures int64 `json:"failures"`
}

func runRowType() schema.Type {
	return schema.Obj("RunRow", append(runFields(), schema.F("failures", schema.Int()))...)
}

// runs is the job runs, which the jobs screen's table lists (docs/UI.md section 8.3). A run's detail is
// the Run screen, so the dataset declares no provenance query (docs/UI.md section 5).
func runs() Dataset {
	return Dataset{
		Descriptor: lens.Descriptor{
			Name:        "runs",
			Ranged:      true,
			RangeColumn: "started_at",
			Dimensions: []lens.Dimension{
				{Name: "workload", Storage: "text", Groupable: true, Filterable: true, Wording: "workload", Values: workloads()},
				{Name: "state", Storage: "text", Groupable: true, Filterable: true, Wording: "state", Values: runStates()},
				{Name: "day", Storage: "date", Groupable: true, Filterable: true, Wording: "day"},
				{Name: "pass", Storage: "text", Filterable: true, Wording: "pass", Values: passes()},
				{Name: "started_at", Storage: "time", Sortable: true, Wording: "started"},
				{Name: "duration", Storage: "number", Sortable: true, Wording: "duration"},
				{Name: "failures", Storage: "number", Sortable: true, Wording: "failures"},
			},
			Default: lens.Defaults{
				Level: lens.Rows, Range: "7d", Sort: lens.Sort{Column: "started_at", Descending: true},
				Filters: map[string][]string{"pass": {"!tick"}},
			},
		},
		Row:     runRowType(),
		Summary: runSummary,
		Rows:    runRows,
		Aggregates: map[string]Aggregate{
			"workload": runsByWorkload,
			"state":    runsByState,
			"day":      runsByDay,
		},
	}
}

// runFilters are a read's range and filters as the runs statements take them.
type runFilters struct {
	rangeStart, rangeEnd                                        pgtype.Timestamptz
	workloadIn, workloadOut, stateIn, stateOut, passIn, passOut []string
	dayIn, dayOut                                               []pgtype.Date
}

func runFiltersOf(r Read) (runFilters, error) {
	f := runFilters{rangeStart: timestamptz(r.Bounds.Start), rangeEnd: timestamptz(r.Bounds.End)}
	f.workloadIn, f.workloadOut = split(r.Request, "workload")
	f.stateIn, f.stateOut = split(r.Request, "state")
	f.passIn, f.passOut = split(r.Request, "pass")
	dayIn, dayOut := split(r.Request, "day")
	var err error
	if f.dayIn, err = dates(dayIn); err != nil {
		return runFilters{}, err
	}
	if f.dayOut, err = dates(dayOut); err != nil {
		return runFilters{}, err
	}
	return f, nil
}

func runSummary(ctx context.Context, q Queries, r Read) ([]Figure, Total, error) {
	f, err := runFiltersOf(r)
	if err != nil {
		return nil, Total{}, err
	}
	row, err := q.Runs.RunFigures(ctx, jobruns.RunFiguresParams{
		AccountID: r.Account, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		WorkloadIn: f.workloadIn, WorkloadOut: f.workloadOut, StateIn: f.stateIn, StateOut: f.stateOut,
		PassIn: f.passIn, PassOut: f.passOut, DayIn: f.dayIn, DayOut: f.dayOut,
	})
	if err != nil {
		return nil, Total{}, err
	}
	last := Figure{Key: "last_failure", Wording: "last failure", At: optionalStamp(row.LastFailureAt), Link: link(r.Account, "jobs", url.Values{"state": {"failed"}})}
	if row.LastFailureRun != "" {
		last.Link = link(r.Account, "jobs/"+url.PathEscape(row.LastFailureRun), nil)
	}
	figures := []Figure{
		count("runs", "runs", row.Runs, link(r.Account, "jobs", nil)),
		count("running", "running", row.Running, link(r.Account, "jobs", url.Values{"state": {"running"}})),
		count("failed", "failed", row.Failed, link(r.Account, "jobs", url.Values{"state": {"failed"}})),
		last,
	}
	return figures, Total{Count: row.Runs}, nil
}

func runRows(ctx context.Context, q Queries, r Read) (any, error) {
	out := []RunRow{}
	first, ok := offset(r.Request)
	if !ok {
		return out, nil
	}
	f, err := runFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Runs.RunRows(ctx, jobruns.RunRowsParams{
		AccountID: r.Account, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		WorkloadIn: f.workloadIn, WorkloadOut: f.workloadOut, StateIn: f.stateIn, StateOut: f.stateOut,
		PassIn: f.passIn, PassOut: f.passOut, DayIn: f.dayIn, DayOut: f.dayOut,
		SortColumn: r.Request.Sort.Column, Descending: r.Request.Sort.Descending,
		AsOf: pgtype.Timestamptz{Time: r.AsOf, Valid: true}, RowOffset: first,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, RunRow{Run: Run{
			RunID: row.RunID, Workload: row.Workload, Pass: &row.Pass, State: row.State,
			PlanID: uuid(row.PlanID), PlanDescription: text(row.PlanDescription.Valid, row.PlanDescription.String),
			PlanStatus: text(row.PlanStatus.Valid, row.PlanStatus.String), ResumedFrom: text(row.ResumedFrom.Valid, row.ResumedFrom.String),
			StartedAt: stamp(row.StartedAt), FinishedAt: optionalStamp(row.FinishedAt), HeartbeatAt: optionalStamp(row.HeartbeatAt),
			Checkpoint: RawJSON(row.Checkpoint), Counters: RawJSON(row.Counters), LastError: text(row.LastError.Valid, row.LastError.String),
		}, Failures: row.Failures})
	}
	return out, nil
}

func runsByWorkload(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := runFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Runs.RunsByWorkload(ctx, jobruns.RunsByWorkloadParams{
		AccountID: r.Account, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		WorkloadIn: f.workloadIn, WorkloadOut: f.workloadOut, StateIn: f.stateIn, StateOut: f.stateOut,
		PassIn: f.passIn, PassOut: f.passOut, DayIn: f.dayIn, DayOut: f.dayOut,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"workload": row.Workload}, Count: row.Runs})
	}
	return out, nil
}

func runsByState(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := runFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Runs.RunsByState(ctx, jobruns.RunsByStateParams{
		AccountID: r.Account, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		WorkloadIn: f.workloadIn, WorkloadOut: f.workloadOut, StateIn: f.stateIn, StateOut: f.stateOut,
		PassIn: f.passIn, PassOut: f.passOut, DayIn: f.dayIn, DayOut: f.dayOut,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"state": row.State}, Count: row.Runs})
	}
	return out, nil
}

func runsByDay(ctx context.Context, q Queries, r Read) (any, error) {
	f, err := runFiltersOf(r)
	if err != nil {
		return nil, err
	}
	rows, err := q.Runs.RunsByDay(ctx, jobruns.RunsByDayParams{
		AccountID: r.Account, RangeStart: f.rangeStart, RangeEnd: f.rangeEnd,
		WorkloadIn: f.workloadIn, WorkloadOut: f.workloadOut, StateIn: f.stateIn, StateOut: f.stateOut,
		PassIn: f.passIn, PassOut: f.passOut, DayIn: f.dayIn, DayOut: f.dayOut,
	})
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, Group{Key: map[string]any{"day": row.Day.Time.Format(dateLayout)}, Count: row.Runs})
	}
	return out, nil
}
