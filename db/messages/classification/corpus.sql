-- name: CorpusFigures :one
-- The account's corpus at a glance, for the system endpoint's corpus block (docs/UI.md section 8.1).
-- Unfiled is a message with no label, and restricted one whose sender class reads restricted now.
SELECT
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE m.labels = '{}') AS unfiled,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted
FROM messages AS m
WHERE m.account_id = @account_id;
