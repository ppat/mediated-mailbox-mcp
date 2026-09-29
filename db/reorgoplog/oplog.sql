-- name: OpLogCount :one
-- The op log's row count for one of the account's plans, shown beside the rollback that replays it
-- (docs/UI.md section 8.3). The op log carries no account, so the plan scopes it.
SELECT count(*) AS operations
FROM reorg_op_log AS l
INNER JOIN reorg_plans AS p ON l.plan_id = p.plan_id
WHERE p.account_id = @account_id AND l.plan_id = @plan_id;
