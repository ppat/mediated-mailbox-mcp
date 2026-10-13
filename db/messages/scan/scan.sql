-- name: PendingPage :many
-- The account's messages waiting for their content scan after the message identified by after, in
-- the order of their identifiers, each with what the scan gate reads of it and of its sender, the
-- sender's volume and prior hits, which delta sync's tick reads (ADR-0093, ADR-0104). A message with
-- no sender statistics reads a volume and hits of zero. The page is keyed on the identifier, so a
-- message left waiting is read once per pass over the backlog.
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
WHERE m.account_id = current_setting('app.account') AND m.scan_state = 'pending' AND m.message_id > @after
ORDER BY m.message_id
LIMIT @page_size;

-- name: PendingPageNewestFirst :many
-- The account's messages waiting for their content scan after the position of the message last
-- read, newest first, by send time and then identifier, each with what PendingPage reads of it and
-- of its sender, which backfill's second pass reads (ADR-0126). The first page passes the position of
-- an infinite send time and an empty identifier, which every message sorts before. The row comparison
-- is what makes the send time an index condition on messages (account_id, sent_at DESC) in custom and
-- generic plans alike, so a late page costs what an early one does. Written with OR, or with a
-- nullable position, the comparison is applied as a filter and a page's cost grows with progress. The
-- page is keyed on the position, so a message left waiting is read once per pass.
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
WHERE
    m.account_id = current_setting('app.account')
    AND m.scan_state = 'pending'
    AND (m.sent_at, m.message_id) < (@after_sent_at::timestamptz, @after_message_id::text)
ORDER BY m.sent_at DESC, m.message_id DESC
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
WHERE account_id = current_setting('app.account') AND message_id = @message_id AND scan_state = 'pending';

-- name: RecordSkip :execrows
-- Records that the scan gate skipped a waiting message, as skipped_restricted or skipped_gate
-- (ADR-0093). A message no longer waiting is left as it is and counts no row.
UPDATE messages
SET scan_state = @scan_state
WHERE account_id = current_setting('app.account') AND message_id = @message_id AND scan_state = 'pending';

-- name: RecordRefusal :execrows
-- Records that a waiting message's body was refused for its size or by its conversion, with the size
-- bound in bytes it was refused under, so it no longer waits for a scan and its body is denied
-- (ADR-0132). A message no longer waiting is left as it is and counts no row.
UPDATE messages
SET scan_state = 'refused', refused_bound = @refused_bound
WHERE account_id = current_setting('app.account') AND message_id = @message_id AND scan_state = 'pending';

-- name: RequeueRefusals :execrows
-- Returns each of the account's messages whose body was refused under another size bound than the one
-- given to pending scan, its bound cleared, so the next scan fetches it again under the bound in force
-- (ADR-0132). Backfill's run-start step makes it, beside the return of stale verdicts.
UPDATE messages
SET scan_state = 'pending', refused_bound = NULL
WHERE
    account_id = current_setting('app.account')
    AND scan_state = 'refused'
    AND refused_bound IS DISTINCT FROM @refused_bound::int;

-- name: RestrictedDomains :many
-- The sender domains of the account's messages the scan gate skipped as restricted, each once, which
-- the delisting transition compares with the policy in force (ADR-0037). The index stores no sender
-- class (ADR-0136), and a message skipped as restricted is the one whose scan waits on its sender's
-- class. The partial index over the domains of those messages serves it.
SELECT DISTINCT m.from_domain
FROM messages AS m
WHERE m.account_id = current_setting('app.account') AND m.scan_state = 'skipped_restricted'
ORDER BY m.from_domain;

-- name: MarkDelisted :execrows
-- The delisting transition for one domain the policy in force no longer restricts. Its messages
-- skipped as restricted return to pending scan (ADR-0037). A skipped message holds no verdict, so the
-- check that only a scanned message carries content flags holds (ADR-0016).
UPDATE messages
SET scan_state = 'pending'
WHERE
    account_id = current_setting('app.account')
    AND from_domain = @domain
    AND scan_state = 'skipped_restricted';

-- name: Backlog :one
-- How many of the account's messages wait for their content scan, the scan backlog depth a scanning
-- workload emits as a metric (ADR-0093).
SELECT count(*) AS pending
FROM messages AS m
WHERE m.account_id = current_setting('app.account') AND m.scan_state = 'pending';

-- name: RequeueStaleVerdicts :many
-- Returns each of the account's scanned messages whose verdict was made under another scanner version
-- or configuration revision than the one given to pending scan, its verdict cleared, so the Redaction
-- Gate denies its body as pending its content scan (ADR-0120). Backfill's run-start step makes it,
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
    account_id = current_setting('app.account')
    AND scan_state = 'scanned'
    AND (
        scanner_version IS DISTINCT FROM @scanner_version::int
        OR scanner_revision IS DISTINCT FROM @scanner_revision::text
    )
RETURNING from_domain;

-- name: RequeueSignalledSkips :execrows
-- Returns each of the account's messages the gate skipped whose subject is now masked to pending scan,
-- since the gate decided without that signal (ADR-0120, ADR-0093).
UPDATE messages
SET scan_state = 'pending'
WHERE account_id = current_setting('app.account') AND scan_state = 'skipped_gate' AND subject_masked;

-- name: GateSkips :many
-- The account's messages the scan gate skipped, in the order of their identifiers, each with what the
-- gate reads of it and of its sender, the sender's volume and prior hits, read as PendingPage reads
-- them, so backfill's run-start step decides each skip again under the gate it holds (ADR-0121). A
-- message with no sender statistics reads a volume and hits of zero.
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
WHERE m.account_id = current_setting('app.account') AND m.scan_state = 'skipped_gate'
ORDER BY m.message_id;

-- name: RequeueGateSkips :execrows
-- Returns to pending scan each of the given messages the scan gate skipped, the skips the gate no
-- longer decides as the same skip (ADR-0121). A message no longer skipped by the gate is left as it is
-- and counts no row.
UPDATE messages
SET scan_state = 'pending'
WHERE
    account_id = current_setting('app.account')
    AND scan_state = 'skipped_gate'
    AND message_id = any(@message_ids::text[]);
