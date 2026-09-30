-- name: RunByID :one
-- One run with its plan's description and status, the run summary endpoint's run (docs/UI.md section
-- 17.4), in the shape the jobs endpoint sends a run.
SELECT
    r.run_id,
    r.workload,
    r.pass,
    r.state,
    r.plan_id,
    pl.description AS plan_description,
    pl.status AS plan_status,
    r.resumed_from,
    r.started_at,
    r.finished_at,
    r.heartbeat_at,
    r.checkpoint,
    r.counters,
    r.last_error
FROM job_runs AS r
LEFT JOIN reorg_plans AS pl ON r.account_id = pl.account_id AND r.plan_id = pl.plan_id
WHERE r.account_id = @account_id AND r.run_id = @run_id;

-- name: LatestResumer :one
-- The latest run whose resumed_from names a run, the run summary's resumer (docs/UI.md section 8.4).
SELECT
    r.run_id,
    r.workload,
    r.pass,
    r.state,
    r.plan_id,
    pl.description AS plan_description,
    pl.status AS plan_status,
    r.resumed_from,
    r.started_at,
    r.finished_at,
    r.heartbeat_at,
    r.checkpoint,
    r.counters,
    r.last_error
FROM job_runs AS r
LEFT JOIN reorg_plans AS pl ON r.account_id = pl.account_id AND r.plan_id = pl.plan_id
WHERE r.account_id = @account_id AND r.resumed_from = @run_id
ORDER BY r.started_at DESC, r.run_id ASC
LIMIT 1;

-- name: FailureDispositions :many
-- A run's item failures counted per disposition, for the run summary's L0 (docs/UI.md section 8.4). A
-- disposition with no failure has no row.
SELECT
    f.disposition,
    count(*) AS failures
FROM job_run_failures AS f
WHERE f.account_id = @account_id AND f.run_id = @run_id
GROUP BY f.disposition
ORDER BY f.disposition;

-- name: RecoveringRuns :many
-- The runs that recovered a run's items, each with how many it recovered, for the run summary's L0
-- (docs/UI.md section 8.4).
SELECT
    f.recovered_by,
    count(*) AS recovered
FROM job_run_failures AS f
WHERE f.account_id = @account_id AND f.run_id = @run_id AND f.recovered_by IS NOT NULL
GROUP BY f.recovered_by
ORDER BY f.recovered_by;

-- name: RunEvents :many
-- Every event of a run's timeline in the order recorded, which the run timeline draws (docs/UI.md
-- sections 7.3 and 8.4).
SELECT
    e.kind,
    e.at,
    e.page,
    e.detail
FROM job_run_events AS e
WHERE e.account_id = @account_id AND e.run_id = @run_id
ORDER BY e.at ASC, e.seq ASC;
