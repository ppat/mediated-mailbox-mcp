-- name: CorpusFigures :one
-- The account's corpus at a glance, for the corpus endpoint and the attention endpoint's backlog rule
-- (docs/UI.md sections 8.1 and 17.4). Unfiled is a message with no label. The restricted messages are
-- not counted here, since the index stores no sender class. The UI sums them from the sender
-- statistics of the senders the policy in force restricts (ADR-0136). The messages are grouped by
-- thread first and the thread rows counted, which gives the count of distinct threads at a fraction of
-- what a distinct count costs, and an account with no message counts zero of each.
SELECT
    coalesce(sum(t.messages), 0)::bigint AS messages,
    count(*) AS threads,
    coalesce(sum(t.unfiled), 0)::bigint AS unfiled
FROM (
    SELECT
        count(*) AS messages,
        count(*) FILTER (WHERE m.labels = '{}') AS unfiled
    FROM messages AS m
    WHERE m.account_id = current_setting('app.account')
    GROUP BY m.thread_id
) AS t;
