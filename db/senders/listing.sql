-- name: SenderPage :many
-- One page of the account's sender statistics, the sender with the most messages first, after the
-- position a cursor names or from the start when it names none (ADR-0017). The stored sender class and
-- the prior scan hits are not read. The class is decided again against the policy in force (ADR-0002),
-- and the hits are a signal derived from bodies that no read serves.
SELECT
    s.domain,
    s.local_part_sample,
    s.display_names,
    s.message_count,
    s.first_seen,
    s.last_seen,
    s.has_list_id_ratio,
    s.label_distribution
FROM senders AS s
WHERE
    s.account_id = @account_id
    AND (
        @first_page::boolean
        OR s.message_count < @after_count::bigint
        OR (s.message_count = @after_count::bigint AND s.domain > @after_domain::citext)
    )
ORDER BY s.message_count DESC, s.domain ASC
LIMIT @page_size;
