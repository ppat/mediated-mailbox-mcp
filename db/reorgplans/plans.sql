-- name: PlanStatusCounts :many
-- The plans dataset's figures, the count of plans per status, under the dataset's range and status
-- filters (docs/UI.md sections 8.9 and 17.1). A status with no plan has no row. Its total is the sum,
-- which the row pages count from. A null range bound is no bound, and a null status array no filter.
SELECT
    p.status,
    count(*) AS plans
FROM reorg_plans AS p
WHERE
    p.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR p.created_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR p.created_at < @range_end::timestamptz)
    AND (@status_in::text[] IS NULL OR p.status = any(@status_in::text[]))
    AND (@status_out::text[] IS NULL OR p.status != all(@status_out::text[]))
GROUP BY p.status
ORDER BY p.status;

-- name: PlanRows :many
-- One page of the plans dataset, fifty rows, under the same filters (docs/UI.md section 8.9). Each
-- plan carries its message count, one operation per message (ADR-0020), and the latest apply and
-- rollback runs found by their plan reference. created_at is the one sortable column, in either
-- direction, and the plan's identity ends the sort.
WITH apply_runs AS (
    SELECT DISTINCT ON (a.plan_id)
        a.plan_id,
        a.run_id,
        a.state
    FROM job_runs AS a
    WHERE a.account_id = @account_id AND a.pass = 'apply'
    ORDER BY a.plan_id ASC, a.started_at DESC, a.run_id ASC
),

rollback_runs AS (
    SELECT DISTINCT ON (b.plan_id)
        b.plan_id,
        b.run_id
    FROM job_runs AS b
    WHERE b.account_id = @account_id AND b.pass = 'rollback'
    ORDER BY b.plan_id ASC, b.started_at DESC, b.run_id ASC
)

SELECT
    p.plan_id,
    p.description,
    p.status,
    p.proposer,
    p.created_at,
    p.approved_at,
    p.approved_by,
    p.refusal_reason,
    ar.run_id AS apply_run_id,
    ar.state AS apply_run_state,
    rr.run_id AS rollback_run_id,
    (
        SELECT count(*)
        FROM reorg_plan_ops AS o
        WHERE o.account_id = p.account_id AND o.plan_id = p.plan_id
    ) AS messages
FROM reorg_plans AS p
LEFT JOIN apply_runs AS ar ON p.plan_id = ar.plan_id
LEFT JOIN rollback_runs AS rr ON p.plan_id = rr.plan_id
WHERE
    p.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR p.created_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR p.created_at < @range_end::timestamptz)
    AND (@status_in::text[] IS NULL OR p.status = any(@status_in::text[]))
    AND (@status_out::text[] IS NULL OR p.status != all(@status_out::text[]))
ORDER BY
    CASE WHEN @descending::boolean THEN p.created_at END DESC,
    p.created_at ASC,
    p.plan_id ASC
LIMIT 50 OFFSET @row_offset;

-- name: StreamPlans :many
-- The plans the event stream follows, every plan applying and each plan it already sent, so a plan
-- that leaves APPLYING is sent once more with its new status (ADR-0058). applied counts the op log's
-- rows, which the op log scopes through its plan, and total the plan's operations.
SELECT
    p.plan_id,
    p.status,
    (
        SELECT count(*)
        FROM reorg_op_log AS l
        WHERE l.plan_id = p.plan_id
    ) AS applied,
    (
        SELECT count(*)
        FROM reorg_plan_ops AS o
        WHERE o.account_id = p.account_id AND o.plan_id = p.plan_id
    ) AS total
FROM reorg_plans AS p
WHERE
    p.account_id = @account_id
    AND (p.status = 'APPLYING' OR p.plan_id = any(@known::uuid[]))
ORDER BY p.plan_id;
