-- name: MessageAuditRows :many
-- The newest fifty audit rows of one message, which a row detail lists (docs/UI.md sections 7.1 and
-- 17.1), newest first, the row's identity ending the sort.
SELECT
    a.id,
    a.ts,
    a.actor,
    a.action
FROM audit_log AS a
WHERE a.account_id = @account_id AND a.message_id = @message_id
ORDER BY a.ts DESC, a.id DESC
LIMIT 50;

-- name: MessageAuditCount :one
-- How many audit rows one message has, beside the newest fifty a row detail lists.
SELECT count(*) AS entries
FROM audit_log AS a
WHERE a.account_id = @account_id AND a.message_id = @message_id;
