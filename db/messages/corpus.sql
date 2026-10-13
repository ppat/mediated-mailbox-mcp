-- name: CorpusFigures :one
-- The account's corpus at a glance, for the corpus endpoint and the attention endpoint's backlog rule
-- (docs/UI.md sections 8.1 and 17.4). Unfiled is a message with no label. The restricted messages are
-- not counted here, since the index stores no sender class. The UI sums them from the sender
-- statistics of the senders the policy in force restricts (ADR-0136).
SELECT
    count(*) AS messages,
    count(DISTINCT m.thread_id) AS threads,
    count(*) FILTER (WHERE m.labels = '{}') AS unfiled
FROM messages AS m
WHERE m.account_id = current_setting('app.account');
