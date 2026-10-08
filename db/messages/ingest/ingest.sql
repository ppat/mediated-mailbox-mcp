-- name: InsertMessage :one
-- Adds one message's metadata to the index, with its sender class, the policy rule that set it or
-- null when none did, its subject already masked (ADR-0003, ADR-0016, ADR-0017) and the scanner
-- version and configuration revision the masking ran under (ADR-0120). A message the index already
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
-- again (ADR-0120).
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

-- name: CountStaleSubjects :one
-- How many of the account's stored subjects were masked under another scanner version or
-- configuration revision than the one given, or under none recorded. Any makes the first pass due
-- again, and once its enumeration has ended the first pass records the count in its checkpoint, as
-- the subjects it has still to fetch again (ADR-0120).
SELECT count(*) AS stale
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (
        m.subject_scanner_version IS DISTINCT FROM @scanner_version::int
        OR m.subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
    );

-- name: StaleSubjects :many
-- Up to the number given of the account's messages whose stored subject was masked under another
-- scanner version or configuration revision than the one given, or under none recorded, with the
-- subject as stored, in the order of their identifiers. They are the subjects the first pass fetches
-- again by identifier once its enumeration has ended, each masked again under the pair given, so a
-- subject it fetched leaves the set (ADR-0120).
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
ORDER BY m.message_id
LIMIT @batch_size;

-- name: StaleUnmaskedSubjects :many
-- Up to the number given of the account's messages after the identifier given whose stored subject
-- is unmasked and was masked under another scanner version or configuration revision than the one
-- given, or under none recorded, with the subject as stored, in the order of their identifiers. A
-- subject stored unmasked is the subject the provider returned, so a backfill run's start masks it
-- again from what is stored (ADR-0120).
SELECT
    m.message_id,
    coalesce(m.subject, '')::text AS subject
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND NOT m.subject_masked
    AND (
        m.subject_scanner_version IS DISTINCT FROM @scanner_version::int
        OR m.subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
    )
    AND m.message_id > @after
ORDER BY m.message_id
LIMIT @batch_size;

-- name: RemaskStoredSubjects :execrows
-- Replaces each stored subject named, unmasked and masked under another scanner version or
-- configuration revision than the one given, or under none recorded, with the subject given, masked
-- again from what was stored, and records the pair given, all in one statement, so masking again
-- every unmasked subject of a mailbox costs a statement a batch and not one a message (ADR-0120). No
-- other column of the row changes. A subject stored masked is left as it is, since only the provider
-- holds what its masks hid.
UPDATE messages AS m
SET
    subject = r.subject,
    subject_masked = r.subject_masked,
    subject_scanner_version = @scanner_version::int,
    subject_scanner_revision = @scanner_revision::text
FROM (
    SELECT
        unnest(@message_ids::text[]) AS message_id,
        unnest(@subjects::text[]) AS subject,
        unnest(@subjects_masked::boolean[]) AS subject_masked
) AS r
WHERE
    m.account_id = @account_id
    AND m.message_id = r.message_id
    AND NOT m.subject_masked
    AND (
        m.subject_scanner_version IS DISTINCT FROM @scanner_version::int
        OR m.subject_scanner_revision IS DISTINCT FROM @scanner_revision::text
    );

-- name: RemaskSubject :execrows
-- Replaces a stored subject masked under another scanner version or configuration revision, or under
-- none recorded, with the subject masked under the pair given, and records the pair (ADR-0120). No
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
