-- name: LatestRun :one
-- The account's latest run of one workload and pass, whatever its state, which a workload reads to
-- decide whether it resumes that run (ADR-0017, ADR-0117).
SELECT
    r.run_id,
    r.state,
    r.checkpoint,
    r.counters
FROM job_runs AS r
WHERE r.account_id = @account_id AND r.workload = @workload AND r.pass = @pass
ORDER BY r.started_at DESC, r.run_id DESC
LIMIT 1;

-- name: RecordedSuccess :one
-- The account's latest success in one workload as its runs record it, the later of the latest page or
-- step a run made durable, which its progress event marks, and the end of the latest run that
-- succeeded, and the start of its earliest run, each null when no run records one. The worker's late
-- alert counts a job from them at its start, so a restart leaves a job that does not succeed ageing
-- (ADR-0119).
SELECT
    greatest(
        (
            SELECT max(e.at)
            FROM job_run_events AS e
            INNER JOIN job_runs AS r ON e.account_id = r.account_id AND e.run_id = r.run_id
            WHERE r.account_id = @account_id AND r.workload = @workload AND e.kind = 'progress'
        ),
        (
            SELECT max(r.finished_at)
            FROM job_runs AS r
            WHERE r.account_id = @account_id AND r.workload = @workload AND r.state = 'succeeded'
        )
    )::timestamptz AS latest_success,
    (
        SELECT min(r.started_at)
        FROM job_runs AS r
        WHERE r.account_id = @account_id AND r.workload = @workload
    )::timestamptz AS earliest_start;

-- name: StartRun :exec
-- Records a run as it starts, with the checkpoint and counters it starts from and the run it resumes,
-- if any (ADR-0117).
INSERT INTO job_runs (
    account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters
) VALUES (
    @account_id,
    @run_id,
    @workload,
    @pass,
    'running',
    sqlc.narg(resumed_from),
    now(),
    now(),
    @checkpoint,
    @counters
);

-- name: RecordProgress :exec
-- Records a running run's checkpoint and counters, in the transaction that made the work they count
-- durable, and its heartbeat.
UPDATE job_runs
SET checkpoint = @checkpoint, counters = @counters, heartbeat_at = now()
WHERE account_id = @account_id AND run_id = @run_id;

-- name: EndRun :exec
-- Records that a running run ended, succeeded or failed, with its last error. A run that is no longer
-- running is left as it is, so a run a later one found stopped keeps the state that run gave it.
UPDATE job_runs
SET state = @state, finished_at = now(), heartbeat_at = now(), last_error = sqlc.narg(last_error)
WHERE account_id = @account_id AND run_id = @run_id AND state = 'running';

-- name: RecordEvent :exec
-- Adds one event to a run's timeline. A progress event carries the checkpoint page. The detail is
-- provider or scanner text, never a body (ADR-0117).
INSERT INTO job_run_events (account_id, run_id, kind, page, detail)
VALUES (@account_id, @run_id, @kind, sqlc.narg(page), sqlc.narg(detail));

-- name: RecordFailure :exec
-- Records one item a run failed on, with its error class, how many attempts it took, when the first
-- and last were, and what became of it (ADR-0117).
INSERT INTO job_run_failures (
    account_id,
    run_id,
    item_kind,
    item_id,
    page,
    error_class,
    error_summary,
    attempts,
    first_at,
    last_at,
    disposition
) VALUES (
    @account_id,
    @run_id,
    @item_kind,
    @item_id,
    sqlc.narg(page),
    @error_class,
    sqlc.narg(error_summary),
    @attempts,
    @first_at,
    @last_at,
    @disposition
);
