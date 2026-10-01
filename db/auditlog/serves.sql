-- name: BodyServesSince :one
-- How many bodies were served since a time and when the first of them was, for the body-serve rule of
-- Home's worth-a-look cards (docs/UI.md section 8.1). A denial is not a serve.
SELECT
    count(*) AS serves,
    min(a.ts)::timestamptz AS first_at
FROM audit_log AS a
WHERE a.account_id = @account_id AND a.action = 'READ_BODY' AND a.ts >= @since;

-- name: BodyServesByDay :many
-- The bodies served on each UTC day from one time up to another, one row per day that has a serve, the
-- daily counts the body-serve rule takes its median of (docs/UI.md section 8.1).
SELECT
    (a.ts AT TIME ZONE 'UTC')::date AS day,
    count(*) AS serves
FROM audit_log AS a
WHERE
    a.account_id = @account_id
    AND a.action = 'READ_BODY'
    AND a.ts >= @range_start
    AND a.ts < @range_end
GROUP BY day
ORDER BY day;
