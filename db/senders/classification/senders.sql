-- name: SenderCounts :many
-- Every one of the account's senders with its message count, which the UI classifies under the policy
-- in force when it reads, since the index stores no sender class (ADR-0136). The policy screens match
-- the senders against the rules to count what each rule matches, and Home sums the messages of the
-- senders the policy restricts (docs/UI.md sections 8.1 and 8.7). Matching a sender against a rule is
-- the sender classifier's, so it is made in the UI's server.
SELECT
    s.domain,
    s.message_count
FROM senders AS s
WHERE s.account_id = current_setting('app.account')
ORDER BY s.domain;

-- name: SenderSearchFigures :one
-- The senders dataset's figures under its search filter, the senders it matches and their messages
-- (docs/UI.md section 8.7). The search is a substring of the stored domain, given in the form the
-- index stores a domain in, so it ignores case as the domains do with no statement lowercasing
-- (ADR-0016). An empty search matches every sender.
SELECT
    count(*) AS senders,
    coalesce(sum(s.message_count), 0)::bigint AS messages
FROM senders AS s
WHERE
    s.account_id = current_setting('app.account')
    AND (@search::text = '' OR strpos(s.domain::text, @search::text) > 0);

-- name: SenderSearchRows :many
-- One page of the senders dataset, fifty rows, under the same filter, with the sender row's fields
-- (docs/UI.md section 7.2) but the class, which the UI classifies under the policy in force, the
-- sender with the most messages first by default, the domain ending the sort.
SELECT
    s.domain,
    s.message_count,
    s.first_seen,
    s.last_seen,
    s.has_list_id_ratio,
    s.scan_hit_count
FROM senders AS s
WHERE
    s.account_id = current_setting('app.account')
    AND (@search::text = '' OR strpos(s.domain::text, @search::text) > 0)
ORDER BY
    CASE WHEN @descending::boolean THEN s.message_count END DESC,
    CASE WHEN NOT @descending::boolean THEN s.message_count END ASC,
    s.domain ASC
LIMIT 50 OFFSET @row_offset;
