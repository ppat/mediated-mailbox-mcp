-- name: PendingPage :many
-- The account's messages waiting for their content scan after the message identified by after, in
-- the order of their identifiers, each with what the scan gate reads of it and of its sender, the
-- sender's volume and prior hits (ADR-0093, ADR-0017). A message with no sender statistics reads a
-- volume and hits of zero. The page is keyed on the identifier, so a message left waiting is read
-- once per pass.
SELECT
    m.message_id,
    m.from_email,
    m.from_domain,
    m.subject_masked,
    m.sent_at,
    (m.list_id IS NOT NULL)::boolean AS has_list_id,
    coalesce(m.size_bytes, 0)::bigint AS size_bytes,
    coalesce(s.message_count, 0)::bigint AS sender_volume,
    coalesce(s.scan_hit_count, 0)::bigint AS sender_hits
FROM messages AS m
LEFT JOIN senders AS s ON m.account_id = s.account_id AND m.from_domain = s.domain
WHERE m.account_id = @account_id AND m.scan_state = 'pending' AND m.message_id > @after
ORDER BY m.message_id
LIMIT @page_size;

-- name: RecordVerdict :execrows
-- Records a waiting message's scan verdict, its content flags, the identifiers of the content rules
-- that fired, the scanner version and the configuration's revision (ADR-0009). The rules are the
-- content rules alone. A message no longer waiting is left as it is and counts no row.
UPDATE messages
SET
    scan_state = 'scanned',
    content_flags = @content_flags,
    rule_ids = @rule_ids,
    scanned_at = now(),
    scanner_version = @scanner_version,
    scanner_revision = @scanner_revision
WHERE account_id = @account_id AND message_id = @message_id AND scan_state = 'pending';

-- name: RecordSkip :execrows
-- Records that the scan gate skipped a waiting message, as skipped_restricted or skipped_gate
-- (ADR-0093). A message no longer waiting is left as it is and counts no row.
UPDATE messages
SET scan_state = @scan_state
WHERE account_id = @account_id AND message_id = @message_id AND scan_state = 'pending';

-- name: RestrictedDomains :many
-- The sender domains of the account whose messages the index stores as restricted or holds skipped as
-- restricted, each once, which the delisting transition compares with the policy in force (ADR-0037).
-- A rule added after a message was stored leaves its stored class normal while the gate skips it as
-- restricted, so the skip state is read as well.
SELECT DISTINCT m.from_domain
FROM messages AS m
WHERE m.account_id = @account_id AND (m.sender_class = 'restricted' OR m.scan_state = 'skipped_restricted')
ORDER BY m.from_domain;

-- name: MarkDelisted :execrows
-- The delisting transition for one domain the policy in force no longer restricts. Its messages stored
-- restricted or skipped as restricted return to a normal sender class with no rule naming it, and to
-- pending scan (ADR-0037, ADR-0016).
UPDATE messages
SET sender_class = 'normal', class_rule_id = NULL, scan_state = 'pending'
WHERE
    account_id = @account_id
    AND from_domain = @domain
    AND (sender_class = 'restricted' OR scan_state = 'skipped_restricted');

-- name: Backlog :one
-- How many of the account's messages wait for their content scan, the scan backlog depth a scanning
-- workload emits as a metric (ADR-0093).
SELECT count(*) AS pending
FROM messages AS m
WHERE m.account_id = @account_id AND m.scan_state = 'pending';
