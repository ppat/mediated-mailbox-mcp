-- name: SenderClasses :many
-- Every one of the account's senders with its stored class and message count, which the policy
-- screens match against the rules to count what each rule matches and restricts (docs/UI.md section
-- 8.7). Matching a sender against a rule is the sender classifier's, so it is made in the UI's server.
SELECT
    s.domain,
    s.sender_class,
    s.message_count
FROM senders AS s
WHERE s.account_id = @account_id
ORDER BY s.domain;

-- name: SenderSearchFigures :one
-- The senders dataset's figures under its search filter, the senders it matches and their messages
-- (docs/UI.md section 8.7). The search is a case-insensitive substring of the domain, and a null
-- search matches every sender.
SELECT
    count(*) AS senders,
    coalesce(sum(s.message_count), 0)::bigint AS messages
FROM senders AS s
WHERE
    s.account_id = @account_id
    AND (@search::text = '' OR strpos(lower(s.domain::text), lower(@search::text)) > 0);

-- name: SenderSearchRows :many
-- One page of the senders dataset, fifty rows, under the same filter, with the sender row's fields
-- (docs/UI.md section 7.2), the sender with the most messages first by default, the domain ending the
-- sort.
SELECT
    s.domain,
    s.sender_class,
    s.message_count,
    s.first_seen,
    s.last_seen,
    s.has_list_id_ratio,
    s.scan_hit_count
FROM senders AS s
WHERE
    s.account_id = @account_id
    AND (@search::text = '' OR strpos(lower(s.domain::text), lower(@search::text)) > 0)
ORDER BY
    CASE WHEN @descending::boolean THEN s.message_count END DESC,
    CASE WHEN NOT @descending::boolean THEN s.message_count END ASC,
    s.domain ASC
LIMIT 50 OFFSET @row_offset;
