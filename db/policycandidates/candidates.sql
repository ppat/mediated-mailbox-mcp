-- name: CandidateStatusCounts :many
-- The candidates dataset's figures, the count of candidates per status, under the dataset's range and
-- status filters (docs/UI.md sections 8.6 and 17.1). A status with no candidate has no row. A null
-- range bound is no bound, and a null status array no filter.
SELECT
    c.status,
    count(*) AS candidates
FROM policy_candidates AS c
WHERE
    c.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR c.created_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.created_at < @range_end::timestamptz)
    AND (@status_in::text[] IS NULL OR c.status = any(@status_in::text[]))
    AND (@status_out::text[] IS NULL OR c.status != all(@status_out::text[]))
GROUP BY c.status
ORDER BY c.status;

-- name: CandidateRows :many
-- One page of the candidates dataset, fifty rows, under the same filters, each with its sender's
-- message count and first-seen time from the sender statistics (docs/UI.md section 8.6). The sortable
-- columns are score and created_at, in either direction, and the candidate's domain ends the sort.
SELECT
    c.domain,
    c.score,
    c.signals,
    c.status,
    c.created_at,
    c.reviewed_at,
    c.reviewed_by,
    s.message_count,
    s.first_seen
FROM policy_candidates AS c
LEFT JOIN senders AS s ON c.account_id = s.account_id AND c.domain = s.domain
WHERE
    c.account_id = @account_id
    AND (@range_start::timestamptz IS NULL OR c.created_at >= @range_start::timestamptz)
    AND (@range_end::timestamptz IS NULL OR c.created_at < @range_end::timestamptz)
    AND (@status_in::text[] IS NULL OR c.status = any(@status_in::text[]))
    AND (@status_out::text[] IS NULL OR c.status != all(@status_out::text[]))
ORDER BY
    CASE WHEN @sort_score::boolean AND @descending::boolean THEN c.score END DESC,
    CASE WHEN @sort_score::boolean AND NOT @descending::boolean THEN c.score END ASC,
    CASE WHEN NOT @sort_score::boolean AND @descending::boolean THEN c.created_at END DESC,
    CASE WHEN NOT @sort_score::boolean AND NOT @descending::boolean THEN c.created_at END ASC,
    c.domain ASC
LIMIT 50 OFFSET @row_offset;
