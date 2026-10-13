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
    r.account_id = current_setting('app.account')
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
    r.account_id = current_setting('app.account')
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
    r.account_id = current_setting('app.account')
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
    r.account_id = current_setting('app.account')
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
--
-- The page's runs are chosen first, and only then is each page run's failures counted, on the
-- failures' primary key, so a sort by start or by duration counts the failures of the page's runs
-- alone, in any plan. A sort by failures must know the count of every run in the range before it can
-- order them, so it reads the account's failures once, as one aggregate holding each run's count by
-- its identity, which each run looks its count up in. To order the runs it neither counts each run in
-- the range apart nor joins the runs to the aggregate, since a generic plan estimates one run and
-- joins by scanning the aggregate once per run. Only the page's runs are then counted, as for every
-- sort. Every other sort skips the aggregate.
WITH failure_counts AS MATERIALIZED (
    SELECT coalesce(jsonb_object_agg(g.run_id, g.failures), '{}') AS by_run
    FROM (
        SELECT
            f.run_id,
            count(*) AS failures
        FROM job_run_failures AS f
        WHERE f.account_id = current_setting('app.account') AND @sort_column::text = 'failures'
        GROUP BY f.run_id
    ) AS g
)

SELECT
    p.run_id,
    p.workload,
    p.pass,
    p.state,
    p.plan_id,
    pl.description AS plan_description,
    pl.status AS plan_status,
    p.resumed_from,
    p.started_at,
    p.finished_at,
    p.heartbeat_at,
    p.checkpoint,
    p.counters,
    p.last_error,
    fc.failures
FROM (
    SELECT
        r.run_id,
        r.workload,
        r.pass,
        r.state,
        r.plan_id,
        r.resumed_from,
        r.started_at,
        r.finished_at,
        r.heartbeat_at,
        r.checkpoint,
        r.counters,
        r.last_error
    FROM job_runs AS r
    CROSS JOIN failure_counts AS c
    WHERE
        r.account_id = current_setting('app.account')
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
        CASE
            WHEN @sort_column::text = 'failures' AND @descending::boolean
                THEN coalesce((c.by_run ->> r.run_id)::bigint, 0)
        END DESC,
        CASE
            WHEN @sort_column::text = 'failures' AND NOT @descending::boolean
                THEN coalesce((c.by_run ->> r.run_id)::bigint, 0)
        END ASC,
        r.run_id ASC
    LIMIT 50 OFFSET @row_offset
) AS p
LEFT JOIN reorg_plans AS pl ON pl.account_id = current_setting('app.account') AND p.plan_id = pl.plan_id
CROSS JOIN
    LATERAL (
        SELECT count(*) AS failures
        FROM job_run_failures AS f
        WHERE f.account_id = current_setting('app.account') AND f.run_id = p.run_id
    ) AS fc
ORDER BY
    CASE WHEN @sort_column::text = 'started_at' AND @descending::boolean THEN p.started_at END DESC,
    CASE WHEN @sort_column::text = 'started_at' AND NOT @descending::boolean THEN p.started_at END ASC,
    CASE
        WHEN @sort_column::text = 'duration' AND @descending::boolean
            THEN coalesce(p.finished_at, @as_of::timestamptz) - p.started_at
    END DESC,
    CASE
        WHEN @sort_column::text = 'duration' AND NOT @descending::boolean
            THEN coalesce(p.finished_at, @as_of::timestamptz) - p.started_at
    END ASC,
    CASE WHEN @sort_column::text = 'failures' AND @descending::boolean THEN fc.failures END DESC,
    CASE WHEN @sort_column::text = 'failures' AND NOT @descending::boolean THEN fc.failures END ASC,
    p.run_id ASC;
