-- name: MessagePage :many
-- One page of the account's messages, newest first, after the position a cursor names or from the
-- start when it names none. The columns are the metadata the client surface serves and the content
-- flags and scan state the Redaction Gate decides from. The stored sender class is not read, because
-- the gate classifies the sender again against the policy in force (ADR-0001, ADR-0002).
SELECT
    m.message_id,
    m.thread_id,
    m.from_email,
    m.from_name,
    m.subject,
    m.sent_at,
    m.labels,
    m.flags,
    m.has_attachments,
    m.attachment_types,
    m.content_flags,
    m.scan_state
FROM messages AS m
WHERE
    m.account_id = @account_id
    AND (
        @after_sent_at::timestamptz IS NULL
        OR (m.sent_at, m.message_id) < (@after_sent_at::timestamptz, @after_message_id::text)
    )
ORDER BY m.sent_at DESC, m.message_id DESC
LIMIT @page_size;

-- name: Message :many
-- One message of the account by its identifier, with the same columns as a page. No row means the
-- account holds no such message.
SELECT
    m.message_id,
    m.thread_id,
    m.from_email,
    m.from_name,
    m.subject,
    m.sent_at,
    m.labels,
    m.flags,
    m.has_attachments,
    m.attachment_types,
    m.content_flags,
    m.scan_state
FROM messages AS m
WHERE m.account_id = @account_id AND m.message_id = @message_id;

-- name: ThreadMessages :many
-- Every message of one of the account's threads, oldest first, with the same columns as a page. No
-- row means the account holds no such thread.
SELECT
    m.message_id,
    m.thread_id,
    m.from_email,
    m.from_name,
    m.subject,
    m.sent_at,
    m.labels,
    m.flags,
    m.has_attachments,
    m.attachment_types,
    m.content_flags,
    m.scan_state
FROM messages AS m
WHERE m.account_id = @account_id AND m.thread_id = @thread_id
ORDER BY m.sent_at ASC, m.message_id ASC;

-- name: ThreadPage :many
-- One page of the account's threads, the one with the latest message first, each with its number of
-- messages and the time of its latest, after the position a cursor names or from the start when it
-- names none.
SELECT
    m.thread_id,
    count(*) AS messages,
    max(m.sent_at)::timestamptz AS latest_at
FROM messages AS m
WHERE m.account_id = @account_id
GROUP BY m.thread_id
HAVING
    @before_latest_at::timestamptz IS NULL
    OR (max(m.sent_at), m.thread_id) < (@before_latest_at::timestamptz, @before_thread_id::text)
ORDER BY latest_at DESC, m.thread_id DESC
LIMIT @page_size;

-- name: Labels :many
-- Every distinct value the account's messages hold in their labels, in order. These are the values the
-- other operations take as a label.
SELECT DISTINCT l.label::text AS label
FROM messages AS m
CROSS JOIN LATERAL unnest(m.labels) AS l (label)
WHERE m.account_id = @account_id
ORDER BY label;
