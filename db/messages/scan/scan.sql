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

-- name: RequeueStaleVerdicts :many
-- Returns each of the account's scanned messages whose verdict was made under another scanner version
-- or configuration revision than the one given to pending scan, its verdict cleared, so the Redaction
-- Gate denies its body as pending its content scan (ADR-0096). A backfill run makes it at its start,
-- before the first pass. It returns each message's sender domain, whose prior hits the caller counts
-- again.
UPDATE messages
SET
    scan_state = 'pending',
    content_flags = '{}',
    rule_ids = '{}',
    scanned_at = NULL,
    scanner_version = NULL,
    scanner_revision = NULL
WHERE
    account_id = @account_id
    AND scan_state = 'scanned'
    AND (
        scanner_version IS DISTINCT FROM @scanner_version::int
        OR scanner_revision IS DISTINCT FROM @scanner_revision::text
    )
RETURNING from_domain;

-- name: RequeueSignalledSkips :execrows
-- Returns each of the account's messages the gate skipped whose subject is now masked to pending scan,
-- since the gate decided without that signal (ADR-0096, ADR-0093).
UPDATE messages
SET scan_state = 'pending'
WHERE account_id = @account_id AND scan_state = 'skipped_gate' AND subject_masked;

-- name: GateSkips :many
-- The account's messages the scan gate skipped, in the order of their identifiers, each with what the
-- gate reads of it and of its sender, the sender's volume and prior hits, read as PendingPage reads
-- them, so a backfill run decides each skip again under the gate it holds (ADR-0098). A message with no
-- sender statistics reads a volume and hits of zero.
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
WHERE m.account_id = @account_id AND m.scan_state = 'skipped_gate'
ORDER BY m.message_id;

-- name: RequeueGateSkips :execrows
-- Returns to pending scan each of the given messages the scan gate skipped, the skips the gate no
-- longer decides as the same skip (ADR-0098). A message no longer skipped by the gate is left as it is
-- and counts no row.
UPDATE messages
SET scan_state = 'pending'
WHERE account_id = @account_id AND scan_state = 'skipped_gate' AND message_id = any(@message_ids::text[]);
