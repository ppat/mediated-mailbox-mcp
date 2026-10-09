-- name: RunFigures :one
-- The runs dataset's figures under its range and filters (docs/UI.md sections 8.3 and 17.1). runs is
-- also the count the row pages count from. The last failure is the latest failed run's finish, or its
-- start while it has none, with its identity for the figure's link, empty with no failed run. A null
-- range bound is no bound, a null array no filter. Every run records a pass (ADR-0016), so the null
-- test in a pass exclusion matches no run, and a day is the UTC date its run started on.
SELECT
    count(*) AS runs,
    count(*) FILTER (WHERE r.state = 'running') AS running,
    count(*) FILTER (WHERE r.state = 'failed') AS failed,
    max(coalesce(r.finished_at, r.started_at)) FILTER (WHERE r.state = 'failed')::timestamptz AS last_failure_at,
    coalesce((
        array_agg(r.run_id ORDER BY coalesce(r.finished_at, r.started_at) DESC, r.run_id ASC)
        FILTER (WHERE r.state = 'failed')
    )[1], '')::text AS last_failure_run
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR r.started_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR r.started_at < @range_end::timestamptz)
    AND (@workload_in::text[] IS NULL OR r.workload = any(@workload_in::text[]))
    AND (@workload_out::text[] IS NULL OR r.workload != all(@workload_out::text[]))
    AND (@state_in::text[] IS NULL OR r.state = any(@state_in::text[]))
    AND (@state_out::text[] IS NULL OR r.state != all(@state_out::text[]))
    AND (@pass_in::text[] IS NULL OR r.pass = any(@pass_in::text[]))
    AND (@pass_out::text[] IS NULL OR r.pass IS NULL OR r.pass != all(@pass_out::text[]))
    AND (@day_in::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date != all(@day_out::date[]));

-- name: RunsByWorkload :many
-- Every group of the runs dataset by workload, under the same filters, ordered by count and then by
-- the group's key, the order the bars draw (docs/UI.md section 17.1).
SELECT
    r.workload,
    count(*) AS runs
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR r.started_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR r.started_at < @range_end::timestamptz)
    AND (@workload_in::text[] IS NULL OR r.workload = any(@workload_in::text[]))
    AND (@workload_out::text[] IS NULL OR r.workload != all(@workload_out::text[]))
    AND (@state_in::text[] IS NULL OR r.state = any(@state_in::text[]))
    AND (@state_out::text[] IS NULL OR r.state != all(@state_out::text[]))
    AND (@pass_in::text[] IS NULL OR r.pass = any(@pass_in::text[]))
    AND (@pass_out::text[] IS NULL OR r.pass IS NULL OR r.pass != all(@pass_out::text[]))
    AND (@day_in::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY r.workload
ORDER BY runs DESC, r.workload ASC;

-- name: RunsByState :many
-- Every group of the runs dataset by run state, as RunsByWorkload.
SELECT
    r.state,
    count(*) AS runs
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR r.started_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR r.started_at < @range_end::timestamptz)
    AND (@workload_in::text[] IS NULL OR r.workload = any(@workload_in::text[]))
    AND (@workload_out::text[] IS NULL OR r.workload != all(@workload_out::text[]))
    AND (@state_in::text[] IS NULL OR r.state = any(@state_in::text[]))
    AND (@state_out::text[] IS NULL OR r.state != all(@state_out::text[]))
    AND (@pass_in::text[] IS NULL OR r.pass = any(@pass_in::text[]))
    AND (@pass_out::text[] IS NULL OR r.pass IS NULL OR r.pass != all(@pass_out::text[]))
    AND (@day_in::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY r.state
ORDER BY runs DESC, r.state ASC;

-- name: RunsByDay :many
-- Every group of the runs dataset by the UTC day each run started on, as RunsByWorkload.
SELECT
    (r.started_at AT TIME ZONE 'UTC')::date AS day,
    count(*) AS runs
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR r.started_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR r.started_at < @range_end::timestamptz)
    AND (@workload_in::text[] IS NULL OR r.workload = any(@workload_in::text[]))
    AND (@workload_out::text[] IS NULL OR r.workload != all(@workload_out::text[]))
    AND (@state_in::text[] IS NULL OR r.state = any(@state_in::text[]))
    AND (@state_out::text[] IS NULL OR r.state != all(@state_out::text[]))
    AND (@pass_in::text[] IS NULL OR r.pass = any(@pass_in::text[]))
    AND (@pass_out::text[] IS NULL OR r.pass IS NULL OR r.pass != all(@pass_out::text[]))
    AND (@day_in::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
GROUP BY day
ORDER BY runs DESC, day ASC;

-- name: RunRows :many
-- One page of the runs dataset, fifty rows, under the same filters (docs/UI.md section 8.3). Each run
-- carries its plan's description and status, as the jobs endpoint sends a run, and its count of item
-- failures. sort_column names one of the three sortable columns, started_at, duration or failures, in
-- either direction, and the run's identity ends the sort. duration runs to the finish, or to as_of
-- while the run has none.
WITH failure_counts AS (
    SELECT
        f.run_id,
        count(*) AS failures
    FROM job_run_failures AS f
    WHERE f.account_id = @account_id
    GROUP BY f.run_id
)

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
    r.last_error,
    coalesce(fc.failures, 0)::bigint AS failures
FROM job_runs AS r
LEFT JOIN reorg_plans AS pl ON r.account_id = pl.account_id AND r.plan_id = pl.plan_id
LEFT JOIN failure_counts AS fc ON r.run_id = fc.run_id
WHERE
    r.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR r.started_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR r.started_at < @range_end::timestamptz)
    AND (@workload_in::text[] IS NULL OR r.workload = any(@workload_in::text[]))
    AND (@workload_out::text[] IS NULL OR r.workload != all(@workload_out::text[]))
    AND (@state_in::text[] IS NULL OR r.state = any(@state_in::text[]))
    AND (@state_out::text[] IS NULL OR r.state != all(@state_out::text[]))
    AND (@pass_in::text[] IS NULL OR r.pass = any(@pass_in::text[]))
    AND (@pass_out::text[] IS NULL OR r.pass IS NULL OR r.pass != all(@pass_out::text[]))
    AND (@day_in::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date = any(@day_in::date[]))
    AND (@day_out::date[] IS NULL OR (r.started_at AT TIME ZONE 'UTC')::date != all(@day_out::date[]))
ORDER BY
    CASE WHEN @sort_column::text = 'started_at' AND @descending::boolean THEN r.started_at END DESC,
    CASE WHEN @sort_column::text = 'started_at' AND NOT @descending::boolean THEN r.started_at END ASC,
    CASE
        WHEN @sort_column::text = 'duration' AND @descending::boolean
            THEN coalesce(r.finished_at, @as_of::timestamptz) - r.started_at
    END DESC,
    CASE
        WHEN @sort_column::text = 'duration' AND NOT @descending::boolean
            THEN coalesce(r.finished_at, @as_of::timestamptz) - r.started_at
    END ASC,
    CASE WHEN @sort_column::text = 'failures' AND @descending::boolean THEN coalesce(fc.failures, 0) END DESC,
    CASE WHEN @sort_column::text = 'failures' AND NOT @descending::boolean THEN coalesce(fc.failures, 0) END ASC,
    r.run_id ASC
LIMIT 50 OFFSET @row_offset;
