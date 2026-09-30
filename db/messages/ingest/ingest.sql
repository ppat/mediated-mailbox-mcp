-- name: InsertMessage :one
-- Adds one message's metadata to the index, with its sender class and its subject already masked
-- (ADR-0003, ADR-0017). A message the index already holds is left as it is and returns no row, so a
-- page ingested twice adds nothing the second time and its caller records masking events only for the
-- messages this call added. The row holds no body, snippet or attachment name (ADR-0016).
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
    sender_class
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
    @sender_class
)
ON CONFLICT (account_id, message_id) DO NOTHING
RETURNING message_id;
