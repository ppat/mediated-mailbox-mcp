-- name: AuditActionCounts :many
-- The audit rows written since a time, counted per action, for the corpus endpoint (docs/UI.md
-- section 8.1). An action with no row has no count.
SELECT
    a.action,
    count(*) AS entries
FROM audit_log AS a
WHERE a.account_id = current_setting('app.account') AND a.ts >= @since
GROUP BY a.action
ORDER BY a.action;
