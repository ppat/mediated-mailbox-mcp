-- name: MaskingPairsAbove :many
-- Every sender and rule pair with more than a count of masking events since a time, for the masking
-- rule of Home's worth-a-look cards (docs/UI.md section 8.1). An event counts under the domain of its
-- message's sender, which every writer stores in the one form the domain normalizer gives, so each
-- domain has one spelling (ADR-0016), and an event whose message the index no longer holds counts
-- under no pair. Only an event whose scanner version and revision equal those its
-- message's subject was masked under counts, so the events of a masking a change of scanner replaced count
-- under none (ADR-0096, docs/UI.md section 8.5). Each pair carries its first event since the time.
SELECT
    e.rule_id,
    m.from_domain::text AS sender,
    count(*) AS events,
    min(e.masked_at)::timestamptz AS first_at
FROM masking_events AS e
INNER JOIN messages AS m
    ON e.account_id = m.account_id AND e.message_id = m.message_id
WHERE
    e.account_id = @account_id
    AND e.masked_at >= @since
    AND e.scanner_version IS NOT DISTINCT FROM m.subject_scanner_version
    AND e.scanner_revision IS NOT DISTINCT FROM m.subject_scanner_revision
GROUP BY m.from_domain, e.rule_id
HAVING count(*) > @above::bigint
ORDER BY sender, e.rule_id;
