-- name: FailureFigures :many
-- The failures dataset's figures under its filters, one row per disposition with its count and how many
-- of those items' messages are restricted or flagged (docs/UI.md sections 8.4 and 17.1). The totals
-- are their sums, which the pages count from. The joined message is the item's own, for a message or
-- an operation item, and a page item joins none, so it counts as neither restricted nor flagged. A null
-- array is no filter. The none flags select or exclude the items with no sender or no page.
SELECT
    f.disposition,
    count(*) AS failures,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted,
    count(*) FILTER (WHERE cardinality(m.content_flags) > 0) AS flagged
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
GROUP BY f.disposition
ORDER BY f.disposition;

-- name: FailuresByErrorClass :many
-- Every group of a run's failures by error class, under the same filters, ordered by count and then
-- by the group's key, the order the bars draw (docs/UI.md section 17.1).
SELECT
    f.error_class,
    count(*) AS failures,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted,
    count(*) FILTER (WHERE cardinality(m.content_flags) > 0) AS flagged
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
GROUP BY f.error_class
ORDER BY failures DESC, f.error_class ASC NULLS LAST;

-- name: FailuresBySender :many
-- Every group of a run's failures by the domain of the item's message, as FailuresByErrorClass,
-- with the items that have none in a null group, which no_domain marks. Every writer stores a domain in
-- the one form the domain normalizer gives, so each domain has one spelling and its key is that
-- spelling, and the sender filter is given its values in the same form (ADR-0016). A stored empty
-- domain is a group of its own, keyed empty, apart from the null group.
SELECT
    coalesce(m.from_domain, '')::text AS from_domain,
    (m.from_domain IS NULL)::boolean AS no_domain,
    count(*) AS failures,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted,
    count(*) FILTER (WHERE cardinality(m.content_flags) > 0) AS flagged
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
GROUP BY m.from_domain
ORDER BY failures DESC, m.from_domain ASC NULLS LAST;

-- name: FailuresByPage :many
-- Every group of a run's failures by the page each item was processed on, as
-- FailuresByErrorClass, with the items that have none in a null group.
SELECT
    f.page,
    count(*) AS failures,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted,
    count(*) FILTER (WHERE cardinality(m.content_flags) > 0) AS flagged
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
GROUP BY f.page
ORDER BY failures DESC, f.page ASC NULLS LAST;

-- name: FailuresByDisposition :many
-- Every group of a run's failures by disposition, as FailuresByErrorClass.
SELECT
    f.disposition,
    count(*) AS failures,
    count(*) FILTER (WHERE m.sender_class = 'restricted') AS restricted,
    count(*) FILTER (WHERE cardinality(m.content_flags) > 0) AS flagged
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
GROUP BY f.disposition
ORDER BY failures DESC, f.disposition ASC NULLS LAST;

-- name: FailureRows :many
-- One page of a run's failures, fifty rows, under the same filters (docs/UI.md section 8.4). Each row
-- carries the message-row fields of the item's message, null for a page item or a message the index no
-- longer holds. sort_column names last_at or attempts, in either direction, and the failure's identity
-- ends the sort.
SELECT
    f.seq,
    f.item_kind,
    f.item_id,
    m.message_id,
    m.from_email,
    m.subject,
    m.sent_at,
    m.labels,
    m.sender_class,
    m.content_flags,
    m.scan_state,
    f.page,
    f.error_class,
    f.attempts,
    f.first_at,
    f.last_at,
    f.disposition,
    f.recovered_by
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE
    f.account_id = @account_id
    AND f.run_id = @run_id
    AND (@error_class_in::text[] IS NULL OR f.error_class = any(@error_class_in::text[]))
    AND (@error_class_out::text[] IS NULL OR f.error_class != all(@error_class_out::text[]))
    AND (@disposition_in::text[] IS NULL OR f.disposition = any(@disposition_in::text[]))
    AND (@disposition_out::text[] IS NULL OR f.disposition != all(@disposition_out::text[]))
    AND (
        (@sender_in::text[] IS NULL AND NOT @sender_in_none::boolean)
        OR m.from_domain = any(@sender_in::text[])
        OR (@sender_in_none::boolean AND m.from_domain IS NULL)
    )
    AND (@sender_out::text[] IS NULL OR m.from_domain IS NULL OR m.from_domain != all(@sender_out::text[]))
    AND (NOT @sender_out_none::boolean OR m.from_domain IS NOT NULL)
    AND (
        (@page_in::integer[] IS NULL AND NOT @page_in_none::boolean)
        OR f.page = any(@page_in::integer[])
        OR (@page_in_none::boolean AND f.page IS NULL)
    )
    AND (@page_out::integer[] IS NULL OR f.page IS NULL OR f.page != all(@page_out::integer[]))
    AND (NOT @page_out_none::boolean OR f.page IS NOT NULL)
ORDER BY
    CASE WHEN @sort_column::text = 'last_at' AND @descending::boolean THEN f.last_at END DESC,
    CASE WHEN @sort_column::text = 'last_at' AND NOT @descending::boolean THEN f.last_at END ASC,
    CASE WHEN @sort_column::text = 'attempts' AND @descending::boolean THEN f.attempts END DESC,
    CASE WHEN @sort_column::text = 'attempts' AND NOT @descending::boolean THEN f.attempts END ASC,
    f.run_id ASC,
    f.seq ASC
LIMIT 50 OFFSET @row_offset;

-- name: FailureDetail :one
-- One of a run's failures with the error summary as recorded, the failures dataset's provenance for its
-- row detail, and its message's sensitivity block, the policy rule that set its class apart from the
-- content rules that set its flags, the time it was scanned and the scanner version (docs/UI.md
-- sections 7.1, 8.4 and 17.1).
SELECT
    f.seq,
    f.item_kind,
    f.item_id,
    m.message_id,
    m.from_email,
    m.subject,
    m.sent_at,
    m.labels,
    m.sender_class,
    m.content_flags,
    m.scan_state,
    f.page,
    f.error_class,
    f.attempts,
    f.first_at,
    f.last_at,
    f.disposition,
    f.recovered_by,
    f.error_summary,
    m.class_rule_id,
    m.rule_ids,
    m.scanned_at,
    m.scanner_version
FROM job_run_failures AS f
LEFT JOIN messages AS m
    ON f.account_id = m.account_id AND f.item_id = m.message_id AND f.item_kind IN ('message', 'op')
WHERE f.account_id = @account_id AND f.run_id = @run_id AND f.seq = @seq;
