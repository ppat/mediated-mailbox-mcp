-- name: InsertMessage :one
-- Adds one message's metadata to the index, with its sender class, the policy rule that set it or
-- null when none did, its subject already masked (ADR-0003, ADR-0016, ADR-0017) and the scanner
-- version and configuration revision the masking ran under (ADR-0096). A message the index already
-- holds is left as it is and returns no row, so a page ingested twice adds nothing the second time.
-- The row holds no body, snippet or attachment name (ADR-0016).
INSERT INTO messages (
    account_id,
    message_id,
    thread_id,
    from_email,
    from_domain,
    from_name,
    subject,
    subject_masked,
    sent_at,
    labels,
    flags,
    has_attachments,
    list_id,
    size_bytes,
    auth_results,
    sender_class,
    class_rule_id,
    subject_scanner_version,
    subject_scanner_revision
) VALUES (
    @account_id,
    @message_id,
    @thread_id,
    @from_email,
    @from_domain,
    sqlc.narg(from_name),
    @subject,
    @subject_masked,
    @sent_at,
    @labels,
    @flags,
    @has_attachments,
    sqlc.narg(list_id),
    @size_bytes,
    @auth_results,
    @sender_class,
    sqlc.narg(class_rule_id),
    @subject_scanner_version,
    @subject_scanner_revision
)
ON CONFLICT (account_id, message_id) DO NOTHING
RETURNING message_id;

-- name: StaleSubject :one
-- Whether any of the account's stored subjects was masked under another scanner version or
-- configuration revision than the one given, or under none recorded, which makes the first pass due
-- again (ADR-0096).
SELECT exists(
    SELECT 1
    FROM messages AS m
    WHERE
        m.account_id = @account_id
        AND (
            m.subject_scanner_version IS DISTINCT FROM @scanner_version::int
            OR m.subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
        )
) AS stale;

-- name: StaleSubjects :many
-- The account's messages whose stored subject was masked under another scanner version or
-- configuration revision than the one given, or under none recorded, with the subject as stored, in
-- the order of their identifiers. When an enumeration made under the pair given ends, these are the
-- messages it did not find (ADR-0096).
SELECT
    m.message_id,
    coalesce(m.subject, '')::text AS subject
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (
        m.subject_scanner_version IS DISTINCT FROM @scanner_version::int
        OR m.subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
    )
ORDER BY m.message_id;

-- name: RemaskSubject :execrows
-- Replaces a stored subject masked under another scanner version or configuration revision, or under
-- none recorded, with the subject masked under the pair given, and records the pair (ADR-0096). No
-- other column of the row changes. A subject already masked under the pair given is left as it is and
-- counts no row, so a page taken twice masks nothing twice.
UPDATE messages
SET
    subject = @subject,
    subject_masked = @subject_masked,
    subject_scanner_version = @scanner_version::int,
    subject_scanner_revision = @scanner_revision::text
WHERE
    account_id = @account_id
    AND message_id = @message_id
    AND (
        subject_scanner_version IS DISTINCT FROM @scanner_version::int
        OR subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
    );
