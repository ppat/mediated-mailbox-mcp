-- name: LastSucceededTick :one
-- When the account's latest delta sync tick that succeeded finished, null when none has (ADR-0034).
-- The ticks are read newest first, in the order of the run index, and the read stops at the first
-- that succeeded, so its cost does not grow with the account's history (ADR-0016).
SELECT (
    SELECT r.finished_at
    FROM job_runs AS r
    WHERE
        r.account_id = current_setting('app.account')
        AND r.workload = 'sync'
        AND r.pass = 'tick'
        AND r.state = 'succeeded'
    ORDER BY r.started_at DESC, r.run_id ASC
    LIMIT 1
)::timestamptz AS finished_at;
