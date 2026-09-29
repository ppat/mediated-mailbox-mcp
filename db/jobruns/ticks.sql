-- name: LastSucceededTick :one
-- When the account's latest delta sync tick that succeeded finished, null when none has (ADR-0034).
SELECT max(r.finished_at)::timestamptz AS finished_at
FROM job_runs AS r
WHERE
    r.account_id = @account_id
    AND r.workload = 'sync'
    AND r.pass = 'tick'
    AND r.state = 'succeeded';
