-- name: LatestRuns :many
-- The latest run of each workload and pass the caller names, which the jobs cards read (docs/UI.md
-- section 8.3). The pairs are the closed set the schema's check on job_runs holds, given as two arrays
-- of one length read in step. Each pair is one lookup on the run index, newest first, and a pair with
-- no run has no row. An apply or rollback run carries its plan's description and status, so the card
-- can title the run and say whether rollback is available.
SELECT
    l.run_id,
    l.workload,
    l.pass,
    l.state,
    l.plan_id,
    l.plan_description,
    l.plan_status,
    l.resumed_from,
    l.started_at,
    l.finished_at,
    l.heartbeat_at,
    l.checkpoint,
    l.counters,
    l.last_error
FROM (
    SELECT
        unnest(@workloads::text[]) AS workload,
        unnest(@passes::text[]) AS pass
) AS k
CROSS JOIN
    LATERAL (
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
        WHERE r.account_id = @account_id AND r.workload = k.workload AND r.pass = k.pass
        ORDER BY r.started_at DESC, r.run_id ASC
        LIMIT 1
    ) AS l
ORDER BY l.workload ASC, l.pass ASC;

-- name: LatestRunInStates :one
-- The latest run of one workload and pass whose state is one of the given states. The jobs cards read
-- the last finished run beside a running one, and the system screen the last successful sync tick
-- (docs/UI.md sections 8.3 and 8.8).
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
WHERE
    r.account_id = @account_id
    AND r.workload = @workload
    AND r.pass = @pass
    AND r.state = any(@states::text[])
ORDER BY r.started_at DESC, r.run_id ASC
LIMIT 1;

-- name: RunsStartedSince :one
-- How many runs of one workload and pass started since a time, the gap recoveries of the last seven
-- days on the delta sync card (docs/UI.md section 8.3).
SELECT count(*) AS runs
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND r.workload = @workload
    AND r.pass = @pass
    AND r.started_at >= @since;

-- name: ProgressSince :one
-- The first and last checkpoint page a run's progress events recorded since a time, from which the
-- backfill card estimates the time left (docs/UI.md section 8.1). With no progress event in the window,
-- events is zero and both pages read zero.
SELECT
    count(e.page) AS events,
    coalesce(min(e.page), 0)::integer AS first_page,
    coalesce(max(e.page), 0)::integer AS last_page
FROM job_run_events AS e
WHERE
    e.account_id = @account_id
    AND e.run_id = @run_id
    AND e.kind = 'progress'
    AND e.at >= @since;

-- name: StreamRuns :many
-- The runs the event stream follows, every run still running and every run that finished since a time
-- (ADR-0058), each with its whole recorded state, the plan an apply or rollback run carries included,
-- the same shape the jobs endpoint sends. The stream sends a run whenever it differs from the last
-- state sent.
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
WHERE
    r.account_id = @account_id
    AND (r.state = 'running' OR r.finished_at >= @since)
ORDER BY r.started_at ASC, r.run_id ASC;

-- name: GapRecoveriesSince :many
-- The delta sync gap recoveries that succeeded and started since a time, oldest first, for the sync-gap
-- rule of Home's worth-a-look cards (docs/UI.md section 8.1). Each carries its counters, which record
-- the window it re-enumerated and the messages it reconciled (ADR-0016).
SELECT
    r.run_id,
    r.started_at,
    r.finished_at,
    r.counters
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND r.workload = 'sync'
    AND r.pass = 'gap_recovery'
    AND r.state = 'succeeded'
    AND r.started_at >= @since
ORDER BY r.started_at, r.run_id;
